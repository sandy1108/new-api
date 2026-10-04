package middleware

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service/clientaccess"
	"github.com/gin-gonic/gin"
)

const clientAccessCredentialIDContextKey = "client_access_credential_id"
const clientAccessAuditKeyContextKey = "client_access_audit_key"
const ClientAccessResourceIDContextKey = "client_access_resource_id"
const ClientAccessScopeContextKey = "client_access_scope"
const ClientAccessErrorTypeContextKey = "client_access_error_type"

// SetClientAccessErrorType 供客户端访问错误响应写入稳定错误码，供脱敏审计使用。
func SetClientAccessErrorType(c *gin.Context, code string) {
	c.Set(ClientAccessErrorTypeContextKey, code)
}

func clientAccessError(c *gin.Context, status int, code string) {
	SetClientAccessErrorType(c, code)
	c.AbortWithStatusJSON(status, gin.H{"success": false, "code": code, "message": http.StatusText(status)})
}

// ClientAccessAuth 独立于后台会话和模型 API Key，仅接受客户端 Bearer 凭据。
func ClientAccessAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		parts := strings.Fields(c.GetHeader("Authorization"))
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			clientAccessError(c, http.StatusUnauthorized, "CLIENT_ACCESS_INVALID_CREDENTIAL")
			return
		}
		credential, err := clientaccess.NewStore(model.DB).FindCredentialByToken(parts[1])
		if err != nil {
			if errors.Is(err, clientaccess.ErrCredentialUnavailable) {
				clientAccessError(c, http.StatusUnauthorized, "CLIENT_ACCESS_INVALID_CREDENTIAL")
			} else {
				clientAccessError(c, http.StatusInternalServerError, "CLIENT_ACCESS_INTERNAL_ERROR")
			}
			return
		}
		c.Set(clientAccessCredentialIDContextKey, credential.Id)
		c.Set(clientAccessAuditKeyContextKey, credential.TokenHash)
		if now := time.Now().Unix(); credential.LastUsedTime <= now-300 {
			if err := clientaccess.NewStore(model.DB).TouchLastUsed(credential.Id, now); err != nil {
				common.SysError("client access last used update failed: " + err.Error())
			}
		}
		c.Next()
	}
}

// ClientAccessRateLimit 在 Scope/资源查询前按凭据计数；Redis 故障时失败关闭。
func ClientAccessRateLimit() gin.HandlerFunc {
	return func(c *gin.Context) {
		credentialID := c.GetInt(clientAccessCredentialIDContextKey)
		if credentialID <= 0 {
			clientAccessError(c, http.StatusUnauthorized, "CLIENT_ACCESS_INVALID_CREDENTIAL")
			return
		}
		allowed, err := clientaccess.LimitCredentialRequest(c.Request.Context(), credentialID)
		if err != nil {
			clientAccessError(c, http.StatusServiceUnavailable, "CLIENT_ACCESS_REDIS_UNAVAILABLE")
			return
		}
		if !allowed {
			clientAccessError(c, http.StatusTooManyRequests, "CLIENT_ACCESS_RATE_LIMITED")
			return
		}
		c.Next()
	}
}

// RequireClientScope 逐次从主库检查 Grant 和资源；前端筛选与缓存都不能替代该判断。
func RequireClientScope(scope string) gin.HandlerFunc {
	return func(c *gin.Context) {
		credentialID := c.GetInt(clientAccessCredentialIDContextKey)
		if credentialID <= 0 {
			clientAccessError(c, http.StatusUnauthorized, "CLIENT_ACCESS_INVALID_CREDENTIAL")
			return
		}
		c.Set(ClientAccessScopeContextKey, scope)
		resourceID, err := clientaccess.NewStore(model.DB).AuthorizeScope(credentialID, scope)
		if err != nil {
			switch {
			case errors.Is(err, clientaccess.ErrScopeDenied):
				clientAccessError(c, http.StatusForbidden, "CLIENT_ACCESS_SCOPE_DENIED")
			case errors.Is(err, clientaccess.ErrResourceUnavailable):
				clientAccessError(c, http.StatusServiceUnavailable, "CLIENT_ACCESS_RESOURCE_UNAVAILABLE")
			default:
				clientAccessError(c, http.StatusInternalServerError, "CLIENT_ACCESS_INTERNAL_ERROR")
			}
			return
		}
		c.Set(ClientAccessResourceIDContextKey, resourceID)
		c.Next()
	}
}
