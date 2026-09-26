package clientaccess

import (
	"context"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/alicebob/miniredis/v2"
	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/require"
)

func TestClientAccessLimiterAllowsThirtyRequestsAndFailsClosedWithoutRedis(t *testing.T) {
	server, err := miniredis.Run()
	require.NoError(t, err)
	t.Cleanup(server.Close)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { require.NoError(t, client.Close()) })
	previousClient, previousEnabled := common.RDB, common.RedisEnabled
	common.RDB, common.RedisEnabled = client, true
	t.Cleanup(func() { common.RDB, common.RedisEnabled = previousClient, previousEnabled })

	for range 30 {
		allowed, err := LimitCredentialRequest(context.Background(), 17)
		require.NoError(t, err)
		require.True(t, allowed)
	}
	allowed, err := LimitCredentialRequest(context.Background(), 17)
	require.NoError(t, err)
	require.False(t, allowed)
	allowed, err = LimitCredentialRequest(context.Background(), 18)
	require.NoError(t, err)
	require.True(t, allowed)

	common.RedisEnabled = false
	allowed, err = LimitCredentialRequest(context.Background(), 19)
	require.Error(t, err)
	require.False(t, allowed)
}
