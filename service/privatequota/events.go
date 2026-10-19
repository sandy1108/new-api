package privatequota

import (
	"context"
	"fmt"
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

type DecisionChange struct {
	SubjectVersion int64
	EventVersion   int64
	Kind           DecisionKind
	Reason         string
	ActorID        int64
}

// ReadLedger在一致短事务内重放本地主体历史，绝不隐式读取上游。
func (store *Store) ReadLedger(ctx context.Context, subjectID int64) (Ledger, int64, error) {
	var ledger Ledger
	var revision int64
	err := store.DB.WithContext(ctx).Session(&gorm.Session{Logger: logger.Default.LogMode(logger.Silent)}).Transaction(func(tx *gorm.DB) error {
		if _, err := model.LockPrivateQuotaSettings(tx); err != nil {
			return err
		}
		var subject model.PrivateQuotaSubject
		if err := tx.First(&subject, "id = ?", subjectID).Error; err != nil {
			return err
		}
		var err error
		ledger, err = replayLedger(tx, subjectID)
		revision = subject.Version
		return err
	})
	return ledger, revision, err
}

// DecideEvent只追加人工决定，以主体及事件双版本保护预览，重放失败则整笔回滚。
func (store *Store) DecideEvent(ctx context.Context, eventID int64, change DecisionChange) (ObservationResult, error) {
	change.Reason = strings.TrimSpace(change.Reason)
	if eventID < 1 || change.ActorID < 1 || change.SubjectVersion < 1 || change.SubjectVersion == math.MaxInt64 || change.EventVersion < 1 ||
		change.Reason == "" || !utf8.ValidString(change.Reason) || utf8.RuneCountInString(change.Reason) > 500 || strings.ContainsFunc(change.Reason, unicode.IsControl) ||
		(change.Kind != ConfirmReset && change.Kind != PlanCorrection && change.Kind != SegmentChange) {
		return ObservationResult{}, model.ErrPrivateQuotaInputInvalid
	}
	var result ObservationResult
	err := store.DB.WithContext(ctx).Session(&gorm.Session{Logger: logger.Default.LogMode(logger.Silent)}).Transaction(func(tx *gorm.DB) error {
		if _, err := model.LockPrivateQuotaSettings(tx); err != nil {
			return err
		}
		var event model.PrivateQuotaEvent
		if err := tx.First(&event, "id = ?", eventID).Error; err != nil {
			return err
		}
		if event.WindowSeconds == 0 || event.Kind == "decision" || event.Status == "superseded" || event.Version != change.EventVersion {
			return model.ErrPrivateQuotaConflict
		}
		var subject model.PrivateQuotaSubject
		if err := tx.First(&subject, "id = ?", event.SubjectID).Error; err != nil {
			return err
		}
		if subject.Version != change.SubjectVersion {
			return model.ErrPrivateQuotaConflict
		}
		var previous model.PrivateQuotaEvent
		if err := tx.Where("subject_id = ? AND kind = ? AND window_seconds = ? AND before_snapshot_id = ? AND after_snapshot_id = ?",
			event.SubjectID, "decision", event.WindowSeconds, event.BeforeSnapshotID, event.AfterSnapshotID).Order("id DESC").Limit(1).Find(&previous).Error; err != nil {
			return err
		}
		now := time.Now().Unix()
		if store.Clock != nil {
			now = store.Clock().Unix()
		}
		decision := model.PrivateQuotaEvent{SubjectID: event.SubjectID, WindowSeconds: event.WindowSeconds, SegmentID: event.SegmentID,
			EventKey: "decision/" + uuid.NewString(), Kind: "decision", Status: "accepted", BeforeSnapshotID: event.BeforeSnapshotID,
			AfterSnapshotID: event.AfterSnapshotID, RulesVersion: RulesVersion, Version: 1, Decision: string(change.Kind),
			ActorID: change.ActorID, Reason: change.Reason, SupersedesEventID: previous.ID, CreatedAt: now}
		if err := tx.Create(&decision).Error; err != nil {
			return err
		}
		if err := rebuildEvents(tx, subject.ID, now); err != nil {
			return err
		}
		if err := tx.Model(&model.PrivateQuotaSubject{}).Where("id = ?", subject.ID).
			Updates(map[string]any{"version": subject.Version + 1, "updated_at": now}).Error; err != nil {
			return err
		}
		result.Revision = subject.Version + 1
		return nil
	})
	return result, err
}

// replayLedger使用不可变快照和全部决定链重建；链分叉/失去证据时失败关闭，不吞掉决定。
func replayLedger(tx *gorm.DB, subjectID int64) (Ledger, error) {
	var rows []model.PrivateQuotaSnapshot
	if err := tx.Where("subject_id = ?", subjectID).Order("fetched_at, id").Find(&rows).Error; err != nil {
		return Ledger{}, err
	}
	samples := make([]Sample, 0, len(rows))
	for _, row := range rows {
		samples = append(samples, Sample{ID: row.ID, SubjectID: subjectID, BatchID: row.SampleKey, WindowSeconds: row.WindowSeconds,
			SegmentID: row.SegmentID, FetchedAt: row.FetchedAt, UsedBasisPoints: row.UsedBasisPoints, ResetAt: row.ResetAt, IntervalSeconds: row.IntervalSeconds})
	}
	var rowsDecisions []model.PrivateQuotaEvent
	if err := tx.Where("subject_id = ? AND kind = ?", subjectID, "decision").Order("id").Find(&rowsDecisions).Error; err != nil {
		return Ledger{}, err
	}
	decisions := make([]Decision, 0, len(rowsDecisions))
	for _, row := range rowsDecisions {
		decisions = append(decisions, Decision{ID: row.ID, EventKey: fmt.Sprintf("%d/%d/%d", row.WindowSeconds, row.BeforeSnapshotID, row.AfterSnapshotID),
			Kind: DecisionKind(row.Decision), Reason: row.Reason, SupersedesID: row.SupersedesEventID})
	}
	ledger, err := BuildLedger(samples, decisions)
	if ledger.SubjectID == 0 {
		ledger.SubjectID = subjectID
	}
	return ledger, err
}

// rebuildEvents与快照/决定处于同一事务。投影可更新，证据和人工决定永不覆盖。
func rebuildEvents(tx *gorm.DB, subjectID, now int64) error {
	ledger, err := replayLedger(tx, subjectID)
	if err != nil {
		return err
	}
	var previous []model.PrivateQuotaEvent
	if err := tx.Where("subject_id = ? AND window_seconds > 0 AND kind <> ?", subjectID, "decision").Find(&previous).Error; err != nil {
		return err
	}
	byKey := make(map[string]model.PrivateQuotaEvent, len(previous))
	for _, row := range previous {
		byKey[row.EventKey] = row
	}
	active := make(map[string]bool, len(ledger.Events))
	for _, event := range ledger.Events {
		key := fmt.Sprintf("boundary/%d/%s", subjectID, event.Key)
		active[key] = true
		if row, exists := byKey[key]; exists {
			if row.Kind == string(event.Kind) && row.Status == string(event.Status) {
				continue
			}
			if err := tx.Model(&model.PrivateQuotaEvent{}).Where("id = ?", row.ID).
				Updates(map[string]any{"kind": string(event.Kind), "status": string(event.Status), "version": row.Version + 1}).Error; err != nil {
				return err
			}
			continue
		}
		row := model.PrivateQuotaEvent{SubjectID: subjectID, WindowSeconds: event.WindowSeconds, EventKey: key, Kind: string(event.Kind),
			Status: string(event.Status), BeforeSnapshotID: event.BeforeSnapshotID, AfterSnapshotID: event.AfterSnapshotID,
			ObservedFrom: event.ObservedFrom, ObservedTo: event.ObservedTo, OldResetAt: event.OldResetAt, NewResetAt: event.NewResetAt,
			RulesVersion: RulesVersion, Version: 1, CreatedAt: now}
		var after model.PrivateQuotaSnapshot
		if err := tx.First(&after, "id = ? AND subject_id = ?", event.AfterSnapshotID, subjectID).Error; err != nil {
			return err
		}
		row.SegmentID = after.SegmentID
		if err := tx.Create(&row).Error; err != nil {
			return err
		}
	}
	for _, row := range previous {
		if active[row.EventKey] || row.Status == "superseded" {
			continue
		}
		if err := tx.Model(&model.PrivateQuotaEvent{}).Where("id = ?", row.ID).
			Updates(map[string]any{"status": "superseded", "version": row.Version + 1}).Error; err != nil {
			return err
		}
	}
	return nil
}
