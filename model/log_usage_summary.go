package model

import (
	"errors"
	"sort"

	"github.com/QuantumNous/new-api/common"
)

// UsageSummaryFilters 定义日志用量聚合的筛选条件。
// UserID 仅由普通用户控制器填充，用于防止依赖客户端提供的 username。
type UsageSummaryFilters struct {
	UserID         int
	StartTimestamp int64
	EndTimestamp   int64
	ModelName      string
	Username       string
	TokenName      string
	ChannelID      int
	Group          string
}

// UsageSummaryItem 是一个 Token、渠道、模型维度的聚合行。
// UserID 和 Username 用于管理员查询多个用户时避免同名维度被混合。
type UsageSummaryItem struct {
	UserID       int    `json:"user_id" gorm:"column:user_id"`
	Username     string `json:"username" gorm:"column:username"`
	TokenID      int    `json:"token_id" gorm:"column:token_id"`
	TokenName    string `json:"token_name" gorm:"column:token_name"`
	ChannelID    int    `json:"channel_id" gorm:"column:channel_id"`
	ChannelName  string `json:"channel_name,omitempty" gorm:"-"`
	ModelName    string `json:"model_name" gorm:"column:model_name"`
	Requests     int64  `json:"requests" gorm:"column:requests"`
	InputTokens  int64  `json:"input_tokens" gorm:"column:input_tokens"`
	OutputTokens int64  `json:"output_tokens" gorm:"column:output_tokens"`
	TotalTokens  int64  `json:"total_tokens" gorm:"-"`
	Quota        int64  `json:"quota" gorm:"column:quota"`
}

// UsageSummary 是 usage-summary 接口返回的总计和明细。
type UsageSummary struct {
	TotalRequests     int64              `json:"total_requests"`
	TotalInputTokens  int64              `json:"total_input_tokens"`
	TotalOutputTokens int64              `json:"total_output_tokens"`
	TotalTokens       int64              `json:"total_tokens"`
	TotalQuota        int64              `json:"total_quota"`
	Items             []UsageSummaryItem `json:"items"`
}

var errUsageSummaryQuery = errors.New("查询统计数据失败")

// GetUsageSummary 在日志数据库中直接完成消费日志聚合。
// 不使用分页，避免客户端为统计数据重复执行 COUNT/OFFSET 查询。
func GetUsageSummary(filters UsageSummaryFilters) (UsageSummary, error) {
	var summary UsageSummary
	items := make([]UsageSummaryItem, 0)

	tx := LOG_DB.Table("logs").Select(`
		user_id,
		username,
		token_id,
		token_name,
		channel_id,
		model_name,
		COUNT(*) AS requests,
		COALESCE(SUM(prompt_tokens), 0) AS input_tokens,
		COALESCE(SUM(completion_tokens), 0) AS output_tokens,
		COALESCE(SUM(quota), 0) AS quota`).
		Where("type = ?", LogTypeConsume)

	if filters.UserID != 0 {
		tx = tx.Where("user_id = ?", filters.UserID)
	}
	if filters.Username != "" {
		var err error
		tx, err = applyExplicitLogTextFilter(tx, "username", filters.Username)
		if err != nil {
			return summary, err
		}
	}
	if filters.TokenName != "" {
		tx = tx.Where("token_name = ?", filters.TokenName)
	}
	if filters.ModelName != "" {
		var err error
		tx, err = applyExplicitLogTextFilter(tx, "model_name", filters.ModelName)
		if err != nil {
			return summary, err
		}
	}
	if filters.StartTimestamp != 0 {
		tx = tx.Where("created_at >= ?", filters.StartTimestamp)
	}
	if filters.EndTimestamp != 0 {
		tx = tx.Where("created_at <= ?", filters.EndTimestamp)
	}
	if filters.ChannelID != 0 {
		tx = tx.Where("channel_id = ?", filters.ChannelID)
	}
	if filters.Group != "" {
		tx = tx.Where(logGroupCol+" = ?", filters.Group)
	}

	if err := tx.Group("user_id, username, token_id, token_name, channel_id, model_name").Scan(&items).Error; err != nil {
		common.SysError("failed to query usage summary: " + err.Error())
		return summary, errUsageSummaryQuery
	}

	if err := fillUsageSummaryChannelNames(items); err != nil {
		return summary, err
	}
	for i := range items {
		items[i].TotalTokens = items[i].InputTokens + items[i].OutputTokens
		summary.TotalRequests += items[i].Requests
		summary.TotalInputTokens += items[i].InputTokens
		summary.TotalOutputTokens += items[i].OutputTokens
		summary.TotalTokens += items[i].TotalTokens
		summary.TotalQuota += items[i].Quota
	}

	sort.SliceStable(items, func(i, j int) bool {
		return usageSummaryItemLess(items[i], items[j])
	})
	summary.Items = items
	return summary, nil
}

func usageSummaryItemLess(left, right UsageSummaryItem) bool {
	if left.TotalTokens != right.TotalTokens {
		return left.TotalTokens > right.TotalTokens
	}
	if left.UserID != right.UserID {
		return left.UserID < right.UserID
	}
	if left.Username != right.Username {
		return left.Username < right.Username
	}
	if left.TokenID != right.TokenID {
		return left.TokenID < right.TokenID
	}
	if left.TokenName != right.TokenName {
		return left.TokenName < right.TokenName
	}
	if left.ChannelID != right.ChannelID {
		return left.ChannelID < right.ChannelID
	}
	if left.ModelName != right.ModelName {
		return left.ModelName < right.ModelName
	}
	return false
}

func fillUsageSummaryChannelNames(items []UsageSummaryItem) error {
	channelIDs := make([]int, 0)
	seen := make(map[int]struct{})
	for _, item := range items {
		if item.ChannelID == 0 {
			continue
		}
		if _, ok := seen[item.ChannelID]; ok {
			continue
		}
		seen[item.ChannelID] = struct{}{}
		channelIDs = append(channelIDs, item.ChannelID)
	}
	if len(channelIDs) == 0 {
		return nil
	}

	channelNames := make(map[int]string, len(channelIDs))
	if common.MemoryCacheEnabled {
		for _, channelID := range channelIDs {
			channel, err := CacheGetChannel(channelID)
			if err == nil {
				channelNames[channelID] = channel.Name
			}
		}
	} else {
		var channels []struct {
			ID   int    `gorm:"column:id"`
			Name string `gorm:"column:name"`
		}
		if err := DB.Table("channels").Select("id, name").Where("id IN ?", channelIDs).Find(&channels).Error; err != nil {
			common.SysError("failed to query usage summary channel names: " + err.Error())
			return errUsageSummaryQuery
		}
		for _, channel := range channels {
			channelNames[channel.ID] = channel.Name
		}
	}

	for i := range items {
		if name, ok := channelNames[items[i].ChannelID]; ok {
			items[i].ChannelName = name
		}
	}
	return nil
}
