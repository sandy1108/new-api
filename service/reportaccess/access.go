// Package reportaccess只拥有私有报表能力，不扩展管理员、PAT或客户端凭据协议。
package reportaccess

import (
	"context"
	"errors"
	"fmt"
	"math"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/tidwall/gjson"
	"gorm.io/gorm"
)

const UsageReadAll = "pong.usage_summary.read_all"
const SubscriptionReadAll = "pong.subscription_quota.read_all"

// ManageGrants是Root固有检查点，不是可分配的普通用户授权或PAT Scope。
const ManageGrants = "pong.report_grants.manage"

type Store struct{ DB *gorm.DB }

type Capabilities struct {
	UsageReadAll        bool   `json:"usage_read_all"`
	SubscriptionReadAll bool   `json:"subscription_read_all"`
	CanManageGrants     bool   `json:"can_manage_grants"`
	Revision            string `json:"revision"`
}

// Capabilities每次从主库核验身份及授权，不使用会话声明或正向授权缓存。
func (s Store) Capabilities(ctx context.Context, identity model.AuthSessionIdentity) (Capabilities, error) {
	var caps Capabilities
	if _, pat := model.ParseAccessTokenSessionID(identity.SessionID); pat {
		return caps, model.ErrPrivateReportForbidden
	}
	if identity.UserID <= 0 || identity.SessionID == "" || identity.SessionVersion < 1 || identity.UserAuthVersion < 1 {
		return caps, model.ErrPrivateReportForbidden
	}
	db := s.DB.WithContext(ctx)
	var user model.User
	if err := db.Select("id", "role", "status", "auth_version").First(&user, identity.UserID).Error; err != nil {
		return caps, err
	}
	if user.Status != common.UserStatusEnabled || user.AuthVersion != identity.UserAuthVersion {
		return caps, model.ErrPrivateReportForbidden
	}
	caps.UsageReadAll = user.Role >= common.RoleAdminUser
	caps.SubscriptionReadAll, caps.CanManageGrants = user.Role == common.RoleRootUser, user.Role == common.RoleRootUser
	var grant model.PrivateReportGrant
	if user.Role == common.RoleCommonUser {
		err := db.Where("user_id = ?", user.Id).Take(&grant).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return caps, err
		}
		if grant.Version > 0 && grant.SubjectAuthVersion == user.AuthVersion {
			caps.UsageReadAll, caps.SubscriptionReadAll = grant.UsageReadAll, grant.SubscriptionReadAll
		}
	}
	caps.Revision = fmt.Sprintf("%d:%d:%d:%t:%t", user.Role, user.AuthVersion, grant.Version, caps.UsageReadAll, caps.SubscriptionReadAll)
	return caps, nil
}

type Change struct {
	UserID int `json:"user_id"`
	model.PrivateReportGrantChange
}

// ParseChange严格解析固定字段；先由common校验JSON，再检测重复/未知键，防止歧义覆盖。
func ParseChange(raw []byte, withTarget bool) (Change, error) {
	var fields map[string]common.RawMessage
	var change Change
	if common.GetJsonType(raw) != "object" || common.Unmarshal(raw, &fields) != nil {
		return change, model.ErrPrivateReportTarget
	}
	expected := 3
	if withTarget {
		expected++
	}
	count := 0
	gjson.ParseBytes(raw).ForEach(func(_, _ gjson.Result) bool { count++; return true })
	if len(fields) != expected || count != expected {
		return change, model.ErrPrivateReportTarget
	}
	for key := range fields {
		if key != "usage_read_all" && key != "subscription_read_all" && key != "version" && !(withTarget && key == "user_id") {
			return change, model.ErrPrivateReportTarget
		}
	}
	if string(fields["usage_read_all"]) != "true" && string(fields["usage_read_all"]) != "false" {
		return change, model.ErrPrivateReportTarget
	}
	if string(fields["subscription_read_all"]) != "true" && string(fields["subscription_read_all"]) != "false" {
		return change, model.ErrPrivateReportTarget
	}
	if common.Unmarshal(raw, &change) != nil || change.Version < 0 || change.Version == math.MaxInt64 || (withTarget && change.UserID <= 0) {
		return Change{}, model.ErrPrivateReportTarget
	}
	return change, nil
}
