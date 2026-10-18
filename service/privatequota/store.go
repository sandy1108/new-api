package privatequota

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var ErrSubjectExists = errors.New("private quota subject already exists")

// Store只操作显式传入的主数据库，不读取全局DB或生产连接配置。
type Store struct {
	DB    *gorm.DB
	Clock func() time.Time
}

type SubjectChange struct {
	Name            string
	SourceChannelID int64
	Enabled         bool
	Version         int64
	ActorID         int64
	Now             int64
}

// CreateSubject在短事务内解析来源并去重；重复创建返回已有主体及ErrSubjectExists，绝不隐式换源。
// ActorID与Now须由可信服务端提供，不能直接绑定客户端提交的完整SubjectChange。
func (store *Store) CreateSubject(ctx context.Context, change SubjectChange) (model.PrivateQuotaSubject, error) {
	change, err := normalizeSubjectChange(change)
	if err != nil {
		return model.PrivateQuotaSubject{}, err
	}
	var subject model.PrivateQuotaSubject
	err = store.DB.WithContext(ctx).Session(&gorm.Session{Logger: logger.Default.LogMode(logger.Silent)}).Transaction(func(tx *gorm.DB) error {
		settings, err := model.LockPrivateQuotaSettings(tx)
		if err != nil {
			return err
		}
		var channel model.Channel
		if err := tx.First(&channel, "id = ?", change.SourceChannelID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrSourceUnavailable
			}
			return err
		}
		fingerprint, err := ChannelFingerprint(&channel, settings.FingerprintKey)
		if err != nil {
			return err
		}
		err = tx.Where("identity_fingerprint = ?", fingerprint).Take(&subject).Error
		if err == nil {
			return ErrSubjectExists
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		subject = model.PrivateQuotaSubject{Provider: "codex", IdentityFingerprint: fingerprint,
			Name: change.Name, SourceChannelID: change.SourceChannelID, Enabled: change.Enabled,
			Version: 1, RulesVersion: RulesVersion, CreatedAt: change.Now, UpdatedAt: change.Now}
		if change.Enabled {
			subject.NextDueAt = change.Now
		}
		if err := tx.Create(&subject).Error; err != nil {
			return err
		}
		return recordSubjectChange(tx, subject, "subject_created", change)
	})
	if err != nil && !errors.Is(err, ErrSubjectExists) {
		return model.PrivateQuotaSubject{}, err
	}
	return subject, err
}

// UpdateSubject只允许同身份换源，使用版本比较防止覆盖旧预览；审计失败则主体变更整体回滚。
func (store *Store) UpdateSubject(ctx context.Context, id int64, change SubjectChange) (model.PrivateQuotaSubject, error) {
	change, err := normalizeSubjectChange(change)
	if err != nil {
		return model.PrivateQuotaSubject{}, err
	}
	if id < 1 || change.Version < 1 {
		return model.PrivateQuotaSubject{}, model.ErrPrivateQuotaInputInvalid
	}
	var subject model.PrivateQuotaSubject
	err = store.DB.WithContext(ctx).Session(&gorm.Session{Logger: logger.Default.LogMode(logger.Silent)}).Transaction(func(tx *gorm.DB) error {
		settings, err := model.LockPrivateQuotaSettings(tx)
		if err != nil {
			return err
		}
		if err := tx.First(&subject, "id = ?", id).Error; err != nil {
			return err
		}
		if subject.Version != change.Version {
			return model.ErrPrivateQuotaConflict
		}
		// 绑定/启用必须校验来源；但原来源损坏或被删不能阻止用户暂停该主体。
		if change.Enabled || subject.SourceChannelID != change.SourceChannelID {
			var channel model.Channel
			if err := tx.First(&channel, "id = ?", change.SourceChannelID).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return ErrSourceUnavailable
				}
				return err
			}
			if err := ValidateSubjectSource(subject, &channel, settings.FingerprintKey); err != nil {
				return err
			}
		}
		nextDueAt := subject.NextDueAt
		if !change.Enabled {
			nextDueAt = 0
		} else if !subject.Enabled || subject.SourceChannelID != change.SourceChannelID {
			nextDueAt = change.Now
		}
		result := tx.Model(&model.PrivateQuotaSubject{}).Where("id = ? AND version = ?", id, change.Version).
			Updates(map[string]any{"name": change.Name, "source_channel_id": change.SourceChannelID,
				"enabled": change.Enabled, "version": gorm.Expr("version + ?", 1),
				"next_due_at": nextDueAt, "updated_at": change.Now})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return model.ErrPrivateQuotaConflict
		}
		if err := tx.First(&subject, "id = ?", id).Error; err != nil {
			return err
		}
		return recordSubjectChange(tx, subject, "subject_updated", change)
	})
	if err != nil {
		return model.PrivateQuotaSubject{}, err
	}
	return subject, nil
}

func normalizeSubjectChange(change SubjectChange) (SubjectChange, error) {
	change.Name = strings.TrimSpace(change.Name)
	if !utf8.ValidString(change.Name) || change.Name == "" || utf8.RuneCountInString(change.Name) > 128 ||
		strings.ContainsFunc(change.Name, unicode.IsControl) || change.SourceChannelID < 1 || change.ActorID < 1 || change.Now < 1 {
		return SubjectChange{}, model.ErrPrivateQuotaInputInvalid
	}
	return change, nil
}

func recordSubjectChange(tx *gorm.DB, subject model.PrivateQuotaSubject, kind string, change SubjectChange) error {
	return tx.Create(&model.PrivateQuotaEvent{SubjectID: subject.ID, EventKey: "subject/" + uuid.NewString(),
		Kind: kind, Status: "accepted", Version: 1, ActorID: change.ActorID, CreatedAt: change.Now,
		Reason: fmt.Sprintf("source_channel_id=%d enabled=%t subject_version=%d", subject.SourceChannelID, subject.Enabled, subject.Version)}).Error
}
