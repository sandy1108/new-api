package controller

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"context"
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service/clientaccess"
	quota "github.com/QuantumNous/new-api/service/privatequota"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"time"
)

func TestPrivateQuotaAPIValidatesVersionsAndDoesNotLeakSourceIdentity(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
	require.NoError(t, db.AutoMigrate(&model.Channel{}))
	require.NoError(t, model.MigratePrivateQuotaTables(db))
	_, err = model.InitializePrivateQuotaSettings(db, time.Now().Unix())
	require.NoError(t, err)
	previous := model.DB
	model.DB = db
	t.Cleanup(func() { model.DB = previous })
	require.NoError(t, db.Create(&model.Channel{Id: 17, Name: "Synthetic", Type: constant.ChannelTypeCodex, Status: common.ChannelStatusEnabled, Key: `{"account_id":"hidden-account","access_token":"hidden-token"}`}).Error)
	engine := gin.New()
	engine.Use(func(c *gin.Context) { c.Set("id", 42); c.Next() })
	engine.GET("/channels", ListPrivateQuotaChannels)
	engine.POST("/subjects", CreatePrivateQuotaSubject)
	engine.GET("/subjects", ListPrivateQuotaSubjects)
	engine.GET("/summary", GetPrivateQuotaSummary)
	engine.GET("/trend", GetPrivateQuotaTrend)
	engine.PUT("/settings", UpdatePrivateQuotaSettings)
	perform := func(method, path, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		response := httptest.NewRecorder()
		engine.ServeHTTP(response, req)
		return response
	}
	for _, path := range []string{"/channels", "/subjects"} {
		response := perform("GET", path, "")
		require.Equal(t, 200, response.Code)
		require.NotContains(t, response.Body.String(), "hidden-")
		require.NotContains(t, response.Body.String(), "fingerprint")
	}
	created := perform("POST", "/subjects", `{"name":"My Plus","source_channel_id":17,"enabled":true}`)
	require.Equal(t, 200, created.Code)
	duplicate := perform("POST", "/subjects", `{"name":"Duplicate","source_channel_id":17,"enabled":true}`)
	require.Equal(t, 409, duplicate.Code)
	for _, path := range []string{"/summary?subject_id=1&month=2026-10", "/trend?subject_id=1&month=2026-10"} {
		response := perform("GET", path, "")
		require.Equal(t, 200, response.Code)
		require.NotContains(t, response.Body.String(), "hidden-")
	}
	require.Contains(t, perform("GET", "/summary?subject_id=1&month=2026-10", "").Body.String(), `"collection_enabled":false`)
	for _, path := range []string{"/summary?subject_id=1&month=2026-13", "/summary?subject_id=1&window_seconds=3600", "/subjects?page=9223372036854775807"} {
		require.Equal(t, 400, perform("GET", path, "").Code)
	}
	require.Equal(t, 404, perform("GET", "/summary?subject_id=99", "").Code)
	require.Equal(t, 200, perform("PUT", "/settings", `{"enabled":false,"interval_minutes":20,"version":1}`).Code)
	require.Equal(t, 409, perform("PUT", "/settings", `{"enabled":true,"interval_minutes":10,"version":1}`).Code)
	// 空账本只读不生成观测，也不为填充图表伪造0%样本。
	var count int64
	require.NoError(t, db.Model(&model.PrivateQuotaSnapshot{}).Count(&count).Error)
	require.Zero(t, count)
	_, _, err = (&quota.Store{DB: db}).ReadLedger(context.Background(), 1)
	require.NoError(t, err)
}

