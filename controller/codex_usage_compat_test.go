package controller

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestAdminCodexUsageRetainsLegacyResponseAfterSharedRefresh(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Channel{}))
	oldDB := model.DB
	model.DB = db
	t.Cleanup(func() { model.DB = oldDB })
	require.NoError(t, db.Create(&model.Channel{
		Id: 17, Name: "codex", Type: constant.ChannelTypeCodex,
		Key: `{"access_token":"old","refresh_token":"refresh","account_id":"account"}`,
	}).Error)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/channel/17/codex/usage", nil)
	c.Params = gin.Params{{Key: "id", Value: "17"}}
	calls := 0
	fetch := func(_ context.Context, _ *http.Client, _, accessToken, _ string) (int, []byte, error) {
		calls++
		if accessToken == "old" {
			return http.StatusUnauthorized, []byte(`{"error":"expired"}`), nil
		}
		return http.StatusOK, []byte(`{"rate_limit":{"primary_window":{"used_percent":10}}}`), nil
	}
	refresh := func(_ context.Context, _ int, opts service.CodexCredentialRefreshOptions) (*service.CodexOAuthKey, *model.Channel, error) {
		require.Contains(t, opts.StaleKey, `"access_token":"old"`)
		return &service.CodexOAuthKey{AccessToken: "new", AccountID: "account"}, nil, nil
	}
	fetchCodexChannelWhamDataWithRefresh(c, fetch, refresh, "fetch error", "获取失败")
	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, 2, calls)
	var response map[string]any
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	require.Equal(t, true, response["success"])
	require.EqualValues(t, 200, response["upstream_status"])
	require.NotNil(t, response["data"])
}
