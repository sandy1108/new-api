package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type usageSummaryResponse struct {
	Success bool               `json:"success"`
	Message string             `json:"message"`
	Data    model.UsageSummary `json:"data"`
}

func setupUsageSummaryControllerDB(t *testing.T) {
	t.Helper()
	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.Log{}))

	require.NoError(t, db.Create(&model.Channel{Id: 7, Name: "official"}).Error)
	logs := []*model.Log{
		{
			UserId:           1,
			Username:         "alice",
			CreatedAt:        1000,
			Type:             model.LogTypeConsume,
			TokenId:          11,
			TokenName:        "primary",
			ChannelId:        7,
			ModelName:        "gpt-a",
			Group:            "vip",
			PromptTokens:     10,
			CompletionTokens: 5,
			Quota:            100,
		},
		{
			UserId:           2,
			Username:         "bob",
			CreatedAt:        1100,
			Type:             model.LogTypeConsume,
			TokenId:          22,
			TokenName:        "backup",
			ChannelId:        7,
			ModelName:        "gpt-b",
			Group:            "vip",
			PromptTokens:     20,
			CompletionTokens: 8,
			Quota:            200,
		},
	}
	for _, log := range logs {
		require.NoError(t, db.Create(log).Error)
	}
}

func decodeUsageSummaryResponse(t *testing.T, recorder *httptest.ResponseRecorder) usageSummaryResponse {
	t.Helper()
	require.Equal(t, http.StatusOK, recorder.Code)
	var payload usageSummaryResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &payload))
	return payload
}

func TestGetLogsUsageSummaryAppliesAdminFilters(t *testing.T) {
	setupUsageSummaryControllerDB(t)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Set("role", common.RoleAdminUser)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/log/usage-summary?start_timestamp=900&end_timestamp=2000&username=alice&channel=7&group=vip&include_trend=true", nil)

	GetLogsUsageSummary(ctx)

	payload := decodeUsageSummaryResponse(t, recorder)
	require.True(t, payload.Success, payload.Message)
	require.Len(t, payload.Data.Items, 1)
	require.Equal(t, "alice", payload.Data.Items[0].Username)
	require.Equal(t, int64(1), payload.Data.TotalRequests)
	require.Equal(t, int64(15), payload.Data.TotalTokens)
	require.Len(t, payload.Data.Trend, 1)
	require.Equal(t, int64(15), payload.Data.Trend[0].TotalTokens)
}

func TestGetLogsSelfUsageSummaryRestrictsAuthenticatedUser(t *testing.T) {
	setupUsageSummaryControllerDB(t)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Set("id", 1)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/log/self/usage-summary?start_timestamp=900&end_timestamp=2000&username=bob", nil)

	GetLogsSelfUsageSummary(ctx)

	payload := decodeUsageSummaryResponse(t, recorder)
	require.True(t, payload.Success, payload.Message)
	require.Len(t, payload.Data.Items, 1)
	require.Equal(t, "alice", payload.Data.Items[0].Username)
	require.Equal(t, int64(1), payload.Data.TotalRequests)
}

func TestGetLogsUsageSummaryRejectsInvalidTimeRange(t *testing.T) {
	setupUsageSummaryControllerDB(t)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/log/usage-summary?start_timestamp=2000&end_timestamp=1000", nil)

	GetLogsUsageSummary(ctx)

	payload := decodeUsageSummaryResponse(t, recorder)
	require.False(t, payload.Success)
	require.Equal(t, "invalid time range", payload.Message)
}
