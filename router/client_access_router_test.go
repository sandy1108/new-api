package router

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestClientAccessManagementRoutesRequireDashboardRootIdentity(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	SetApiRouter(router)

	for _, path := range []string{
		"/api/client-access/scopes",
		"/api/client-access/credentials",
	} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		resp := httptest.NewRecorder()
		router.ServeHTTP(resp, req)
		require.Equal(t, http.StatusUnauthorized, resp.Code, path)
	}
}

func TestClientAccessCodexUsageRouteRequiresSeparateClientCredential(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.User{}))
	require.NoError(t, model.MigrateClientAccessTables(db))
	oldDB := model.DB
	model.DB = db
	t.Cleanup(func() { model.DB = oldDB })
	router := gin.New()
	SetApiRouter(router)
	for _, authorization := range []string{"", "Bearer sk-model-token", "Bearer napi_client_invalid"} {
		req := httptest.NewRequest(http.MethodGet, "/api/client-access/v1/codex/usage", nil)
		if authorization != "" {
			req.Header.Set("Authorization", authorization)
		}
		resp := httptest.NewRecorder()
		router.ServeHTTP(resp, req)
		require.Equal(t, http.StatusUnauthorized, resp.Code)
	}
}
