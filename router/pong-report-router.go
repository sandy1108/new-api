package router

import (
	"github.com/QuantumNous/new-api/controller"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/service/reportaccess"
	"github.com/gin-gonic/gin"
)

// registerPongReportRoutes精确列出报表能力，不降低原管理员路由组的权限。
func registerPongReportRoutes(api *gin.RouterGroup) {
	read := api.Group("/pong-reports", middleware.DisableCache(), middleware.UserAuth(), middleware.PongReportSessionOnly())
	read.GET("/capabilities", controller.GetPongReportCapabilities)
	usage := read.Group("", middleware.RequirePongReportCapability(reportaccess.UsageReadAll), middleware.PongReportReadLimit("usage"))
	usage.GET("/usage-summary", controller.PongReportQuery("start_timestamp", "end_timestamp", "include_trend", "scope"), controller.GetPongReportUsage)
	ledger := read.Group("/subscription", middleware.RequirePongReportCapability(reportaccess.SubscriptionReadAll), middleware.PongReportReadLimit("ledger"))
	ledger.GET("/subjects", controller.PongReportQuery("page", "page_size"), controller.ListPrivateQuotaSubjects)
	ledger.GET("/summary", controller.PongReportQuery("subject_id", "month", "window_seconds"), controller.GetPrivateQuotaSummary)
	ledger.GET("/cycles", controller.PongReportQuery("subject_id", "month", "window_seconds", "page", "page_size"), controller.GetPrivateQuotaCycles)
	ledger.GET("/trend", controller.PongReportQuery("subject_id", "month", "window_seconds"), controller.GetPrivateQuotaTrend)
	ledger.GET("/events", controller.PongReportQuery("subject_id", "month", "window_seconds", "status", "page", "page_size"), controller.GetPrivateQuotaEvents)
	manage := api.Group("/pong-report-access", middleware.DisableCache(), middleware.RootAuth(), middleware.PongReportSessionOnly(), middleware.RequirePongReportCapability(reportaccess.ManageGrants))
	manage.GET("/users", controller.ListPongReportUsers)
	manage.GET("/users/:id", controller.GetPongReportUser)
	manage.PUT("/users/:id", middleware.PongReportWriteGuard(), middleware.UserCriticalRateLimit("pong-report-grant"), controller.UpdatePongReportUser)
}
