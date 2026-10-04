package middleware

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestClientAccessAuditLogsMaskedRequestOutcome(t *testing.T) {
	gin.SetMode(gin.TestMode)
	previousWriter := gin.DefaultWriter
	var logs bytes.Buffer
	gin.DefaultWriter = &logs
	t.Cleanup(func() { gin.DefaultWriter = previousWriter })

	router := gin.New()
	router.GET("/api/client-access/v1/codex/usage", ClientAccessAudit(), func(c *gin.Context) {
		c.Set(clientAccessCredentialIDContextKey, 42)
		c.Set(clientAccessAuditKeyContextKey, "secret-token-hash")
		c.Set(ClientAccessScopeContextKey, "codex.quota.read")
		c.Set(ClientAccessResourceIDContextKey, "7384920")
		SetClientAccessErrorType(c, "CODEX_UPSTREAM_RATE_LIMITED")
		c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "code": "CODEX_UPSTREAM_RATE_LIMITED", "debug": "oauth-secret"})
	})
	request := httptest.NewRequest(http.MethodGet, "/api/client-access/v1/codex/usage?debug=query-secret", nil)
	request.Header.Set("Authorization", "Bearer napi_client_never-log-this")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	require.Equal(t, http.StatusServiceUnavailable, response.Code)
	logOutput := logs.String()
	for _, secret := range []string{"napi_client_never-log-this", "secret-token-hash", "7384920", "oauth-secret", "query-secret"} {
		require.NotContains(t, logOutput, secret)
	}
	eventStart := strings.Index(logOutput, "client_access_audit ")
	require.NotEqual(t, -1, eventStart, logOutput)
	eventJSON := strings.TrimSpace(logOutput[eventStart+len("client_access_audit "):])
	var event map[string]any
	require.NoError(t, json.Unmarshal([]byte(eventJSON), &event))
	require.Equal(t, "client_access_request", event["event"])
	require.Equal(t, float64(42), event["credential_id"])
	require.Equal(t, "codex.quota.read", event["scope"])
	require.NotEmpty(t, event["resource_fingerprint"])
	require.Equal(t, float64(http.StatusServiceUnavailable), event["status"])
	require.Equal(t, "failure", event["result"])
	require.Equal(t, "CODEX_UPSTREAM_RATE_LIMITED", event["error_type"])
	require.Equal(t, "/api/client-access/v1/codex/usage", event["route"])
}

func TestClientAccessAuditLogsSuccessfulRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	previousWriter := gin.DefaultWriter
	var logs bytes.Buffer
	gin.DefaultWriter = &logs
	t.Cleanup(func() { gin.DefaultWriter = previousWriter })

	router := gin.New()
	router.GET("/api/client-access/v1/codex/usage", ClientAccessAudit(), func(c *gin.Context) {
		c.Set(clientAccessCredentialIDContextKey, 42)
		c.Set(clientAccessAuditKeyContextKey, "secret-token-hash")
		c.Set(ClientAccessScopeContextKey, "codex.quota.read")
		c.Set(ClientAccessResourceIDContextKey, "7384920")
		c.Status(http.StatusOK)
	})
	request := httptest.NewRequest(http.MethodGet, "/api/client-access/v1/codex/usage", nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	require.Equal(t, http.StatusOK, response.Code)
	eventStart := strings.Index(logs.String(), "client_access_audit ")
	require.NotEqual(t, -1, eventStart, logs.String())
	eventJSON := strings.TrimSpace(logs.String()[eventStart+len("client_access_audit "):])
	var event map[string]any
	require.NoError(t, json.Unmarshal([]byte(eventJSON), &event))
	require.Equal(t, "success", event["result"])
	require.Equal(t, float64(http.StatusOK), event["status"])
	require.NotContains(t, event, "error_type")
}

func TestClientAccessAuditLogsPanicWithoutPanicDetails(t *testing.T) {
	gin.SetMode(gin.TestMode)
	previousWriter := gin.DefaultWriter
	previousErrorWriter := gin.DefaultErrorWriter
	var logs bytes.Buffer
	gin.DefaultWriter = &logs
	gin.DefaultErrorWriter = io.Discard
	t.Cleanup(func() {
		gin.DefaultWriter = previousWriter
		gin.DefaultErrorWriter = previousErrorWriter
	})

	router := gin.New()
	router.Use(gin.CustomRecovery(func(c *gin.Context, _ any) {
		c.AbortWithStatus(http.StatusInternalServerError)
	}))
	router.GET("/api/client-access/v1/codex/usage", ClientAccessAudit(), func(c *gin.Context) {
		c.Set(clientAccessCredentialIDContextKey, 42)
		c.Set(clientAccessAuditKeyContextKey, "secret-token-hash")
		c.Set(ClientAccessScopeContextKey, "codex.quota.read")
		c.Set(ClientAccessResourceIDContextKey, "7384920")
		panic("panic-sensitive-value")
	})
	request := httptest.NewRequest(http.MethodGet, "/api/client-access/v1/codex/usage", nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	require.Equal(t, http.StatusInternalServerError, response.Code)
	var errorResponse struct {
		Success bool   `json:"success"`
		Code    string `json:"code"`
		Message string `json:"message"`
	}
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &errorResponse))
	require.False(t, errorResponse.Success)
	require.Equal(t, "CLIENT_ACCESS_INTERNAL_ERROR", errorResponse.Code)
	require.Equal(t, http.StatusText(http.StatusInternalServerError), errorResponse.Message)
	require.NotContains(t, response.Body.String(), "panic-sensitive-value")
	logOutput := logs.String()
	require.NotContains(t, logOutput, "panic-sensitive-value")
	eventStart := strings.Index(logOutput, "client_access_audit ")
	require.NotEqual(t, -1, eventStart, logOutput)
	eventJSON := strings.TrimSpace(logOutput[eventStart+len("client_access_audit "):])
	var event map[string]any
	require.NoError(t, json.Unmarshal([]byte(eventJSON), &event))
	require.Equal(t, float64(http.StatusInternalServerError), event["status"])
	require.Equal(t, "failure", event["result"])
	require.Equal(t, "PANIC", event["error_type"])
}

func TestSetUpLoggerRedactsClientAccessQueryString(t *testing.T) {
	gin.SetMode(gin.TestMode)
	previousWriter := gin.DefaultWriter
	var logs bytes.Buffer
	gin.DefaultWriter = &logs
	t.Cleanup(func() { gin.DefaultWriter = previousWriter })

	router := gin.New()
	SetUpLogger(router)
	router.GET("/api/client-access/v1/codex/usage", func(c *gin.Context) {
		require.Equal(t, "never-log-client-token", c.Query("token"))
		c.Status(http.StatusUnauthorized)
	})
	request := httptest.NewRequest(http.MethodGet, "/api/client-access/v1/codex/usage?token=never-log-client-token", nil)
	router.ServeHTTP(httptest.NewRecorder(), request)

	require.NotContains(t, logs.String(), "never-log-client-token")
	require.Contains(t, logs.String(), "GET /api/client-access/v1/codex/usage")
}
