package controller

import (
	"fmt"
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

func parseUsageSummaryFilters(c *gin.Context) (model.UsageSummaryFilters, error) {
	startTimestamp, err := parseOptionalUsageSummaryTimestamp(c, "start_timestamp")
	if err != nil {
		return model.UsageSummaryFilters{}, err
	}
	endTimestamp, err := parseOptionalUsageSummaryTimestamp(c, "end_timestamp")
	if err != nil {
		return model.UsageSummaryFilters{}, err
	}
	if startTimestamp != 0 && endTimestamp != 0 && endTimestamp < startTimestamp {
		return model.UsageSummaryFilters{}, fmt.Errorf("invalid time range")
	}
	includeTrend, err := parseOptionalUsageSummaryBool(c, "include_trend")
	if err != nil {
		return model.UsageSummaryFilters{}, err
	}

	channelID, err := parseOptionalUsageSummaryInt(c, "channel")
	if err != nil {
		return model.UsageSummaryFilters{}, err
	}

	return model.UsageSummaryFilters{
		StartTimestamp: startTimestamp,
		EndTimestamp:   endTimestamp,
		IncludeTrend:   includeTrend,
		ModelName:      c.Query("model_name"),
		Username:       c.Query("username"),
		TokenName:      c.Query("token_name"),
		ChannelID:      channelID,
		Group:          c.Query("group"),
	}, nil
}

func parseOptionalUsageSummaryBool(c *gin.Context, key string) (bool, error) {
	raw := c.Query(key)
	if raw == "" {
		return false, nil
	}
	value, err := strconv.ParseBool(raw)
	if err != nil {
		return false, fmt.Errorf("invalid %s", key)
	}
	return value, nil
}

func parseOptionalUsageSummaryTimestamp(c *gin.Context, key string) (int64, error) {
	raw := c.Query(key)
	if raw == "" {
		return 0, nil
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || value < 0 {
		return 0, fmt.Errorf("invalid %s", key)
	}
	return value, nil
}

func parseOptionalUsageSummaryInt(c *gin.Context, key string) (int, error) {
	raw := c.Query(key)
	if raw == "" {
		return 0, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < 0 {
		return 0, fmt.Errorf("invalid %s", key)
	}
	return value, nil
}

// GetLogsUsageSummary 返回管理员可见范围内的消费日志聚合结果。
func GetLogsUsageSummary(c *gin.Context) {
	filters, err := parseUsageSummaryFilters(c)
	if err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}

	summary, err := model.GetUsageSummary(filters)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, summary)
}

// GetLogsSelfUsageSummary 强制按认证用户 ID 筛选，避免 username 参数越权。
func GetLogsSelfUsageSummary(c *gin.Context) {
	filters, err := parseUsageSummaryFilters(c)
	if err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	filters.UserID = c.GetInt("id")
	filters.Username = ""

	summary, err := model.GetUsageSummary(filters)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, summary)
}
