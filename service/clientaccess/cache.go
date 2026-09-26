package clientaccess

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/go-redis/redis/v8"
)

const codexUsageCacheTTL = 90 * time.Second
const codexUsageFetchUnlockScript = `if redis.call("GET", KEYS[1]) == ARGV[1] then return redis.call("DEL", KEYS[1]) else return 0 end`

// ErrClientAccessRedisUnavailable 表示客户端访问所依赖的 Redis 不可用。
var ErrClientAccessRedisUnavailable = errors.New("client access redis unavailable")

// ErrCodexUsageFetchLockTimeout 表示等待同渠道额度抓取锁超时。
var ErrCodexUsageFetchLockTimeout = errors.New("codex usage fetch lock timeout")

func codexUsageCacheKey(channelID int) string {
	return fmt.Sprintf("new-api:client-access:codex-usage:%d", channelID)
}

func requireClientAccessRedis() error {
	if !common.RedisEnabled || common.RDB == nil {
		return ErrClientAccessRedisUnavailable
	}
	return nil
}

func GetCachedCodexUsage(ctx context.Context, channelID int) (*CodexUsageData, error) {
	if err := requireClientAccessRedis(); err != nil {
		return nil, err
	}
	encoded, err := common.RDB.Get(ctx, codexUsageCacheKey(channelID)).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrClientAccessRedisUnavailable, err)
	}
	var data CodexUsageData
	if err := common.Unmarshal(encoded, &data); err != nil {
		return nil, err
	}
	data.Status = "cached"
	return &data, nil
}

func PutCachedCodexUsage(ctx context.Context, channelID int, data CodexUsageData) error {
	if err := requireClientAccessRedis(); err != nil {
		return err
	}
	encoded, err := common.Marshal(data)
	if err != nil {
		return err
	}
	if err := common.RDB.Set(ctx, codexUsageCacheKey(channelID), encoded, codexUsageCacheTTL).Err(); err != nil {
		return fmt.Errorf("%w: %v", ErrClientAccessRedisUnavailable, err)
	}
	return nil
}

// acquireCodexUsageFetchLock 使同渠道跨实例缓存未命中只产生一次上游读取。
func acquireCodexUsageFetchLock(ctx context.Context, channelID int) (func() error, error) {
	if err := requireClientAccessRedis(); err != nil {
		return nil, err
	}
	var random [16]byte
	if _, err := rand.Read(random[:]); err != nil {
		return nil, err
	}
	owner := hex.EncodeToString(random[:])
	key := fmt.Sprintf("new-api:client-access:codex-fetch:%d", channelID)
	waitCtx, cancelWait := context.WithTimeout(ctx, 20*time.Second)
	defer cancelWait()
	for {
		acquired, err := common.RDB.SetNX(waitCtx, key, owner, 60*time.Second).Result()
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrClientAccessRedisUnavailable, err)
		}
		if acquired {
			return func() error {
				releaseCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
				defer cancel()
				return common.RDB.Eval(releaseCtx, codexUsageFetchUnlockScript, []string{key}, owner).Err()
			}, nil
		}
		select {
		case <-waitCtx.Done():
			return nil, fmt.Errorf("%w: %v", ErrCodexUsageFetchLockTimeout, waitCtx.Err())
		case <-time.After(50 * time.Millisecond):
		}
	}
}
