package controller

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service/clientaccess"
	"github.com/gin-gonic/gin"
)

func GetClientCodexUsage(c *gin.Context) {
	channelID, err := strconv.Atoi(c.GetString(middleware.ClientAccessResourceIDContextKey))
	if err != nil || channelID <= 0 {
		writeClientCodexUsageError(c, http.StatusServiceUnavailable, "CLIENT_ACCESS_RESOURCE_UNAVAILABLE")
		return
	}
	data, err := clientaccess.NewStore(model.DB).GetCodexUsage(c.Request.Context(), channelID)
	if err != nil {
		status := http.StatusInternalServerError
		code := "CLIENT_ACCESS_INTERNAL_ERROR"
		switch {
		case errors.Is(err, clientaccess.ErrResourceUnavailable):
			status = http.StatusServiceUnavailable
			code = "CLIENT_ACCESS_RESOURCE_UNAVAILABLE"
		case errors.Is(err, clientaccess.ErrCodexUpstreamAuth):
			status = http.StatusServiceUnavailable
			code = "CODEX_UPSTREAM_AUTH_UNAVAILABLE"
		case errors.Is(err, clientaccess.ErrCodexUpstreamLimited):
			status = http.StatusServiceUnavailable
			code = "CODEX_UPSTREAM_RATE_LIMITED"
		case errors.Is(err, clientaccess.ErrCodexUpstreamUnavailable):
			status = http.StatusServiceUnavailable
			code = "CODEX_UPSTREAM_UNAVAILABLE"
		case errors.Is(err, clientaccess.ErrClientAccessRedisUnavailable):
			status = http.StatusServiceUnavailable
			code = "CLIENT_ACCESS_REDIS_UNAVAILABLE"
		case errors.Is(err, clientaccess.ErrCodexUsageFetchLockTimeout):
			status = http.StatusServiceUnavailable
			code = "CODEX_USAGE_FETCH_BUSY"
		}
		writeClientCodexUsageError(c, status, code)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": data})
}

func writeClientCodexUsageError(c *gin.Context, status int, code string) {
	middleware.SetClientAccessErrorType(c, code)
	c.JSON(status, gin.H{"success": false, "code": code, "message": http.StatusText(status)})
}
