package middleware

import (
	"errors"
	"net/http"

	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service/reportaccess"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// PongReportSessionOnly复用官方会话身份，拒绝PAT和模型/客户端凭据的权限扩张。
func PongReportSessionOnly() gin.HandlerFunc {
	return func(c *gin.Context) {
		identity, ok := GetSessionAuthIdentity(c)
		_, pat := model.ParseAccessTokenSessionID(identity.SessionID)
		if !ok || pat {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"success": false, "code": "PONG_REPORT_SESSION_REQUIRED", "message": "Dashboard session required"})
			return
		}
		c.Next()
	}
}

// RequirePongReportCapability对每次请求查询权威主库，绝不改写role或冒用Root。
func RequirePongReportCapability(permission string) gin.HandlerFunc {
	return func(c *gin.Context) {
		identity, ok := GetSessionAuthIdentity(c)
		if !ok {
			c.AbortWithStatus(http.StatusForbidden)
			return
		}
		caps, err := (reportaccess.Store{DB: model.DB}).Capabilities(c.Request.Context(), identity)
		if err != nil {
			status := http.StatusServiceUnavailable
			if errors.Is(err, model.ErrPrivateReportForbidden) || errors.Is(err, gorm.ErrRecordNotFound) {
				status = http.StatusForbidden
			}
			c.AbortWithStatusJSON(status, gin.H{"success": false, "code": "PONG_REPORT_ACCESS_UNAVAILABLE", "message": http.StatusText(status)})
			return
		}
		allowed := permission == reportaccess.UsageReadAll && caps.UsageReadAll || permission == reportaccess.SubscriptionReadAll && caps.SubscriptionReadAll || permission == reportaccess.ManageGrants && caps.CanManageGrants
		if !allowed {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"success": false, "code": "PONG_REPORT_FORBIDDEN", "message": "Forbidden"})
			return
		}
		c.Set("pong_report_readonly", true)
		c.Next()
	}
}

// PongReportWriteGuard与现有私有账本共用已验证的同源JSON护栏。
func PongReportWriteGuard() gin.HandlerFunc { return PrivateQuotaWriteGuard() }

// PongReportReadLimit独立于可关闭的全局限流；每用户每分钟60次覆盖五项账本读取及交互。
func PongReportReadLimit(scope string) gin.HandlerFunc {
	return userRateLimitFactory(60, 60, "pong-reports:"+scope)
}
