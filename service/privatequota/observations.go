package privatequota

import (
	"context"
	"errors"
	"math"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type Observation struct {
	WindowSeconds   int64
	UsedBasisPoints *int
	ResetAt         int64
}

// ObservationBatch只接收一次新鲜读取的观测；不接受OAuth正文或调用方指定的分段。
type ObservationBatch struct {
	SubjectID       int64
	SubjectVersion  int64
	SourceChannelID int64
	SampleKey       string
	StartedAt       int64
	FetchedAt       int64
	Windows         []Observation
}

type ObservationResult struct {
	Revision  int64
	Duplicate bool
}

// SaveObservationBatch在短事务中保存整批原始证据和采集进度；任一窗口失败则全部回滚。
// 同事务重放边界和已有决定；网络与OAuth刷新必须由事务外的采集器完成。
func (store *Store) SaveObservationBatch(ctx context.Context, lease model.PrivateQuotaLease, batch ObservationBatch) (ObservationResult, error) {
	if batch.SubjectID < 1 || batch.SubjectVersion < 1 || batch.SourceChannelID < 1 ||
		batch.SampleKey == "" || len(batch.SampleKey) > 64 || !utf8.ValidString(batch.SampleKey) ||
		batch.SampleKey != strings.TrimSpace(batch.SampleKey) || strings.ContainsFunc(batch.SampleKey, unicode.IsControl) ||
		batch.StartedAt < 1 || batch.StartedAt > math.MaxInt64-86400 || batch.FetchedAt < batch.StartedAt ||
		len(batch.Windows) < 1 || len(batch.Windows) > 2 {
		return ObservationResult{}, model.ErrPrivateQuotaInputInvalid
	}
	for i, window := range batch.Windows {
		if window.WindowSeconds != FiveHourSeconds && window.WindowSeconds != SevenDaySeconds ||
			i > 0 && window.WindowSeconds == batch.Windows[0].WindowSeconds {
			return ObservationResult{}, model.ErrPrivateQuotaInputInvalid
		}
	}
	var result ObservationResult
	err := store.DB.WithContext(ctx).Session(&gorm.Session{Logger: logger.Default.LogMode(logger.Silent)}).Transaction(func(tx *gorm.DB) error {
		settings, err := model.LockPrivateQuotaSettings(tx)
		if err != nil {
			return err
		}
		// 必须在取得写锁后取时钟，排队耗时不能延长已失效的写入许可。
		now := time.Now().Unix()
		if store.Clock != nil {
			now = store.Clock().Unix()
		}
		if !settings.Enabled || settings.Version != lease.SettingsVersion || lease.Owner == "" || lease.Fence < 1 ||
			settings.LeaseOwner != lease.Owner || settings.LeaseVersion != lease.Fence || settings.LeaseUntil <= now {
			return model.ErrPrivateQuotaLeaseLost
		}
		if batch.FetchedAt > now {
			return model.ErrPrivateQuotaInputInvalid
		}
		var subject model.PrivateQuotaSubject
		if err := tx.First(&subject, "id = ?", batch.SubjectID).Error; err != nil {
			return err
		}
		if !subject.Enabled || subject.SourceChannelID != batch.SourceChannelID || subject.RulesVersion != RulesVersion {
			return model.ErrPrivateQuotaConflict
		}
		var channel model.Channel
		if err := tx.First(&channel, "id = ?", batch.SourceChannelID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrSourceUnavailable
			}
			return err
		}
		if err := ValidateSubjectSource(subject, &channel, settings.FingerprintKey); err != nil {
			return err
		}
		var existing []model.PrivateQuotaSnapshot
		if err := tx.Where("subject_id = ? AND sample_key = ?", subject.ID, batch.SampleKey).Find(&existing).Error; err != nil {
			return err
		}
		if len(existing) != 0 {
			if len(existing) != len(batch.Windows) {
				return model.ErrPrivateQuotaConflict
			}
			for _, window := range batch.Windows {
				matched := false
				for _, row := range existing {
					if row.WindowSeconds != window.WindowSeconds {
						continue
					}
					matched = row.FetchedAt == batch.FetchedAt && row.StartedAt == batch.StartedAt && row.SourceChannelID == batch.SourceChannelID &&
						row.ResetAt == window.ResetAt && (row.UsedBasisPoints == nil && window.UsedBasisPoints == nil ||
						row.UsedBasisPoints != nil && window.UsedBasisPoints != nil && *row.UsedBasisPoints == *window.UsedBasisPoints)
				}
				if !matched {
					return model.ErrPrivateQuotaConflict
				}
			}
			result = ObservationResult{Revision: subject.Version, Duplicate: true}
			return nil
		}
		if subject.Version != batch.SubjectVersion || subject.Version == math.MaxInt64 {
			return model.ErrPrivateQuotaConflict
		}
		var latest model.PrivateQuotaSnapshot
		err = tx.Where("subject_id = ?", subject.ID).Order("fetched_at DESC, id DESC").Take(&latest).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if err == nil && batch.FetchedAt <= latest.FetchedAt {
			return model.ErrPrivateQuotaConflict
		}
		segment := latest.SegmentID
		if segment == "" {
			segment = uuid.NewString()
		}
		valid := false
		for _, window := range batch.Windows {
			sample := Sample{ID: 1, SubjectID: subject.ID, BatchID: batch.SampleKey, SegmentID: segment,
				FetchedAt: batch.FetchedAt, WindowSeconds: window.WindowSeconds, UsedBasisPoints: window.UsedBasisPoints,
				ResetAt: window.ResetAt, IntervalSeconds: int64(settings.IntervalMinutes) * 60}
			quality := sample.invalidReason()
			valid = valid || quality == ""
			row := model.PrivateQuotaSnapshot{SubjectID: subject.ID, SampleKey: batch.SampleKey, WindowSeconds: window.WindowSeconds,
				SegmentID: segment, StartedAt: batch.StartedAt, FetchedAt: batch.FetchedAt, RecordedAt: now,
				UsedBasisPoints: window.UsedBasisPoints, ResetAt: window.ResetAt, IntervalSeconds: sample.IntervalSeconds,
				QualityCode: quality, SourceChannelID: batch.SourceChannelID}
			if err := tx.Create(&row).Error; err != nil {
				return err
			}
		}
		if err := rebuildEvents(tx, subject.ID, now); err != nil {
			return err
		}
		updates := map[string]any{"version": subject.Version + 1, "last_attempt_at": batch.StartedAt,
			"next_due_at": batch.StartedAt + int64(settings.IntervalMinutes)*60, "updated_at": now}
		if valid {
			updates["last_success_at"], updates["failure_count"], updates["error_code"] = batch.FetchedAt, 0, ""
		} else {
			updates["failure_count"], updates["error_code"] = gorm.Expr("failure_count + ?", 1), "invalid_observation"
		}
		if err := tx.Model(&model.PrivateQuotaSubject{}).Where("id = ?", subject.ID).Updates(updates).Error; err != nil {
			return err
		}
		result.Revision = subject.Version + 1
		return nil
	})
	if err != nil {
		return ObservationResult{}, err
	}
	return result, nil
}
