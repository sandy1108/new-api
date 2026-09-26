package middleware

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strconv"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
)

type clientAccessRequestAuditEvent struct {
	Event               string `json:"event"`
	CredentialID        int    `json:"credential_id"`
	Scope               string `json:"scope,omitempty"`
	ResourceFingerprint string `json:"resource_fingerprint,omitempty"`
	Method              string `json:"method"`
	Route               string `json:"route"`
	Status              int    `json:"status"`
	Result              string `json:"result"`
	ErrorType           string `json:"error_type,omitempty"`
	DurationMS          int64  `json:"duration_ms"`
}

// ClientAccessAudit 为客户端访问路由写脱敏结构化请求日志，不记录请求头、查询参数或响应正文。
func ClientAccessAudit() gin.HandlerFunc {
	return func(c *gin.Context) {
		startedAt := time.Now()
		defer func() {
			if recovered := recover(); recovered != nil {
				// 只记录固定错误类型，不把 panic 原文写入客户端访问审计。
				logClientAccessRequestAudit(c, startedAt, http.StatusInternalServerError, true)
				// 客户端路由不能交给全局恢复器返回内部 panic 文本。
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
					"success": false,
					"code":    "CLIENT_ACCESS_INTERNAL_ERROR",
					"message": http.StatusText(http.StatusInternalServerError),
				})
				return
			}
			logClientAccessRequestAudit(c, startedAt, c.Writer.Status(), false)
		}()
		c.Next()
	}
}

func logClientAccessRequestAudit(c *gin.Context, startedAt time.Time, status int, panicked bool) {
	result := "success"
	if status >= http.StatusBadRequest {
		result = "failure"
	}
	errorType := c.GetString(ClientAccessErrorTypeContextKey)
	if panicked {
		errorType = "PANIC"
	} else if errorType == "" && status >= http.StatusBadRequest {
		errorType = "HTTP_" + strconv.Itoa(status)
	}
	event := clientAccessRequestAuditEvent{
		Event:               "client_access_request",
		CredentialID:        c.GetInt(clientAccessCredentialIDContextKey),
		Scope:               c.GetString(ClientAccessScopeContextKey),
		ResourceFingerprint: clientAccessResourceFingerprint(c.GetString(ClientAccessResourceIDContextKey), c.GetString(clientAccessAuditKeyContextKey)),
		Method:              c.Request.Method,
		Route:               c.FullPath(),
		Status:              status,
		Result:              result,
		ErrorType:           errorType,
		DurationMS:          time.Since(startedAt).Milliseconds(),
	}
	encoded, err := common.Marshal(event)
	if err != nil {
		common.SysError("client access audit event encoding failed")
		return
	}
	common.SysLog("client_access_audit " + string(encoded))
}

func clientAccessResourceFingerprint(resourceID, key string) string {
	if resourceID == "" || key == "" {
		return ""
	}
	mac := hmac.New(sha256.New, []byte(key))
	_, _ = mac.Write([]byte("client-access-resource:" + resourceID))
	return hex.EncodeToString(mac.Sum(nil)[:12])
}
