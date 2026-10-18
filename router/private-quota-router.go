package router

import (
	"github.com/QuantumNous/new-api/controller"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/gin-gonic/gin"
)

// 私有模块只注册一个入口，不修改官方渠道、令牌或Scope协议。
func registerPrivateQuotaRoutes(api *gin.RouterGroup) {
	group := api.Group("/private-quota", middleware.RootAuth(), middleware.PrivateQuotaSessionOnly(), middleware.DisableCache())
	group.GET("/settings", controller.GetPrivateQuotaSettings)
	group.GET("/channels", controller.ListPrivateQuotaChannels)
	group.GET("/subjects", controller.ListPrivateQuotaSubjects)
	group.GET("/summary", controller.GetPrivateQuotaSummary)
	group.GET("/cycles", controller.GetPrivateQuotaCycles)
	group.GET("/trend", controller.GetPrivateQuotaTrend)
	group.GET("/events", controller.GetPrivateQuotaEvents)
	group.PUT("/settings", middleware.PrivateQuotaWriteGuard(), controller.UpdatePrivateQuotaSettings)
	group.POST("/subjects", middleware.PrivateQuotaWriteGuard(), controller.CreatePrivateQuotaSubject)
	group.PUT("/subjects/:id", middleware.PrivateQuotaWriteGuard(), controller.UpdatePrivateQuotaSubject)
	group.POST("/events/:id/decision", middleware.PrivateQuotaWriteGuard(), controller.DecidePrivateQuotaEvent)
}
