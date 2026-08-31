package model

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func findUsageSummaryItem(t *testing.T, items []UsageSummaryItem, tokenID, channelID int, modelName string) UsageSummaryItem {
	t.Helper()
	for _, item := range items {
		if item.TokenID == tokenID && item.ChannelID == channelID && item.ModelName == modelName {
			return item
		}
	}
	require.Failf(t, "usage summary item not found", "token=%d channel=%d model=%s", tokenID, channelID, modelName)
	return UsageSummaryItem{}
}

func TestGetUsageSummaryGroupsConsumptionLogs(t *testing.T) {
	truncateTables(t)
	require.NoError(t, DB.Create(&Channel{Id: 7, Name: "official"}).Error)

	logs := []*Log{
		{
			UserId:           1,
			Username:         "alice",
			CreatedAt:        1000,
			Type:             LogTypeConsume,
			TokenId:          11,
			TokenName:        "primary",
			ChannelId:        7,
			ModelName:        "gpt-a",
			PromptTokens:     10,
			CompletionTokens: 5,
			Quota:            100,
		},
		{
			UserId:           1,
			Username:         "alice",
			CreatedAt:        1100,
			Type:             LogTypeConsume,
			TokenId:          11,
			TokenName:        "primary",
			ChannelId:        7,
			ModelName:        "gpt-a",
			PromptTokens:     20,
			CompletionTokens: 8,
			Quota:            200,
		},
		{
			UserId:           1,
			Username:         "alice",
			CreatedAt:        1200,
			Type:             LogTypeConsume,
			TokenId:          11,
			TokenName:        "primary",
			ChannelId:        8,
			ModelName:        "gpt-a",
			PromptTokens:     3,
			CompletionTokens: 2,
			Quota:            50,
		},
		{
			UserId:           2,
			Username:         "bob",
			CreatedAt:        1300,
			Type:             LogTypeConsume,
			TokenId:          22,
			TokenName:        "backup",
			ChannelId:        7,
			ModelName:        "gpt-b",
			PromptTokens:     4,
			CompletionTokens: 1,
			Quota:            70,
		},
		{
			UserId:           1,
			Username:         "alice",
			CreatedAt:        1400,
			Type:             LogTypeError,
			TokenId:          11,
			TokenName:        "primary",
			ChannelId:        7,
			ModelName:        "gpt-a",
			PromptTokens:     999,
			CompletionTokens: 999,
			Quota:            999,
		},
		{
			UserId:           1,
			Username:         "alice",
			CreatedAt:        3000,
			Type:             LogTypeConsume,
			TokenId:          11,
			TokenName:        "primary",
			ChannelId:        7,
			ModelName:        "gpt-a",
			PromptTokens:     1000,
			CompletionTokens: 1000,
			Quota:            1000,
		},
	}
	for _, log := range logs {
		require.NoError(t, DB.Create(log).Error)
	}

	summary, err := GetUsageSummary(UsageSummaryFilters{
		StartTimestamp: 900,
		EndTimestamp:   2000,
	})
	require.NoError(t, err)

	require.Equal(t, int64(4), summary.TotalRequests)
	require.Equal(t, int64(37), summary.TotalInputTokens)
	require.Equal(t, int64(16), summary.TotalOutputTokens)
	require.Equal(t, int64(53), summary.TotalTokens)
	require.Equal(t, int64(420), summary.TotalQuota)
	require.Len(t, summary.Items, 3)
	// 并列总 Token 时按稳定维度键排序，而不是被 quota 值影响。
	require.Equal(t, 11, summary.Items[0].TokenID)
	require.Equal(t, 7, summary.Items[0].ChannelID)
	require.Equal(t, 11, summary.Items[1].TokenID)
	require.Equal(t, 8, summary.Items[1].ChannelID)
	require.Equal(t, 22, summary.Items[2].TokenID)
	require.Equal(t, 2, summary.Items[2].UserID)

	primary := findUsageSummaryItem(t, summary.Items, 11, 7, "gpt-a")
	require.Equal(t, int64(2), primary.Requests)
	require.Equal(t, int64(30), primary.InputTokens)
	require.Equal(t, int64(13), primary.OutputTokens)
	require.Equal(t, int64(43), primary.TotalTokens)
	require.Equal(t, int64(300), primary.Quota)
	require.Equal(t, "official", primary.ChannelName)
	require.Equal(t, "alice", primary.Username)

	missingChannel := findUsageSummaryItem(t, summary.Items, 11, 8, "gpt-a")
	require.Equal(t, int64(1), missingChannel.Requests)
	require.Empty(t, missingChannel.ChannelName)

	bob := findUsageSummaryItem(t, summary.Items, 22, 7, "gpt-b")
	require.Equal(t, int64(1), bob.Requests)
	require.Equal(t, "bob", bob.Username)
}

