package controller

import (
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/service/reportaccess"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// pongReportError只输出固定错误，不把数据库、OAuth或驱动信息带给观察用户。
func pongReportError(c *gin.Context, status int) {
	c.JSON(status, gin.H{"success": false, "code": "PONG_REPORT_" + strconv.Itoa(status), "message": http.StatusText(status)})
}

func GetPongReportCapabilities(c *gin.Context) {
	identity, ok := middleware.GetSessionAuthIdentity(c)
	if !ok {
		pongReportError(c, http.StatusForbidden)
		return
	}
	caps, err := (reportaccess.Store{DB: model.DB}).Capabilities(c.Request.Context(), identity)
	if err != nil {
		status := http.StatusServiceUnavailable
		if errors.Is(err, model.ErrPrivateReportForbidden) || errors.Is(err, gorm.ErrRecordNotFound) {
			status = http.StatusForbidden
		}
		pongReportError(c, status)
		return
	}
	common.ApiSuccess(c, caps)
}

type pongReportUserView struct {
	ID                  int    `json:"id"`
	Username            string `json:"username"`
	DisplayName         string `json:"display_name"`
	UsageReadAll        bool   `json:"usage_read_all"`
	SubscriptionReadAll bool   `json:"subscription_read_all"`
	Version             int64  `json:"version"`
	Invalidated         bool   `json:"invalidated"`
	UpdatedAt           int64  `json:"updated_at"`
}

// ListPongReportUsers独立投影普通账户及授权，不序列化官方完整用户对象。
func ListPongReportUsers(c *gin.Context) {
	page, size, ok := privateQuotaPage(c)
	if !ok {
		return
	}
	search := strings.TrimSpace(c.Query("search"))
	if len(search) > 80 {
		pongReportError(c, 400)
		return
	}
	db := model.DB.WithContext(c.Request.Context())
	query := db.Model(&model.User{}).Where("role = ? AND status = ?", common.RoleCommonUser, common.UserStatusEnabled)
	if search != "" {
		// 不开放SQL通配查询；精确用户名或显示名，另支持数字ID。
		id, _ := strconv.Atoi(search)
		query = query.Where("username = ? OR display_name = ? OR id = ?", search, search, id)
	}
	var count int64
	if err := query.Count(&count).Error; err != nil {
		pongReportError(c, 503)
		return
	}
	var users []model.User
	if err := query.Select("id", "username", "display_name", "auth_version").Order("id").Offset((page - 1) * size).Limit(size).Find(&users).Error; err != nil {
		pongReportError(c, 503)
		return
	}
	items := make([]pongReportUserView, 0, len(users))
	for _, user := range users {
		view, err := readPongReportUser(db, user)
		if err != nil {
			pongReportError(c, 503)
			return
		}
		items = append(items, view)
	}
	common.ApiSuccess(c, gin.H{"items": items, "total": count})
}

func readPongReportUser(db *gorm.DB, user model.User) (pongReportUserView, error) {
	view := pongReportUserView{ID: user.Id, Username: user.Username, DisplayName: user.DisplayName}
	var grant model.PrivateReportGrant
	err := db.Where("user_id = ?", user.Id).Take(&grant).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return view, nil
	}
	if err != nil {
		return view, err
	}
	view.UsageReadAll, view.SubscriptionReadAll = grant.UsageReadAll, grant.SubscriptionReadAll
	view.Version, view.UpdatedAt, view.Invalidated = grant.Version, grant.UpdatedAt, grant.SubjectAuthVersion != user.AuthVersion
	return view, nil
}

func GetPongReportUser(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		pongReportError(c, 400)
		return
	}
	var user model.User
	db := model.DB.WithContext(c.Request.Context())
	if err := db.Select("id", "username", "display_name", "auth_version").Where("id = ? AND role = ? AND status = ?", id, common.RoleCommonUser, common.UserStatusEnabled).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			pongReportError(c, 404)
		} else {
			pongReportError(c, 503)
		}
		return
	}
	view, err := readPongReportUser(db, user)
	if err != nil {
		pongReportError(c, 503)
		return
	}
	common.ApiSuccess(c, view)
}

// UpdatePongReportUser的再次验证绑定目标/勾选/版本，保存时重新验证Root会话。
func UpdatePongReportUser(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		pongReportError(c, 400)
		return
	}
	raw, err := io.ReadAll(c.Request.Body)
	if err != nil {
		pongReportError(c, 400)
		return
	}
	change, err := reportaccess.ParseChange(raw, false)
	if err != nil {
		pongReportError(c, 400)
		return
	}
	change.UserID = id
	payload, err := common.Marshal(change)
	if err != nil {
		pongReportError(c, 500)
		return
	}
	if middleware.RequireSecurityProof(c, service.VerificationOperation{Scope: service.VerificationScopePongReportGrants, Context: payload}) == nil {
		return
	}
	identity, ok := middleware.GetSessionAuthIdentity(c)
	if !ok {
		pongReportError(c, 403)
		return
	}
	grant, err := model.UpdatePrivateReportGrant(model.DB.WithContext(c.Request.Context()), identity, id, change.PrivateReportGrantChange)
	if err != nil {
		status := http.StatusServiceUnavailable
		if errors.Is(err, model.ErrPrivateReportConflict) {
			status = 409
		}
		if errors.Is(err, model.ErrPrivateReportTarget) {
			status = 400
		}
		if errors.Is(err, model.ErrPrivateReportForbidden) || errors.Is(err, model.ErrUserSessionInactive) {
			status = 403
		}
		pongReportError(c, status)
		return
	}
	common.ApiSuccess(c, gin.H{"version": grant.Version})
}