func TestClientAccessManagementCreateListsOnlyMaskedCredentialAndRejectsStaleVersion(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Channel{}))
	require.NoError(t, model.MigrateClientAccessTables(db))
	originalDB := model.DB
	model.DB = db
	t.Cleanup(func() { model.DB = originalDB })
	require.NoError(t, db.Create(&model.Channel{Id: 17, Name: "codex", Type: constant.ChannelTypeCodex,
		Status: common.ChannelStatusEnabled, Key: `{"access_token":"secret-access","account_id":"secret-account"}`}).Error)

	router := gin.New()
	router.Use(func(c *gin.Context) { c.Set("id", 1); c.Next() })
	router.POST("/credentials", CreateClientAccessCredential)
	router.GET("/credentials", ListClientAccessCredentials)
	router.GET("/credentials/:id", GetClientAccessCredential)
	router.GET("/scopes", GetClientAccessScopes)
	router.GET("/scopes/:scope/resources", GetClientAccessScopeResources)
	router.PUT("/credentials/:id/grants", UpdateClientAccessGrants)
	router.POST("/credentials/:id/rotate", RotateClientAccessCredential)
	router.POST("/credentials/:id/disable", DisableClientAccessCredential)
	router.POST("/credentials/:id/enable", EnableClientAccessCredential)
	router.POST("/credentials/:id/revoke", RevokeClientAccessCredential)

	perform := func(method, path string, body any) *httptest.ResponseRecorder {
		t.Helper()
		encoded, err := json.Marshal(body)
		require.NoError(t, err)
		req := httptest.NewRequest(method, path, bytes.NewReader(encoded))
		req.Header.Set("Content-Type", "application/json")
		resp := httptest.NewRecorder()
		router.ServeHTTP(resp, req)
		return resp
	}
	created := perform(http.MethodPost, "/credentials", map[string]any{
		"name": "Android", "expired_time": -1,
		"grants": []map[string]string{{"scope": "codex.quota.read", "resource_id": "17"}},
	})
	require.Equal(t, http.StatusOK, created.Code)
	var createdBody struct {
		Data struct {
			Token      string `json:"token"`
			Credential struct {
				Id      int   `json:"id"`
				Version int64 `json:"version"`
			} `json:"credential"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(created.Body.Bytes(), &createdBody))
	require.NotEmpty(t, createdBody.Data.Token)
	require.NotContains(t, created.Body.String(), "secret-access")
	require.NotContains(t, created.Body.String(), "token_hash")

	listed := perform(http.MethodGet, "/credentials", nil)
	require.Equal(t, http.StatusOK, listed.Code)
	require.NotContains(t, listed.Body.String(), createdBody.Data.Token)
	require.NotContains(t, listed.Body.String(), "token_hash")
	require.NotContains(t, listed.Body.String(), "secret-access")
	detail := perform(http.MethodGet, "/credentials/1", nil)
	require.Equal(t, http.StatusOK, detail.Code)
	require.Contains(t, detail.Body.String(), `"resource_id":"17"`)
	require.NotContains(t, detail.Body.String(), createdBody.Data.Token)
	require.NotContains(t, detail.Body.String(), "token_hash")
	require.NotContains(t, detail.Body.String(), "secret-access")

	stale := perform(http.MethodPut, "/credentials/1/grants", map[string]any{
		"version": 999, "grants": []map[string]string{},
	})
	require.Equal(t, http.StatusConflict, stale.Code)

	scopes := perform(http.MethodGet, "/scopes", nil)
	require.Equal(t, http.StatusOK, scopes.Code)
	require.Contains(t, scopes.Body.String(), "codex.quota.read")
	require.NotContains(t, scopes.Body.String(), "validator")
	resources := perform(http.MethodGet, "/scopes/codex.quota.read/resources", nil)
	require.Equal(t, http.StatusOK, resources.Code)
	require.Contains(t, resources.Body.String(), `"id":17`)
	require.NotContains(t, resources.Body.String(), "secret-access")

	rotate := perform(http.MethodPost, "/credentials/1/rotate", map[string]any{"version": 1})
	require.Equal(t, http.StatusOK, rotate.Code)
	var rotated struct {
		Data struct {
			Token string `json:"token"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rotate.Body.Bytes(), &rotated))
	require.NotEmpty(t, rotated.Data.Token)
	_, err = clientaccess.NewStore(db).FindCredentialByToken(createdBody.Data.Token)
	require.Error(t, err)

	disabled := perform(http.MethodPost, "/credentials/1/disable", map[string]any{"version": 2})
	require.Equal(t, http.StatusOK, disabled.Code)
	_, err = clientaccess.NewStore(db).FindCredentialByToken(rotated.Data.Token)
	require.Error(t, err)
	require.Equal(t, http.StatusOK, perform(http.MethodPost, "/credentials/1/enable", map[string]any{"version": 3}).Code)
	_, err = clientaccess.NewStore(db).FindCredentialByToken(rotated.Data.Token)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, perform(http.MethodPost, "/credentials/1/revoke", map[string]any{"version": 4}).Code)
	require.Equal(t, http.StatusConflict, perform(http.MethodPost, "/credentials/1/enable", map[string]any{"version": 5}).Code)
}
