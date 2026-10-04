package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service/clientaccess"
	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestClientCodexUsageEndpointIsScopedAndRateLimited(t *testing.T) {
	gin.SetMode(gin.TestMode)
	redisServer, err := miniredis.Run()
	require.NoError(t, err)
	t.Cleanup(redisServer.Close)
	rdb := redis.NewClient(&redis.Options{Addr: redisServer.Addr()})
	t.Cleanup(func() { require.NoError(t, rdb.Close()) })
	oldRDB, oldEnabled := common.RDB, common.RedisEnabled
	common.RDB, common.RedisEnabled = rdb, true
	t.Cleanup(func() { common.RDB, common.RedisEnabled = oldRDB, oldEnabled })

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"access_token":"upstream-secret","rate_limit":{"primary_window":{"used_percent":12,"limit_window_seconds":18000}}}`))
	}))
	t.Cleanup(upstream.Close)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, db.AutoMigrate(&model.Channel{}))
	require.NoError(t, model.MigrateClientAccessTables(db))
	oldDB := model.DB
	model.DB = db
	t.Cleanup(func() { model.DB = oldDB })
	baseURL := upstream.URL
	require.NoError(t, db.Create(&model.Channel{Id: 17, Name: "codex", Type: constant.ChannelTypeCodex,
		Status: common.ChannelStatusEnabled, BaseURL: &baseURL,
		Key: `{"access_token":"server-secret","account_id":"secret-account"}`}).Error)
	store := clientaccess.NewStore(db)
	issued, err := store.CreateCredential(1, clientaccess.CreateInput{Name: "Android", ExpiredTime: -1,
		Grants: []clientaccess.GrantInput{{Scope: clientaccess.ScopeCodexQuotaRead, ResourceID: "17"}}})
	require.NoError(t, err)

	router := gin.New()
	router.GET("/usage", middleware.ClientAccessAuth(), middleware.ClientAccessRateLimit(),
		middleware.RequireClientScope(clientaccess.ScopeCodexQuotaRead), GetClientCodexUsage)
	request := func(token string) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(http.MethodGet, "/usage", nil)
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		resp := httptest.NewRecorder()
		router.ServeHTTP(resp, req)
		return resp
	}
	require.Equal(t, http.StatusUnauthorized, request("").Code)
	first := request(issued.Token)
	require.Equal(t, http.StatusOK, first.Code)
	require.Contains(t, first.Body.String(), `"status":"fresh"`)
	require.NotContains(t, first.Body.String(), "upstream-secret")
	require.NotContains(t, first.Body.String(), "server-secret")
	require.NotContains(t, first.Body.String(), "secret-account")
	require.Equal(t, http.StatusOK, request(issued.Token).Code)
	for range 28 {
		require.Equal(t, http.StatusOK, request(issued.Token).Code)
	}
	require.Equal(t, http.StatusTooManyRequests, request(issued.Token).Code)

	common.RedisEnabled = false
	require.Equal(t, http.StatusServiceUnavailable, request(issued.Token).Code)
}

func TestGetClientCodexUsageReturnsInternalErrorForDatabaseFailure(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	previousDB := model.DB
	model.DB = db
	t.Cleanup(func() { model.DB = previousDB })

	response := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(response)
	context.Request = httptest.NewRequest(http.MethodGet, "/api/client-access/v1/codex/usage", nil)
	context.Set(middleware.ClientAccessResourceIDContextKey, "17")
	GetClientCodexUsage(context)

	require.Equal(t, http.StatusInternalServerError, response.Code)
	require.Contains(t, response.Body.String(), `"code":"CLIENT_ACCESS_INTERNAL_ERROR"`)
	require.NotContains(t, response.Body.String(), "no such table")
}

func TestGetClientCodexUsageKeepsRedisFailureAsServiceUnavailable(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Channel{}))
	require.NoError(t, db.Create(&model.Channel{
		Id: 17, Name: "codex", Type: constant.ChannelTypeCodex, Status: common.ChannelStatusEnabled,
		Key: `{"access_token":"server-secret","account_id":"secret-account"}`,
	}).Error)
	previousDB, previousRedis, previousRedisEnabled := model.DB, common.RDB, common.RedisEnabled
	model.DB, common.RDB, common.RedisEnabled = db, nil, false
	t.Cleanup(func() {
		model.DB, common.RDB, common.RedisEnabled = previousDB, previousRedis, previousRedisEnabled
	})

	response := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(response)
	context.Request = httptest.NewRequest(http.MethodGet, "/api/client-access/v1/codex/usage", nil)
	context.Set(middleware.ClientAccessResourceIDContextKey, "17")
	GetClientCodexUsage(context)

	require.Equal(t, http.StatusServiceUnavailable, response.Code)
	require.Contains(t, response.Body.String(), `"code":"CLIENT_ACCESS_REDIS_UNAVAILABLE"`)
}
