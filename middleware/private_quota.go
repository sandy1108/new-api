package middleware

import (
	"mime"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// PrivateQuotaSessionOnly只接受有效后台会话；不向PAT或模型令牌扩大权限。
func PrivateQuotaSessionOnly() gin.HandlerFunc {
	return func(c *gin.Context) {
		if _, ok := GetSessionAuthIdentity(c); !ok {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"success": false, "code": "PRIVATE_QUOTA_SESSION_REQUIRED", "message": "Dashboard session required"})
			return
		}
		c.Next()
	}
}

// PrivateQuotaWriteGuard在本地HTTP也启用；JSON、自定义头和同源校验共同防CSRF。
func PrivateQuotaWriteGuard() gin.HandlerFunc {
	return func(c *gin.Context) {
		origin, ok := requestBrowserOrigin(c.Request)
		media, _, err := mime.ParseMediaType(c.GetHeader("Content-Type"))
		if !ok || !isAllowedSessionOrigin(c.Request, origin) || err != nil || media != "application/json" ||
			c.GetHeader("X-PongAPI-Request") != "1" || strings.EqualFold(c.GetHeader("Sec-Fetch-Site"), "cross-site") {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"success": false, "code": "PRIVATE_QUOTA_ORIGIN_FORBIDDEN", "message": "Same-origin JSON request required"})
			return
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 8192)
		c.Next()
	}
}
