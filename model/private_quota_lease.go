package model

import (
	"errors"
	"math"
	"strings"
	"unicode"
	"unicode/utf8"

	"gorm.io/gorm"
)

var (
	ErrPrivateQuotaLeaseUnavailable = errors.New("private quota lease unavailable")
	ErrPrivateQuotaLeaseLost        = errors.New("private quota lease lost")
)

// PrivateQuotaLease是内部采集许可；Fence区分同名进程的历次接管，不得作为API凭据返回。
type PrivateQuotaLease struct {
	Owner           string `json:"-"`
	Fence           int64  `json:"-"`
	SettingsVersion int64  `json:"-"`
	Until           int64  `json:"-"`
}

// LockPrivateQuotaSettings须在短事务内使用，以无值变更UPDATE先取得单例写锁。
// 三库采用同一写入顺序，避免SQLite读事务升级写锁；禁止在持锁期间请求上游。
func LockPrivateQuotaSettings(tx *gorm.DB) (PrivateQuotaSettings, error) {
	if err := tx.Model(&PrivateQuotaSettings{}).Where("id = ?", 1).
		UpdateColumn("version", gorm.Expr("version")).Error; err != nil {
		return PrivateQuotaSettings{}, err
	}
	return GetPrivateQuotaSettings(tx)
}

// ClaimPrivateQuotaLease只允许启用状态下接管空闲/到期租约，每次接管递增Fence。
func ClaimPrivateQuotaLease(db *gorm.DB, owner string, now int64) (PrivateQuotaLease, error) {
	if !validPrivateQuotaOwner(owner) || now < 1 || now > math.MaxInt64-120 {
		return PrivateQuotaLease{}, ErrPrivateQuotaInputInvalid
	}
	var lease PrivateQuotaLease
	err := privateQuotaSensitiveDB(db).Transaction(func(tx *gorm.DB) error {
		settings, err := LockPrivateQuotaSettings(tx)
		if err != nil {
			return err
		}
		if !settings.Enabled || settings.LeaseUntil > now {
			return ErrPrivateQuotaLeaseUnavailable
		}
		if settings.LeaseVersion < 0 || settings.LeaseVersion == math.MaxInt64 {
			return ErrPrivateQuotaStateInvalid
		}
		lease = PrivateQuotaLease{Owner: owner, Fence: settings.LeaseVersion + 1, SettingsVersion: settings.Version, Until: now + 120}
		return tx.Model(&PrivateQuotaSettings{}).Where("id = ?", 1).Updates(map[string]any{
			"lease_owner": lease.Owner, "lease_version": lease.Fence, "lease_until": lease.Until,
		}).Error
	})
	if err != nil {
		return PrivateQuotaLease{}, err
	}
	return lease, nil
}

// RenewPrivateQuotaLease不能复活已到期租约，也不能在配置变更后继续续租。
// 持锁校验后只检查SQL错误，不把MySQL同值更新的零行数误认为失租。
func RenewPrivateQuotaLease(db *gorm.DB, lease PrivateQuotaLease, now int64) (PrivateQuotaLease, error) {
	if !validPrivateQuotaOwner(lease.Owner) || lease.Fence < 1 || lease.SettingsVersion < 1 || now < 1 || now > math.MaxInt64-120 {
		return PrivateQuotaLease{}, ErrPrivateQuotaInputInvalid
	}
	err := privateQuotaSensitiveDB(db).Transaction(func(tx *gorm.DB) error {
		settings, err := LockPrivateQuotaSettings(tx)
		if err != nil {
			return err
		}
		if !settings.Enabled || settings.Version != lease.SettingsVersion || settings.LeaseOwner != lease.Owner ||
			settings.LeaseVersion != lease.Fence || settings.LeaseUntil <= now {
			return ErrPrivateQuotaLeaseLost
		}
		lease.Until = max(settings.LeaseUntil, now+120)
		return tx.Model(&PrivateQuotaSettings{}).Where("id = ?", 1).UpdateColumn("lease_until", lease.Until).Error
	})
	if err != nil {
		return PrivateQuotaLease{}, err
	}
	return lease, nil
}

// ReleasePrivateQuotaLease只释放自身Fence，不要求配置仍启用，绝不清掉接管者的租约。
func ReleasePrivateQuotaLease(db *gorm.DB, lease PrivateQuotaLease) error {
	if !validPrivateQuotaOwner(lease.Owner) || lease.Fence < 1 {
		return ErrPrivateQuotaInputInvalid
	}
	result := privateQuotaSensitiveDB(db).Model(&PrivateQuotaSettings{}).
		Where("id = ? AND lease_owner = ? AND lease_version = ?", 1, lease.Owner, lease.Fence).
		Updates(map[string]any{"lease_owner": "", "lease_until": 0})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return ErrPrivateQuotaLeaseLost
	}
	return nil
}

func validPrivateQuotaOwner(owner string) bool {
	return owner != "" && len(owner) <= 128 && utf8.ValidString(owner) && owner == strings.TrimSpace(owner) && !strings.ContainsFunc(owner, unicode.IsControl)
}
