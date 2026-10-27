package controller

import (
	"context"
	"net/url"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

// PongReportQuery只允许当前报表的固定查询字段，拒绝重复参数与隐式扩权参数。
func PongReportQuery(keys ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		allowed := make(map[string]bool, len(keys))
		for _, key := range keys {
			allowed[key] = true
		}
		values, err := url.ParseQuery(c.Request.URL.RawQuery)
		if err != nil {
			c.Abort()
			pongReportError(c, 400)
			return
		}
		for key, values := range values {
			if !allowed[key] || len(values) != 1 {
				c.Abort()
				pongReportError(c, 400)
				return
			}
		}
		ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
		defer cancel()
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}

func GetPongReportUsage(c *gin.Context) {
	filters, err := parseUsageSummaryFilters(c)
	scope := c.DefaultQuery("scope", "all")
	if err != nil || (scope != "all" && scope != "self") || filters.StartTimestamp <= 0 || filters.EndTimestamp <= 0 || filters.EndTimestamp < filters.StartTimestamp || filters.EndTimestamp-filters.StartTimestamp > 92*86400 {
		pongReportError(c, 400)
		return
	}
	if scope == "self" {
		filters.UserID = c.GetInt("id")
	}
	summary, err := model.GetUsageSummaryContext(c.Request.Context(), filters)
	if err != nil {
		pongReportError(c, 503)
		return
	}
	common.ApiSuccess(c, summary)
}

// privateQuotaSubjectData将观察DTO与配置DTO隔离，防止将来新增配置字段进入报表。
func privateQuotaSubjectData(c *gin.Context, row model.PrivateQuotaSubject) any {
	if !c.GetBool("pong_report_readonly") {
		return privateQuotaSubjectDTO(row)
	}
	return struct {
		ID            int64  `json:"id"`
		Name          string `json:"name"`
		Enabled       bool   `json:"enabled"`
		LastAttemptAt int64  `json:"last_attempt_at"`
		LastSuccessAt int64  `json:"last_success_at"`
		ErrorCode     string `json:"error_code"`
	}{row.ID, row.Name, row.Enabled, row.LastAttemptAt, row.LastSuccessAt, row.ErrorCode}
}
