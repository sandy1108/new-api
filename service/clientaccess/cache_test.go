package clientaccess

import (
	"context"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/alicebob/miniredis/v2"
	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/require"
)

func TestCodexUsageCacheExpiresAndFailsClosedWithoutRedis(t *testing.T) {
	server, err := miniredis.Run()
	require.NoError(t, err)
	t.Cleanup(server.Close)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { require.NoError(t, client.Close()) })
	previousClient, previousEnabled := common.RDB, common.RedisEnabled
	common.RDB, common.RedisEnabled = client, true
	t.Cleanup(func() { common.RDB, common.RedisEnabled = previousClient, previousEnabled })

	ctx := context.Background()
	missing, err := GetCachedCodexUsage(ctx, 17)
	require.NoError(t, err)
	require.Nil(t, missing)
	want := CodexUsageData{Provider: "codex", QuotaKind: "subscription_codex", Status: "fresh", FetchedAt: "2026-09-25T00:00:00Z"}
	require.NoError(t, PutCachedCodexUsage(ctx, 17, want))
	got, err := GetCachedCodexUsage(ctx, 17)
	require.NoError(t, err)
	require.Equal(t, "cached", got.Status)
	require.Equal(t, want.FetchedAt, got.FetchedAt)
	server.FastForward(91 * time.Second)
	got, err = GetCachedCodexUsage(ctx, 17)
	require.NoError(t, err)
	require.Nil(t, got)

	common.RedisEnabled = false
	_, err = GetCachedCodexUsage(ctx, 17)
	require.Error(t, err)
}
