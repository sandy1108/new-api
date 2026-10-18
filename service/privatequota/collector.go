package privatequota

import (
	"context"
	"errors"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type Collector struct {
	Store *Store
	Owner string
	Fetch func(context.Context, model.PrivateQuotaSubject, string) ([]Observation, int64, error)
}

func (store *Store) now() time.Time {
	if store.Clock != nil {
		return store.Clock()
	}
	return time.Now()
}

// Run每15秒调度，实际频率由各主体due决定；默认关闭时不访问上游。
func (collector *Collector) Run(ctx context.Context) {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for {
		if err := collector.Tick(ctx); err != nil && ctx.Err() == nil {
			// 不打印驱动/上游原始错误，避免OAuth或数据库SQL进入日志。
			common.SysError("private quota collector tick failed; collection remains fail-closed")
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// Tick单实例持有有围栏的租约，串行读取有限批次；网络请求不在数据库事务中。
func (collector *Collector) Tick(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	store := collector.Store
	settings, err := model.GetPrivateQuotaSettings(store.DB.WithContext(ctx))
	if err != nil || !settings.Enabled {
		return err
	}
	lease, err := model.ClaimPrivateQuotaLease(store.DB.WithContext(ctx), collector.Owner, store.now().Unix())
	if errors.Is(err, model.ErrPrivateQuotaLeaseUnavailable) {
		return nil
	}
	if err != nil {
		return err
	}
	work, cancel := context.WithCancel(ctx)
	done := make(chan error, 1)
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-work.Done():
				done <- nil
				return
			case <-ticker.C:
				if _, err := model.RenewPrivateQuotaLease(store.DB.WithContext(work), lease, store.now().Unix()); err != nil {
					cancel()
					done <- err
					return
				}
			}
		}
	}()
	defer func() {
		cancel()
		<-done
		release, releaseCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer releaseCancel()
		_ = model.ReleasePrivateQuotaLease(store.DB.WithContext(release), lease)
	}()
	var subjects []model.PrivateQuotaSubject
	if err := store.DB.WithContext(work).Where("enabled = ? AND next_due_at <= ?", true, store.now().Unix()).Order("next_due_at, id").Limit(20).Find(&subjects).Error; err != nil {
		return err
	}
	fetch := collector.Fetch
	if fetch == nil {
		fetch = store.FetchFresh
	}
	for _, subject := range subjects {
		if err := work.Err(); err != nil {
			return err
		}
		started := store.now().Unix()
		subjectCtx, subjectCancel := context.WithTimeout(work, 60*time.Second)
		rows, retryAfter, fetchErr := fetch(subjectCtx, subject, settings.FingerprintKey)
		subjectCancel()
		if err := work.Err(); err != nil {
			return err
		}
		if fetchErr == nil {
			_, fetchErr = store.SaveObservationBatch(work, lease, ObservationBatch{SubjectID: subject.ID, SubjectVersion: subject.Version,
				SourceChannelID: subject.SourceChannelID, SampleKey: uuid.NewString(), StartedAt: started, FetchedAt: store.now().Unix(), Windows: rows})
		}
		if fetchErr != nil {
			if errors.Is(fetchErr, model.ErrPrivateQuotaConflict) {
				continue
			}
			if errors.Is(fetchErr, model.ErrPrivateQuotaLeaseLost) {
				return fetchErr
			}
			if err := store.recordFailure(work, lease, subject, started, retryAfter, fetchErr); err != nil && !errors.Is(err, model.ErrPrivateQuotaConflict) {
				return err
			}
		}
	}
	return nil
}

// recordFailure只保存稳定错误码，保留最后成功观测；身份异常自动停采但不改官方渠道。
func (store *Store) recordFailure(ctx context.Context, lease model.PrivateQuotaLease, subject model.PrivateQuotaSubject, started, retryAfter int64, cause error) error {
	code, pause := "upstream_unavailable", false
	switch {
	case errors.Is(cause, ErrIdentityChanged):
		code, pause = "identity_changed", true
	case errors.Is(cause, ErrSourceUnavailable):
		code, pause = "source_unavailable", true
	case errors.Is(cause, ErrUpstreamAuth):
		code = "upstream_auth"
	case errors.Is(cause, ErrUpstreamLimited):
		code = "upstream_limited"
	case errors.Is(cause, ErrInvalidResponse):
		code = "invalid_response"
	}
	return store.DB.WithContext(ctx).Session(&gorm.Session{Logger: logger.Default.LogMode(logger.Silent)}).Transaction(func(tx *gorm.DB) error {
		settings, err := model.LockPrivateQuotaSettings(tx)
		if err != nil {
			return err
		}
		now := store.now().Unix()
		if !settings.Enabled || settings.Version != lease.SettingsVersion || settings.LeaseOwner != lease.Owner || settings.LeaseVersion != lease.Fence || settings.LeaseUntil <= now {
			return model.ErrPrivateQuotaLeaseLost
		}
		var latest model.PrivateQuotaSubject
		if err := tx.First(&latest, "id = ?", subject.ID).Error; err != nil {
			return err
		}
		if latest.Version != subject.Version || !latest.Enabled {
			return model.ErrPrivateQuotaConflict
		}
		delay := min(int64(86400), int64(settings.IntervalMinutes)*60*(1<<min(latest.FailureCount+1, 8)))
		// 仅429遵守更长的Retry-After；普通故障仍受24小时上限约束。
		retryLimit := int64(86400)
		if errors.Is(cause, ErrUpstreamLimited) {
			retryLimit = 31536000
		}
		delay = max(delay, min(max(retryAfter, 0), retryLimit))
		updates := map[string]any{"failure_count": latest.FailureCount + 1, "error_code": code, "last_attempt_at": started, "next_due_at": now + delay, "version": latest.Version + 1, "updated_at": now}
		if pause {
			updates["enabled"], updates["next_due_at"] = false, 0
		}
		if err := tx.Model(&model.PrivateQuotaSubject{}).Where("id = ?", subject.ID).Updates(updates).Error; err != nil {
			return err
		}
		if pause {
			return tx.Create(&model.PrivateQuotaEvent{SubjectID: subject.ID, EventKey: "source/" + uuid.NewString(), Kind: "collection_paused", Status: "accepted", Version: 1, Reason: code, CreatedAt: now}).Error
		}
		return nil
	})
}
