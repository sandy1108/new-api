package model

import (
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var (
	ErrPrivateQuotaConflict     = errors.New("private quota version conflict")
	ErrPrivateQuotaStateInvalid = errors.New("private quota identity state requires recovery")
	ErrPrivateQuotaInputInvalid = errors.New("invalid private quota settings")
)

// PrivateQuotaSettings是私有单例配置；身份密钥与租约字段绝不通过JSON输出。
// 时间由调用方提供，禁止GORM在重放/测试中偷偷替换为真实时钟。
type PrivateQuotaSettings struct {
	ID              int64  `json:"-" gorm:"primaryKey;autoIncrement:false"`
	Enabled         bool   `json:"enabled" gorm:"not null"`
	IntervalMinutes int    `json:"interval_minutes" gorm:"not null"`
	Version         int64  `json:"version" gorm:"not null"`
	FingerprintKey  string `json:"-" gorm:"type:varchar(64);not null"`
	KeyVersion      int    `json:"-" gorm:"not null"`
	LeaseOwner      string `json:"-" gorm:"type:varchar(128);not null"`
	LeaseUntil      int64  `json:"-" gorm:"not null"`
	LeaseVersion    int64  `json:"-" gorm:"not null"`
	UpdatedAt       int64  `json:"updated_at" gorm:"autoUpdateTime:false;not null"`
	UpdatedBy       int64  `json:"updated_by" gorm:"not null"`
}

func (PrivateQuotaSettings) TableName() string { return "private_quota_settings" }

// PrivateQuotaSubject仅保存身份指纹与逻辑渠道引用，不关联官方表的迁移或级联删除。
type PrivateQuotaSubject struct {
	ID                  int64  `json:"-" gorm:"primaryKey"`
	Provider            string `json:"-" gorm:"type:varchar(32);not null"`
	IdentityFingerprint string `json:"-" gorm:"type:varchar(64);not null;uniqueIndex:uidx_private_quota_identity"`
	Name                string `json:"-" gorm:"type:varchar(128);not null"`
	SourceChannelID     int64  `json:"-" gorm:"not null"`
	Enabled             bool   `json:"-" gorm:"not null"`
	Version             int64  `json:"-" gorm:"not null"`
	NextDueAt           int64  `json:"-" gorm:"not null;index:idx_private_quota_due"`
	LastAttemptAt       int64  `json:"-" gorm:"not null"`
	LastSuccessAt       int64  `json:"-" gorm:"not null"`
	FailureCount        int    `json:"-" gorm:"not null"`
	ErrorCode           string `json:"-" gorm:"type:varchar(64);not null"`
	RulesVersion        int    `json:"-" gorm:"not null"`
	CreatedAt           int64  `json:"-" gorm:"autoCreateTime:false;not null"`
	UpdatedAt           int64  `json:"-" gorm:"autoUpdateTime:false;not null"`
}

func (PrivateQuotaSubject) TableName() string { return "private_quota_subjects" }

// PrivateQuotaSnapshot是不含OAuth正文的原始观测；NULL百分比保留未知语义。
type PrivateQuotaSnapshot struct {
	ID              int64  `json:"-" gorm:"primaryKey;index:idx_private_quota_observation,priority:4"`
	SubjectID       int64  `json:"-" gorm:"not null;uniqueIndex:uidx_private_quota_sample,priority:1;index:idx_private_quota_observation,priority:1"`
	SampleKey       string `json:"-" gorm:"type:varchar(64);not null;uniqueIndex:uidx_private_quota_sample,priority:2"`
	WindowSeconds   int64  `json:"-" gorm:"not null;uniqueIndex:uidx_private_quota_sample,priority:3;index:idx_private_quota_observation,priority:2"`
	SegmentID       string `json:"-" gorm:"type:varchar(64);not null"`
	FetchedAt       int64  `json:"-" gorm:"not null;index:idx_private_quota_observation,priority:3"`
	StartedAt       int64  `json:"-" gorm:"not null"`
	RecordedAt      int64  `json:"-" gorm:"not null"`
	UsedBasisPoints *int   `json:"-"`
	ResetAt         int64  `json:"-" gorm:"not null"`
	IntervalSeconds int64  `json:"-" gorm:"not null"`
	QualityCode     string `json:"-" gorm:"type:varchar(64);not null"`
	SourceChannelID int64  `json:"-" gorm:"not null"`
}

func (PrivateQuotaSnapshot) TableName() string { return "private_quota_snapshots" }

// PrivateQuotaEvent兼容观测边界与配置审计；全局配置审计的SubjectID/WindowSeconds为0。
// 人工替代通过新行保留历史，后续服务负责事务校验唯一有效链。
type PrivateQuotaEvent struct {
	ID                int64  `json:"-" gorm:"primaryKey"`
	SubjectID         int64  `json:"-" gorm:"not null;index:idx_private_quota_event_subject,priority:1"`
	WindowSeconds     int64  `json:"-" gorm:"not null"`
	SegmentID         string `json:"-" gorm:"type:varchar(64);not null"`
	EventKey          string `json:"-" gorm:"type:varchar(128);not null;uniqueIndex:uidx_private_quota_event"`
	Kind              string `json:"-" gorm:"type:varchar(32);not null"`
	Status            string `json:"-" gorm:"type:varchar(32);not null"`
	BeforeSnapshotID  int64  `json:"-" gorm:"not null"`
	AfterSnapshotID   int64  `json:"-" gorm:"not null"`
	ObservedFrom      int64  `json:"-" gorm:"not null"`
	ObservedTo        int64  `json:"-" gorm:"not null"`
	OldResetAt        int64  `json:"-" gorm:"not null"`
	NewResetAt        int64  `json:"-" gorm:"not null"`
	RulesVersion      int    `json:"-" gorm:"not null"`
	Version           int64  `json:"-" gorm:"not null"`
	Decision          string `json:"-" gorm:"type:varchar(32);not null"`
	ActorID           int64  `json:"-" gorm:"not null"`
	Reason            string `json:"-" gorm:"type:text;not null"`
	SupersedesEventID int64  `json:"-" gorm:"not null"`
	CreatedAt         int64  `json:"-" gorm:"autoCreateTime:false;not null;index:idx_private_quota_event_subject,priority:2"`
}

func (PrivateQuotaEvent) TableName() string { return "private_quota_events" }

// PrivateQuotaSettingsView使用独立白名单；不能把持久化对象直接用作后台响应。
type PrivateQuotaSettingsView struct {
	Enabled         bool  `json:"enabled"`
	IntervalMinutes int   `json:"interval_minutes"`
	Version         int64 `json:"version"`
	UpdatedAt       int64 `json:"updated_at"`
}

type PrivateQuotaSettingsChange struct {
	Enabled         bool
	IntervalMinutes int
	Version         int64
	ActorID         int64
	Now             int64
}

func GetPrivateQuotaSettings(db *gorm.DB) (PrivateQuotaSettings, error) {
	var settings PrivateQuotaSettings
	if err := db.Where("id = ?", 1).Take(&settings).Error; err != nil {
		return PrivateQuotaSettings{}, err
	}
	key, err := hex.DecodeString(settings.FingerprintKey)
	if err != nil || len(key) != 32 || settings.KeyVersion != 1 || settings.Version < 1 ||
		settings.IntervalMinutes < 2 || settings.IntervalMinutes > 1440 {
		return PrivateQuotaSettings{}, ErrPrivateQuotaStateInvalid
	}
	return settings, nil
}

// UpdatePrivateQuotaSettings用写入优先的CAS串行化配置修改；审计失败则整笔回滚。
// 只更新白名单字段，不允许配置提交修改身份密钥或抢占采集租约。
func UpdatePrivateQuotaSettings(db *gorm.DB, change PrivateQuotaSettingsChange) (PrivateQuotaSettingsView, error) {
	if change.IntervalMinutes < 2 || change.IntervalMinutes > 1440 || change.Version < 1 || change.ActorID < 1 || change.Now < 1 {
		return PrivateQuotaSettingsView{}, ErrPrivateQuotaInputInvalid
	}
	var view PrivateQuotaSettingsView
	err := db.Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&PrivateQuotaSettings{}).Where("id = ? AND version = ?", 1, change.Version).
			Updates(map[string]any{"enabled": change.Enabled, "interval_minutes": change.IntervalMinutes,
				"version": gorm.Expr("version + ?", 1), "updated_at": change.Now, "updated_by": change.ActorID})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ErrPrivateQuotaConflict
		}
		settings, err := GetPrivateQuotaSettings(tx)
		if err != nil {
			return err
		}
		// 配置变更使旧租约失效，重排后续采集；历史快照保留原采样间隔。
		if err := tx.Model(&PrivateQuotaSubject{}).Where("enabled = ?", true).
			Updates(map[string]any{"next_due_at": change.Now}).Error; err != nil {
			return err
		}
		audit := PrivateQuotaEvent{EventKey: "settings/" + uuid.NewString(), Kind: "settings_updated", Status: "accepted",
			Version: 1, ActorID: change.ActorID, CreatedAt: change.Now,
			Reason: fmt.Sprintf("enabled=%t interval_minutes=%d version=%d", settings.Enabled, settings.IntervalMinutes, settings.Version)}
		if err := tx.Create(&audit).Error; err != nil {
			return err
		}
		view = PrivateQuotaSettingsView{Enabled: settings.Enabled, IntervalMinutes: settings.IntervalMinutes,
			Version: settings.Version, UpdatedAt: settings.UpdatedAt}
		return nil
	})
	if err != nil {
		return PrivateQuotaSettingsView{}, err
	}
	return view, nil
}

// 插入单例时可能发生正常的唯一键竞争；禁止GORM错误日志打印包含指纹密钥的完整SQL。
func privateQuotaSensitiveDB(db *gorm.DB) *gorm.DB {
	return db.Session(&gorm.Session{Logger: logger.Default.LogMode(logger.Silent)})
}