func TestGetUsageSummaryAppliesUserAndDimensionFilters(t *testing.T) {
	truncateTables(t)

	logs := []*Log{
		{
			UserId:           1,
			Username:         "alice",
			CreatedAt:        1000,
			Type:             LogTypeConsume,
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
			UserId:           1,
			Username:         "alice",
			CreatedAt:        1100,
			Type:             LogTypeConsume,
			TokenId:          12,
			TokenName:        "other",
			ChannelId:        7,
			ModelName:        "gpt-a",
			Group:            "vip",
			PromptTokens:     20,
			CompletionTokens: 8,
			Quota:            200,
		},
		{
			UserId:           2,
			Username:         "bob",
			CreatedAt:        1200,
			Type:             LogTypeConsume,
			TokenId:          11,
			TokenName:        "primary",
			ChannelId:        7,
			ModelName:        "gpt-a",
			Group:            "vip",
			PromptTokens:     30,
			CompletionTokens: 9,
			Quota:            300,
		},
	}
	for _, log := range logs {
		require.NoError(t, DB.Create(log).Error)
	}

	summary, err := GetUsageSummary(UsageSummaryFilters{
		UserID:         1,
		StartTimestamp: 900,
		EndTimestamp:   2000,
		TokenName:      "primary",
		ModelName:      "gpt-a",
		ChannelID:      7,
		Group:          "vip",
	})
	require.NoError(t, err)
	require.Len(t, summary.Items, 1)
	require.Equal(t, int64(1), summary.TotalRequests)
	require.Equal(t, int64(15), summary.TotalTokens)
	require.Equal(t, "alice", summary.Items[0].Username)
}

func TestGetUsageSummaryReturnsEmptyTotals(t *testing.T) {
	truncateTables(t)

	summary, err := GetUsageSummary(UsageSummaryFilters{
		StartTimestamp: 900,
		EndTimestamp:   2000,
	})
	require.NoError(t, err)
	require.Empty(t, summary.Items)
	require.Equal(t, int64(0), summary.TotalRequests)
	require.Equal(t, int64(0), summary.TotalInputTokens)
	require.Equal(t, int64(0), summary.TotalOutputTokens)
	require.Equal(t, int64(0), summary.TotalTokens)
	require.Equal(t, int64(0), summary.TotalQuota)
}

func TestGetUsageSummaryIncludesDailyTrendWhenRequested(t *testing.T) {
	truncateTables(t)

	logs := []*Log{
		{
			UserId:           1,
			Username:         "alice",
			CreatedAt:        86400 + 100,
			Type:             LogTypeConsume,
			TokenId:          11,
			TokenName:        "primary",
			ChannelId:        7,
			ModelName:        "gpt-a",
			PromptTokens:     10,
			CompletionTokens: 5,
			Quota:            100,
		},
		{
			UserId:           1,
			Username:         "alice",
			CreatedAt:        2*86400 + 100,
			Type:             LogTypeConsume,
			TokenId:          11,
			TokenName:        "primary",
			ChannelId:        7,
			ModelName:        "gpt-a",
			PromptTokens:     20,
			CompletionTokens: 8,
			Quota:            200,
		},
		{
			UserId:           1,
			Username:         "alice",
			CreatedAt:        2*86400 + 200,
			Type:             LogTypeError,
			TokenId:          11,
			TokenName:        "primary",
			ChannelId:        7,
			ModelName:        "gpt-a",
			PromptTokens:     999,
			CompletionTokens: 999,
			Quota:            999,
		},
	}
	for _, log := range logs {
		require.NoError(t, DB.Create(log).Error)
	}

	summary, err := GetUsageSummary(UsageSummaryFilters{
		StartTimestamp: 86400,
		EndTimestamp:   3 * 86400,
		IncludeTrend:   true,
	})
	require.NoError(t, err)
	require.Len(t, summary.Trend, 2)
	require.Equal(t, int64(86400), summary.Trend[0].Timestamp)
	require.Equal(t, int64(1), summary.Trend[0].Requests)
	require.Equal(t, int64(15), summary.Trend[0].TotalTokens)
	require.Equal(t, int64(2*86400), summary.Trend[1].Timestamp)
	require.Equal(t, int64(1), summary.Trend[1].Requests)
	require.Equal(t, int64(28), summary.Trend[1].TotalTokens)
}
