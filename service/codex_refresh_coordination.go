package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var localCodexRefreshLocks sync.Map

const codexRefreshUnlockScript = `if redis.call("GET", KEYS[1]) == ARGV[1] then return redis.call("DEL", KEYS[1]) else return 0 end`

// acquireCodexRefreshLock 在多实例环境使用 Redis；单实例无 Redis 时仅允许旧管理路径本地互斥。
func acquireCodexRefreshLock(ctx context.Context, channelID int, requireDistributed bool) (func() error, error) {
	if !common.RedisEnabled {
		if requireDistributed {
			return nil, errors.New("distributed refresh lock unavailable")
		}
		value, _ := localCodexRefreshLocks.LoadOrStore(channelID, &sync.Mutex{})
		mutex := value.(*sync.Mutex)
		mutex.Lock()
		return func() error { mutex.Unlock(); return nil }, nil
	}
	if common.RDB == nil {
		return nil, errors.New("redis refresh lock unavailable")
	}
	var random [16]byte
	if _, err := rand.Read(random[:]); err != nil {
		return nil, err
	}
	owner := hex.EncodeToString(random[:])
	key := fmt.Sprintf("new-api:codex-refresh:%d", channelID)
	for {
		acquired, err := common.RDB.SetNX(ctx, key, owner, 30*time.Second).Result()
		if err != nil {
			return nil, err
		}
		if acquired {
			return func() error {
				releaseCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
				defer cancel()
				return common.RDB.Eval(releaseCtx, codexRefreshUnlockScript, []string{key}, owner).Err()
			}, nil
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(50 * time.Millisecond):
		}
	}
}

// persistCodexKeyIfUnchanged 用旧 Key 作条件更新，防止迟到的刷新结果覆盖较新的 OAuth 凭据。
func persistCodexKeyIfUnchanged(db *gorm.DB, channelID int, oldKey, newKey string) (bool, error) {
	result := db.Model(&model.Channel{}).
		Where("id = ?", channelID).
		Where(clause.Eq{Column: clause.Column{Name: "key"}, Value: oldKey}).
		Update("key", newKey)
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected == 1, nil
}
