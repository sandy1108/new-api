package model

import (
	"errors"
	"math"
	"time"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrPrivateReportConflict  = errors.New("report grant version conflict")
	ErrPrivateReportTarget    = errors.New("report grant target unavailable")
	ErrPrivateReportForbidden = errors.New("report access forbidden")
)

// PrivateReportGrant只存两项报表能力，不参与官方角色或Casbin权限计算。
type PrivateReportGrant struct {
	ID                  int64 `json:"-" gorm:"primaryKey"`
	UserID              int   `json:"user_id" gorm:"not null;uniqueIndex:uidx_private_report_grant_user"`
	UsageReadAll        bool  `json:"usage_read_all" gorm:"not null;default:false"`
	SubscriptionReadAll bool  `json:"subscription_read_all" gorm:"not null;default:false"`
	SubjectAuthVersion  int64 `json:"-" gorm:"not null"`
	Version             int64 `json:"version" gorm:"not null"`
	UpdatedBy           int   `json:"-" gorm:"not null"`
	UpdatedAt           int64 `json:"updated_at" gorm:"autoUpdateTime:false;not null"`
}

func (PrivateReportGrant) TableName() string { return "private_report_grants" }

// PrivateReportGrantEvent与授权同事务追加，审计失败时禁止授权部分提交。
type PrivateReportGrantEvent struct {
	ID                 int64 `json:"-" gorm:"primaryKey"`
	UserID             int   `json:"-" gorm:"not null;index:idx_private_report_event_user"`
	ActorID            int   `json:"-" gorm:"not null"`
	BeforeUsage        bool  `json:"-" gorm:"not null"`
	BeforeSubscription bool  `json:"-" gorm:"not null"`
	AfterUsage         bool  `json:"-" gorm:"not null"`
	AfterSubscription  bool  `json:"-" gorm:"not null"`
	BeforeAuthVersion  int64 `json:"-" gorm:"not null"`
	AfterAuthVersion   int64 `json:"-" gorm:"not null"`
	Version            int64 `json:"-" gorm:"not null"`
	CreatedAt          int64 `json:"-" gorm:"autoCreateTime:false;not null"`
}

func (PrivateReportGrantEvent) TableName() string { return "private_report_grant_events" }

func MigratePrivateReportAccessTables(db *gorm.DB) error {
	return db.AutoMigrate(&PrivateReportGrant{}, &PrivateReportGrantEvent{})
}

type PrivateReportGrantChange struct {
	UsageReadAll        bool  `json:"usage_read_all"`
	SubscriptionReadAll bool  `json:"subscription_read_all"`
	Version             int64 `json:"version"`
}

// UpdatePrivateReportGrant重新核验权威Root会话及目标身份；撤权保留版本墓碑。
func UpdatePrivateReportGrant(db *gorm.DB, actor AuthSessionIdentity, target int, change PrivateReportGrantChange) (PrivateReportGrant, error) {
	if target <= 0 || change.Version < 0 || change.Version == math.MaxInt64 {
		return PrivateReportGrant{}, ErrPrivateReportTarget
	}
	if _, pat := ParseAccessTokenSessionID(actor.SessionID); pat {
		return PrivateReportGrant{}, ErrPrivateReportForbidden
	}
	var saved PrivateReportGrant
	err := db.Transaction(func(tx *gorm.DB) error {
		// 先预留私有授权行：避免MySQL对不存在行UPDATE产生可共享间隙锁，
		// 并在SQLite首先取得写锁。版本0只存在于当前事务，失败时连同行一并回滚。
		reservation := PrivateReportGrant{UserID: target}
		if err := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "user_id"}}, DoNothing: true}).Create(&reservation).Error; err != nil {
			return err
		}
		if err := ValidateAuthSessionWithTx(tx, actor); err != nil {
			return err
		}
		var operator User
		if err := tx.Select("id", "role").First(&operator, actor.UserID).Error; err != nil {
			return err
		}
		if operator.Role != common.RoleRootUser {
			return ErrPrivateReportForbidden
		}
		var user User
		if err := lockForUpdate(tx).Select("id", "role", "status", "auth_version").First(&user, target).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrPrivateReportTarget
			}
			return err
		}
		if user.Role != common.RoleCommonUser || user.Status != common.UserStatusEnabled || user.AuthVersion < 1 {
			return ErrPrivateReportTarget
		}
		var previous PrivateReportGrant
		err := lockForUpdate(tx).Where("user_id = ?", target).Take(&previous).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if previous.Version != change.Version {
			return ErrPrivateReportConflict
		}
		if previous.ID != 0 && previous.UsageReadAll == change.UsageReadAll && previous.SubscriptionReadAll == change.SubscriptionReadAll && previous.SubjectAuthVersion == user.AuthVersion {
			saved = previous
			return nil
		}
		saved = PrivateReportGrant{ID: previous.ID, UserID: target, UsageReadAll: change.UsageReadAll, SubscriptionReadAll: change.SubscriptionReadAll, SubjectAuthVersion: user.AuthVersion, Version: change.Version + 1, UpdatedBy: actor.UserID, UpdatedAt: time.Now().Unix()}
		result := tx.Model(&PrivateReportGrant{}).Where("id = ? AND version = ?", previous.ID, change.Version).Updates(map[string]any{
			"usage_read_all": saved.UsageReadAll, "subscription_read_all": saved.SubscriptionReadAll, "subject_auth_version": saved.SubjectAuthVersion,
			"version": saved.Version, "updated_by": saved.UpdatedBy, "updated_at": saved.UpdatedAt,
		})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ErrPrivateReportConflict
		}
		return tx.Create(&PrivateReportGrantEvent{UserID: target, ActorID: actor.UserID, BeforeUsage: previous.UsageReadAll, BeforeSubscription: previous.SubscriptionReadAll,
			AfterUsage: saved.UsageReadAll, AfterSubscription: saved.SubscriptionReadAll, BeforeAuthVersion: previous.SubjectAuthVersion, AfterAuthVersion: saved.SubjectAuthVersion, Version: saved.Version, CreatedAt: saved.UpdatedAt}).Error
	})
	return saved, err
}
