package clientaccess

import (
	"context"
	"errors"
	"fmt"

	"github.com/QuantumNous/new-api/common"
)

const credentialLimitScript = `local n = redis.call("INCR", KEYS[1]); if n == 1 then redis.call("EXPIRE", KEYS[1], ARGV[1]) end; return n`

// LimitCredentialRequest 对已认证凭据按固定 60 秒窗口计数；Redis 不可用时拒绝公共查询。
func LimitCredentialRequest(ctx context.Context, credentialID int) (bool, error) {
	if credentialID <= 0 {
		return false, errors.New("client access limiter requires a credential id")
	}
	if !common.RedisEnabled || common.RDB == nil {
		return false, ErrClientAccessRedisUnavailable
	}
	key := fmt.Sprintf("new-api:client-access:limit:%d", credentialID)
	count, err := common.RDB.Eval(ctx, credentialLimitScript, []string{key}, 60).Int64()
	if err != nil {
		return false, fmt.Errorf("%w: %v", ErrClientAccessRedisUnavailable, err)
	}
	return count <= 30, nil
}
