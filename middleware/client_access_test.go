package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service/clientaccess"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestClientAccessMiddlewareChecksLiveGrantAndChannel(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Channel{}))
	require.NoError(t, model.MigrateClientAccessTables(db))
	originalDB := model.DB
	model.DB = db
	t.Cleanup(func() { model.DB = originalDB })
	require.NoError(t, db.Create(&model.Channel{
		Id: 17, Name: "codex", Type: constant.ChannelTypeCodex, Status: common.ChannelStatusEnabled,
		Key: `{"access_token":"access","account_id":"account"}`,
	}).Error)
	store := clientaccess.NewStore(db)
	issued, err := store.CreateCredential(1, clientaccess.CreateInput{Name: "Android", ExpiredTime: -1,
		Grants: []clientaccess.GrantInput{{Scope: clientaccess.ScopeCodexQuotaRead, ResourceID: "17"}}})
	require.NoError(t, err)

	router := gin.New()
	router.GET("/quota", ClientAccessAuth(), RequireClientScope(clientaccess.ScopeCodexQuotaRead), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"resource_id": c.GetString(ClientAccessResourceIDContextKey)})
	})
	request := func(token string) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(http.MethodGet, "/quota", nil)
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		resp := httptest.NewRecorder()
		router.ServeHTTP(resp, req)
		return resp
	}
	require.Equal(t, http.StatusUnauthorized, request("").Code)
	require.Equal(t, http.StatusUnauthorized, request("napi_client_invalid").Code)
	resp := request(issued.Token)
	require.Equal(t, http.StatusOK, resp.Code)
	require.Contains(t, resp.Body.String(), `"resource_id":"17"`)

	require.NoError(t, db.Model(&model.Channel{}).Where("id = ?", 17).Update("status", common.ChannelStatusManuallyDisabled).Error)
	require.Equal(t, http.StatusServiceUnavailable, request(issued.Token).Code)
	require.NoError(t, db.Model(&model.Channel{}).Where("id = ?", 17).Update("status", common.ChannelStatusEnabled).Error)
	_, err = store.UpdateGrants(issued.Credential.Id, 1, nil)
	require.NoError(t, err)
	require.Equal(t, http.StatusForbidden, request(issued.Token).Code)
}
