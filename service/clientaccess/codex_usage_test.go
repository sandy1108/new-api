package clientaccess

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/alicebob/miniredis/v2"
	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/require"
)

func TestNormalizeCodexUsageMapsOnlyExplicitWindowsAndOmitsSensitiveFields(t *testing.T) {
	body := []byte(`{
		"account_id":"secret-account",
		"access_token":"secret-access",
		"rate_limit":{
			"primary_window":{"used_percent":12.5,"limit_window_seconds":18000,"reset_at":1780000000},
			"secondary_window":{"used_percent":32.1,"limit_window_seconds":604800,"reset_at":1780500000}
		}
	}`)
	data, err := NormalizeCodexUsage(body, time.Unix(1779990000, 0))
	require.NoError(t, err)
	require.NotNil(t, data.Windows.FiveHour)
	require.Equal(t, 12.5, *data.Windows.FiveHour.UsedPercent)
	require.Equal(t, 87.5, *data.Windows.FiveHour.RemainingPercent)
	require.EqualValues(t, 18000, data.Windows.FiveHour.WindowSeconds)
	require.NotNil(t, data.Windows.SevenDay)
	require.Equal(t, 32.1, *data.Windows.SevenDay.UsedPercent)
	encoded, err := json.Marshal(data)
	require.NoError(t, err)
	require.NotContains(t, string(encoded), "secret-access")
	require.NotContains(t, string(encoded), "secret-account")
	require.NotContains(t, string(encoded), "account_id")
}

func TestGetCodexUsageSharesCacheAcrossConcurrentRequests(t *testing.T) {
	redisServer, err := miniredis.Run()
	require.NoError(t, err)
	t.Cleanup(redisServer.Close)
	client := redis.NewClient(&redis.Options{Addr: redisServer.Addr()})
	t.Cleanup(func() { require.NoError(t, client.Close()) })
	previousClient, previousEnabled := common.RDB, common.RedisEnabled
	common.RDB, common.RedisEnabled = client, true
	t.Cleanup(func() { common.RDB, common.RedisEnabled = previousClient, previousEnabled })

	var upstreamCalls atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamCalls.Add(1)
		require.Equal(t, "/backend-api/wham/usage", r.URL.Path)
		require.Equal(t, "Bearer test-access", r.Header.Get("Authorization"))
		time.Sleep(25 * time.Millisecond)
		_, _ = w.Write([]byte(`{"rate_limit":{"primary_window":{"used_percent":12,"limit_window_seconds":18000}}}`))
	}))
	t.Cleanup(upstream.Close)
	store := newClientAccessTestStore(t)
	baseURL := upstream.URL
	require.NoError(t, store.DB.Create(&model.Channel{Id: 17, Name: "codex", Type: constant.ChannelTypeCodex,
		Status: common.ChannelStatusEnabled, BaseURL: &baseURL,
		Key: `{"access_token":"test-access","account_id":"account"}`}).Error)
	oldDB := model.DB
	model.DB = store.DB
	t.Cleanup(func() { model.DB = oldDB })

	var wg sync.WaitGroup
	results := make(chan string, 2)
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			data, err := store.GetCodexUsage(context.Background(), 17)
			if err != nil {
				results <- err.Error()
				return
			}
			results <- data.Status
		}()
	}
	wg.Wait()
	close(results)
	for status := range results {
		require.Contains(t, []string{"fresh", "cached"}, status)
	}
	require.EqualValues(t, 1, upstreamCalls.Load())
}

func TestNormalizeCodexUsageDoesNotGuessUnknownOrMissingWindow(t *testing.T) {
	body := []byte(`{"rate_limit":{"primary_window":{"used_percent":40,"limit_window_seconds":3600},"secondary_window":{"limit_window_seconds":18000}}}`)
	data, err := NormalizeCodexUsage(body, time.Unix(1779990000, 0))
	require.NoError(t, err)
	require.Nil(t, data.Windows.SevenDay)
	require.NotNil(t, data.Windows.FiveHour)
	require.Nil(t, data.Windows.FiveHour.UsedPercent)
	require.Nil(t, data.Windows.FiveHour.RemainingPercent)
	encoded, err := json.Marshal(data)
	require.NoError(t, err)
	require.Contains(t, string(encoded), `"seven_day":null`)
	require.NotContains(t, string(encoded), `"used_percent":40`)
}

func TestNormalizeCodexUsageRejectsErrorPayloadInsteadOfInventingQuota(t *testing.T) {
	_, err := NormalizeCodexUsage([]byte(`{"error":"upstream failed"}`), time.Now())
	require.Error(t, err)
}

func TestCachedUsageCannotBypassDisabledChannel(t *testing.T) {
	redisServer, err := miniredis.Run()
	require.NoError(t, err)
	t.Cleanup(redisServer.Close)
	client := redis.NewClient(&redis.Options{Addr: redisServer.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	previousClient, previousEnabled := common.RDB, common.RedisEnabled
	common.RDB, common.RedisEnabled = client, true
	t.Cleanup(func() { common.RDB, common.RedisEnabled = previousClient, previousEnabled })
	store := newClientAccessTestStore(t)
	addCodexChannel(t, store, 17, constant.ChannelTypeCodex, true)
	require.NoError(t, PutCachedCodexUsage(context.Background(), 17, CodexUsageData{Provider: "codex"}))
	require.NoError(t, store.DB.Model(&model.Channel{}).Where("id = ?", 17).Update("status", common.ChannelStatusManuallyDisabled).Error)
	_, err = store.GetCodexUsage(context.Background(), 17)
	require.ErrorIs(t, err, ErrResourceUnavailable)
}

func TestGetCodexUsageMapsUpstreamFailuresWithoutReturningRawPayload(t *testing.T) {
	redisServer, err := miniredis.Run()
	require.NoError(t, err)
	t.Cleanup(redisServer.Close)
	client := redis.NewClient(&redis.Options{Addr: redisServer.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	previousClient, previousEnabled := common.RDB, common.RedisEnabled
	common.RDB, common.RedisEnabled = client, true
	t.Cleanup(func() { common.RDB, common.RedisEnabled = previousClient, previousEnabled })
	store := newClientAccessTestStore(t)
	for _, tc := range []struct {
		id     int
		status int
		body   string
		want   error
	}{
		{21, http.StatusTooManyRequests, `{"error":"private rate limit detail"}`, ErrCodexUpstreamLimited},
		{22, http.StatusUnauthorized, `{"error":"private oauth detail"}`, ErrCodexUpstreamAuth},
		{23, http.StatusOK, `{"error":"private malformed success"}`, ErrCodexUpstreamUnavailable},
	} {
		upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(tc.status)
			_, _ = w.Write([]byte(tc.body))
		}))
		baseURL := upstream.URL
		require.NoError(t, store.DB.Create(&model.Channel{Id: tc.id, Name: "codex", Type: constant.ChannelTypeCodex,
			Status: common.ChannelStatusEnabled, BaseURL: &baseURL,
			Key: `{"access_token":"test-access","account_id":"account"}`}).Error)
		_, err := store.GetCodexUsage(context.Background(), tc.id)
		require.ErrorIs(t, err, tc.want)
		require.NotContains(t, err.Error(), "private")
		upstream.Close()
	}
}
