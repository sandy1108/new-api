package service

import (
	"context"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/alicebob/miniredis/v2"
	"github.com/glebarez/sqlite"
	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestPersistCodexKeyIfUnchangedDoesNotOverwriteNewerRefresh(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Channel{}))
	require.NoError(t, db.Create(&model.Channel{Id: 17, Name: "codex", Key: "old-key"}).Error)

	written, err := persistCodexKeyIfUnchanged(db, 17, "old-key", "new-key")
	require.NoError(t, err)
	require.True(t, written)
	written, err = persistCodexKeyIfUnchanged(db, 17, "old-key", "stale-key")
	require.NoError(t, err)
	require.False(t, written)
	var channel model.Channel
	require.NoError(t, db.First(&channel, 17).Error)
	require.Equal(t, "new-key", channel.Key)
}

// TestPersistCodexKeyIfUnchangedIsolatedSQLDatabases 只连接显式启用的隔离数据库，覆盖保留字列在真实方言中的条件更新。
func TestPersistCodexKeyIfUnchangedIsolatedSQLDatabases(t *testing.T) {
	if os.Getenv("CLIENT_ACCESS_ISOLATED_TEST_DB") != "1" {
		t.Skip("requires isolated test database opt-in")
	}
	for _, tc := range []struct {
		name string
		env  string
		open func(string) gorm.Dialector
	}{
		{"postgres", "CLIENT_ACCESS_TEST_POSTGRES_DSN", func(dsn string) gorm.Dialector { return postgres.Open(dsn) }},
		{"mysql", "CLIENT_ACCESS_TEST_MYSQL_DSN", func(dsn string) gorm.Dialector { return mysql.Open(dsn) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dsn := os.Getenv(tc.env)
			if dsn == "" {
				t.Skip(tc.env + " is not configured")
			}
			db, err := gorm.Open(tc.open(dsn), &gorm.Config{})
			require.NoError(t, err)
			sqlDB, err := db.DB()
			require.NoError(t, err)
			t.Cleanup(func() { _ = sqlDB.Close() })
			require.NoError(t, db.AutoMigrate(&model.Channel{}))

			channelID := int(time.Now().UnixMilli())
			oldKey := fmt.Sprintf("old-key-%d", channelID)
			channel := model.Channel{Id: channelID, Name: fmt.Sprintf("refresh-cas-%d", channelID), Key: oldKey}
			require.NoError(t, db.Create(&channel).Error)
			t.Cleanup(func() { _ = db.Delete(&model.Channel{}, channelID).Error })

			written, err := persistCodexKeyIfUnchanged(db, channelID, oldKey, "new-key")
			require.NoError(t, err)
			require.True(t, written)
			written, err = persistCodexKeyIfUnchanged(db, channelID, oldKey, "stale-key")
			require.NoError(t, err)
			require.False(t, written)
			var updated model.Channel
			require.NoError(t, db.First(&updated, channelID).Error)
			require.Equal(t, "new-key", updated.Key)
		})
	}
}

func TestCodexRefreshRedisLockDoesNotReleaseAnotherOwner(t *testing.T) {
	server, err := miniredis.Run()
	require.NoError(t, err)
	t.Cleanup(server.Close)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { require.NoError(t, client.Close()) })
	previousClient, previousEnabled := common.RDB, common.RedisEnabled
	common.RDB, common.RedisEnabled = client, true
	t.Cleanup(func() { common.RDB, common.RedisEnabled = previousClient, previousEnabled })

	unlock, err := acquireCodexRefreshLock(context.Background(), 17, true)
	require.NoError(t, err)
	ctx, cancel := context.WithTimeout(context.Background(), 35*time.Millisecond)
	defer cancel()
	_, err = acquireCodexRefreshLock(ctx, 17, true)
	require.Error(t, err)

	key := "new-api:codex-refresh:17"
	require.NoError(t, client.Set(context.Background(), key, "new-owner", time.Minute).Err())
	require.NoError(t, unlock())
	value, err := client.Get(context.Background(), key).Result()
	require.NoError(t, err)
	require.Equal(t, "new-owner", value)
}

func TestCoordinatedCodexRefreshUsesOneRefreshAndPreservesLatestKey(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, db.AutoMigrate(&model.Channel{}))
	stale := `{"access_token":"old-access","refresh_token":"old-refresh","account_id":"account"}`
	require.NoError(t, db.Create(&model.Channel{Id: 17, Name: "codex", Type: constant.ChannelTypeCodex, Key: stale}).Error)
	originalDB, originalRedis := model.DB, common.RedisEnabled
	model.DB, common.RedisEnabled = db, false
	t.Cleanup(func() { model.DB, common.RedisEnabled = originalDB, originalRedis })

	var upstreamCalls atomic.Int32
	refresh := func(_ context.Context, token, _ string) (*CodexOAuthTokenResult, error) {
		require.Equal(t, "old-refresh", token)
		upstreamCalls.Add(1)
		time.Sleep(20 * time.Millisecond)
		return &CodexOAuthTokenResult{AccessToken: "new-access", RefreshToken: "new-refresh", ExpiresAt: time.Now().Add(time.Hour)}, nil
	}
	var wg sync.WaitGroup
	results := make(chan string, 2)
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			key, _, err := refreshCodexChannelCredential(context.Background(), 17,
				CodexCredentialRefreshOptions{StaleKey: stale}, refresh)
			if err != nil {
				results <- err.Error()
				return
			}
			results <- key.AccessToken
		}()
	}
	wg.Wait()
	close(results)
	for result := range results {
		require.Equal(t, "new-access", result)
	}
	require.EqualValues(t, 1, upstreamCalls.Load())
	var channel model.Channel
	require.NoError(t, db.First(&channel, 17).Error)
	require.Contains(t, channel.Key, "new-refresh")
}
