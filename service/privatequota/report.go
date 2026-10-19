package privatequota

import (
	"context"
	"fmt"

	"github.com/QuantumNous/new-api/model"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Report只在服务内使用，控制器必须映射白名单DTO。全部查询来自同一短事务。
type Report struct {
	CollectionEnabled bool
	Subject           model.PrivateQuotaSubject
	Ledger            Ledger
	Snapshots         []model.PrivateQuotaSnapshot
	Events            []model.PrivateQuotaEvent
}

func (store *Store) ReadReport(ctx context.Context, id int64) (Report, error) {
	var report Report
	err := store.DB.WithContext(ctx).Session(&gorm.Session{Logger: logger.Default.LogMode(logger.Silent)}).Transaction(func(tx *gorm.DB) error {
		settings, err := model.LockPrivateQuotaSettings(tx)
		if err != nil {
			return err
		}
		report.CollectionEnabled = settings.Enabled
		if err := tx.First(&report.Subject, "id = ?", id).Error; err != nil {
			return err
		}
		if err := tx.Where("subject_id = ?", id).Order("fetched_at, id").Find(&report.Snapshots).Error; err != nil {
			return err
		}
		if err := tx.Where("subject_id = ?", id).Order("id").Find(&report.Events).Error; err != nil {
			return err
		}
		report.Ledger, err = report.replay(nil)
		return err
	})
	return report, err
}

func (report Report) replay(extra *Decision) (Ledger, error) {
	samples := make([]Sample, 0, len(report.Snapshots))
	for _, row := range report.Snapshots {
		samples = append(samples, Sample{ID: row.ID, SubjectID: row.SubjectID, BatchID: row.SampleKey, WindowSeconds: row.WindowSeconds,
			SegmentID: row.SegmentID, FetchedAt: row.FetchedAt, UsedBasisPoints: row.UsedBasisPoints, ResetAt: row.ResetAt, IntervalSeconds: row.IntervalSeconds})
	}
	decisions := make([]Decision, 0)
	for _, row := range report.Events {
		if row.Kind != "decision" {
			continue
		}
		decisions = append(decisions, Decision{ID: row.ID, EventKey: fmt.Sprintf("%d/%d/%d", row.WindowSeconds, row.BeforeSnapshotID, row.AfterSnapshotID), Kind: DecisionKind(row.Decision), Reason: row.Reason, SupersedesID: row.SupersedesEventID})
	}
	if extra != nil {
		decisions = append(decisions, *extra)
	}
	ledger, err := BuildLedger(samples, decisions)
	if ledger.SubjectID == 0 {
		ledger.SubjectID = report.Subject.ID
	}
	return ledger, err
}

// PreviewDecision与保存使用同一纯规则，不写临时决定、不查询上游。
func (report Report) PreviewDecision(event model.PrivateQuotaEvent, kind DecisionKind) (Ledger, error) {
	var lastID, replaces int64
	for _, row := range report.Events {
		lastID = max(lastID, row.ID)
		if row.Kind == "decision" && row.WindowSeconds == event.WindowSeconds && row.BeforeSnapshotID == event.BeforeSnapshotID && row.AfterSnapshotID == event.AfterSnapshotID {
			replaces = row.ID
		}
	}
	return report.replay(&Decision{ID: lastID + 1, EventKey: fmt.Sprintf("%d/%d/%d", event.WindowSeconds, event.BeforeSnapshotID, event.AfterSnapshotID), Kind: kind, Reason: "preview", SupersedesID: replaces})
}
