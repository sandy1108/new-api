package router

import (
	"github.com/QuantumNous/new-api/controller"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/service/clientaccess"
	"github.com/gin-gonic/gin"
)

// registerClientAccessRoutes 将私有凭据管理集中在独立路由文件，不改官方 Token 路由。
func registerClientAccessRoutes(api *gin.RouterGroup) {
	manager := api.Group("/client-access")
	manager.Use(middleware.RootAuth(), middleware.DisableCache())
	manager.GET("/scopes", controller.GetClientAccessScopes)
	manager.GET("/scopes/:scope/resources", controller.GetClientAccessScopeResources)
	manager.GET("/credentials", controller.ListClientAccessCredentials)
	manager.GET("/credentials/:id", controller.GetClientAccessCredential)
	manager.POST("/credentials", middleware.CriticalRateLimit(), controller.CreateClientAccessCredential)
	manager.PUT("/credentials/:id/grants", middleware.CriticalRateLimit(), controller.UpdateClientAccessGrants)
	manager.POST("/credentials/:id/rotate", middleware.CriticalRateLimit(), controller.RotateClientAccessCredential)
	manager.POST("/credentials/:id/disable", middleware.CriticalRateLimit(), controller.DisableClientAccessCredential)
	manager.POST("/credentials/:id/enable", middleware.CriticalRateLimit(), controller.EnableClientAccessCredential)
	manager.POST("/credentials/:id/revoke", middleware.CriticalRateLimit(), controller.RevokeClientAccessCredential)

	reader := api.Group("/client-access/v1")
	reader.Use(middleware.DisableCache(), middleware.ClientAccessAudit(), middleware.ClientAccessAuth(), middleware.ClientAccessRateLimit())
	reader.GET("/codex/usage", middleware.RequireClientScope(clientaccess.ScopeCodexQuotaRead), controller.GetClientCodexUsage)
}
