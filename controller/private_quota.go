package controller

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	quota "github.com/QuantumNous/new-api/service/privatequota"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// 私有账本控制器只输出独立白名单，绝不直接序列化渠道或持久化事件。
type privateQuotaSubjectView struct {
	ID              int64  `json:"id"`
	Name            string `json:"name"`
	SourceChannelID int64  `json:"source_channel_id"`
	Enabled         bool   `json:"enabled"`
	Version         int64  `json:"version"`
	NextDueAt       int64  `json:"next_due_at"`
	LastAttemptAt   int64  `json:"last_attempt_at"`
	LastSuccessAt   int64  `json:"last_success_at"`
	ErrorCode       string `json:"error_code"`
}

func privateQuotaSubjectDTO(row model.PrivateQuotaSubject) privateQuotaSubjectView {
	return privateQuotaSubjectView{row.ID, row.Name, row.SourceChannelID, row.Enabled, row.Version, row.NextDueAt, row.LastAttemptAt, row.LastSuccessAt, row.ErrorCode}
}

func privateQuotaDB(c *gin.Context) *gorm.DB {
	return model.DB.WithContext(c.Request.Context()).Session(&gorm.Session{Logger: logger.Default.LogMode(logger.Silent)})
}
func privateQuotaStore(c *gin.Context) *quota.Store { return &quota.Store{DB: privateQuotaDB(c)} }
func privateQuotaOK(c *gin.Context, data any) {
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": data})
}
func privateQuotaError(c *gin.Context, err error) {
	status, code := http.StatusInternalServerError, "PRIVATE_QUOTA_INTERNAL"
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		status, code = http.StatusNotFound, "PRIVATE_QUOTA_NOT_FOUND"
	case errors.Is(err, model.ErrPrivateQuotaInputInvalid):
		status, code = http.StatusBadRequest, "PRIVATE_QUOTA_INVALID_INPUT"
	case errors.Is(err, model.ErrPrivateQuotaConflict), errors.Is(err, quota.ErrSubjectExists):
		status, code = http.StatusConflict, "PRIVATE_QUOTA_VERSION_CONFLICT"
	case errors.Is(err, quota.ErrIdentityChanged):
		status, code = http.StatusConflict, "PRIVATE_QUOTA_IDENTITY_CHANGED"
	case errors.Is(err, quota.ErrSourceUnavailable):
		status, code = http.StatusBadRequest, "PRIVATE_QUOTA_SOURCE_UNAVAILABLE"
	case errors.Is(err, model.ErrPrivateQuotaStateInvalid):
		status, code = http.StatusServiceUnavailable, "PRIVATE_QUOTA_RECOVERY_REQUIRED"
	}
	c.JSON(status, gin.H{"success": false, "code": code, "message": http.StatusText(status)})
}

func GetPrivateQuotaSettings(c *gin.Context) {
	settings, err := model.GetPrivateQuotaSettings(privateQuotaDB(c))
	if err != nil {
		privateQuotaError(c, err)
		return
	}
	privateQuotaOK(c, model.PrivateQuotaSettingsView{Enabled: settings.Enabled, IntervalMinutes: settings.IntervalMinutes, Version: settings.Version, UpdatedAt: settings.UpdatedAt})
}

func UpdatePrivateQuotaSettings(c *gin.Context) {
	var input struct {
		Enabled         *bool `json:"enabled"`
		IntervalMinutes int   `json:"interval_minutes"`
		Version         int64 `json:"version"`
	}
	if c.ShouldBindJSON(&input) != nil || input.Enabled == nil {
		privateQuotaError(c, model.ErrPrivateQuotaInputInvalid)
		return
	}
	view, err := model.UpdatePrivateQuotaSettings(privateQuotaDB(c), model.PrivateQuotaSettingsChange{Enabled: *input.Enabled, IntervalMinutes: input.IntervalMinutes, Version: input.Version, ActorID: int64(c.GetInt("id")), Now: time.Now().Unix()})
	if err != nil {
		privateQuotaError(c, err)
		return
	}
	privateQuotaOK(c, view)
}

// 分页在转换为int前校验，避免极大page造成OFFSET溢出或无界读取。
func privateQuotaPage(c *gin.Context) (int, int, bool) {
	page, e1 := strconv.ParseInt(c.DefaultQuery("page", "1"), 10, 64)
	size, e2 := strconv.ParseInt(c.DefaultQuery("page_size", "20"), 10, 64)
	if e1 != nil || e2 != nil || page < 1 || page > 1000000 || size < 1 || size > 100 {
		privateQuotaError(c, model.ErrPrivateQuotaInputInvalid)
		return 0, 0, false
	}
	return int(page), int(size), true
}

func ListPrivateQuotaChannels(c *gin.Context) {
	page, size, ok := privateQuotaPage(c)
	if !ok {
		return
	}
	settings, err := model.GetPrivateQuotaSettings(privateQuotaDB(c))
	if err != nil {
		privateQuotaError(c, err)
		return
	}
	var channels []model.Channel
	query := privateQuotaDB(c).Where("type = ? AND status = ?", constant.ChannelTypeCodex, common.ChannelStatusEnabled)
	var total int64
	if err := query.Model(&model.Channel{}).Count(&total).Error; err != nil {
		privateQuotaError(c, err)
		return
	}
	if err := query.Order("id").Offset((page - 1) * size).Limit(size).Find(&channels).Error; err != nil {
		privateQuotaError(c, err)
		return
	}
	type channelView struct {
		ID    int    `json:"id"`
		Name  string `json:"name"`
		Type  int    `json:"type"`
		Bound bool   `json:"bound"`
	}
	items := make([]channelView, 0, len(channels))
	for _, channel := range channels {
		fingerprint, err := quota.ChannelFingerprint(&channel, settings.FingerprintKey)
		if err != nil {
			continue
		}
		var count int64
		if err := privateQuotaDB(c).Model(&model.PrivateQuotaSubject{}).Where("identity_fingerprint = ?", fingerprint).Count(&count).Error; err != nil {
			privateQuotaError(c, err)
			return
		}
		items = append(items, channelView{channel.Id, channel.Name, channel.Type, count > 0})
	}
	// total是扫描范围的渠道数；不返回被过滤渠道的任何详情。
	privateQuotaOK(c, gin.H{"items": items, "total": total, "page": page, "page_size": size})
}

func ListPrivateQuotaSubjects(c *gin.Context) {
	page, size, ok := privateQuotaPage(c)
	if !ok {
		return
	}
	var total int64
	var rows []model.PrivateQuotaSubject
	query := privateQuotaDB(c).Model(&model.PrivateQuotaSubject{})
	if err := query.Count(&total).Error; err != nil {
		privateQuotaError(c, err)
		return
	}
	if err := query.Order("id").Offset((page - 1) * size).Limit(size).Find(&rows).Error; err != nil {
		privateQuotaError(c, err)
		return
	}
	items := make([]privateQuotaSubjectView, 0, len(rows))
	for _, row := range rows {
		items = append(items, privateQuotaSubjectDTO(row))
	}
	privateQuotaOK(c, gin.H{"items": items, "total": total, "page": page, "page_size": size})
}

func CreatePrivateQuotaSubject(c *gin.Context) { savePrivateQuotaSubject(c, false) }
func UpdatePrivateQuotaSubject(c *gin.Context) { savePrivateQuotaSubject(c, true) }
func savePrivateQuotaSubject(c *gin.Context, update bool) {
	var input struct {
		Name            string `json:"name"`
		SourceChannelID int64  `json:"source_channel_id"`
		Enabled         *bool  `json:"enabled"`
		Version         int64  `json:"version"`
	}
	if c.ShouldBindJSON(&input) != nil || input.Enabled == nil {
		privateQuotaError(c, model.ErrPrivateQuotaInputInvalid)
		return
	}
	change := quota.SubjectChange{Name: input.Name, SourceChannelID: input.SourceChannelID, Enabled: *input.Enabled, Version: input.Version, ActorID: int64(c.GetInt("id")), Now: time.Now().Unix()}
	var row model.PrivateQuotaSubject
	var err error
	if update {
		id, parseErr := strconv.ParseInt(c.Param("id"), 10, 64)
		if parseErr != nil || id < 1 {
			privateQuotaError(c, model.ErrPrivateQuotaInputInvalid)
			return
		}
		row, err = privateQuotaStore(c).UpdateSubject(c.Request.Context(), id, change)
	} else {
		row, err = privateQuotaStore(c).CreateSubject(c.Request.Context(), change)
	}
	if err != nil {
		privateQuotaError(c, err)
		return
	}
	privateQuotaOK(c, privateQuotaSubjectDTO(row))
}

type privateQuotaQuery struct {
	Month            string
	From, To, Window int64
	Location         *time.Location
	Report           quota.Report
}

func readPrivateQuotaQuery(c *gin.Context) (privateQuotaQuery, bool) {
	var query privateQuotaQuery
	query.Location = time.FixedZone("Asia/Shanghai", 8*3600)
	query.Month = c.DefaultQuery("month", time.Now().In(query.Location).Format("2006-01"))
	start, err := time.ParseInLocation("2006-01", query.Month, query.Location)
	id, errID := strconv.ParseInt(c.Query("subject_id"), 10, 64)
	window, errWindow := strconv.ParseInt(c.DefaultQuery("window_seconds", "604800"), 10, 64)
	if err != nil || start.Format("2006-01") != query.Month || start.Year() < 2000 || start.Year() > 2100 || errID != nil || id < 1 || errWindow != nil || (window != quota.SevenDaySeconds && window != quota.FiveHourSeconds) {
		privateQuotaError(c, model.ErrPrivateQuotaInputInvalid)
		return query, false
	}
	query.From, query.To, query.Window = start.Unix(), start.AddDate(0, 1, 0).Unix(), window
	query.Report, err = privateQuotaStore(c).ReadReport(c.Request.Context(), id)
	if err != nil {
		privateQuotaError(c, err)
		return query, false
	}
	return query, true
}

func privateQuotaCycleDTO(cycle quota.Cycle) gin.H {
	return gin.H{"window_seconds": cycle.WindowSeconds, "started_at": cycle.StartedAt, "ended_at": cycle.EndedAt, "ended_by": cycle.EndedBy,
		"expected_reset_at": cycle.ExpectedResetAt, "high_water_basis_points": cycle.HighWaterBasisPoints, "observed_basis_points": cycle.ObservedBasisPoints,
		"partial": cycle.Partial, "has_gap": cycle.HasGap, "has_correction": cycle.HasCorrection}
}
func privateQuotaWindowDTO(window quota.WindowSummary) gin.H {
	var current any
	if window.Current != nil {
		current = privateQuotaCycleDTO(*window.Current)
	}
	return gin.H{"observed_units": float64(window.ObservedBasisPoints) / 10000, "unused_units": float64(window.UnusedBasisPoints) / 10000,
		"reset_count": window.ResetCount, "early_resets": window.EarlyResets, "expired_resets": window.ExpiredResets, "manual_resets": window.ManualResets,
		"pending_events": window.PendingEvents, "pending_since": window.PendingSince, "partial_cycles": window.PartialCycles, "excluded_cycles": window.ExcludedCycles, "evaluated_cycles": window.EvaluatedCycles, "current": current}
}
func privateQuotaSummaryDTO(summary quota.MonthSummary) gin.H {
	return gin.H{"month": summary.Month, "timezone": summary.Timezone, "coverage_start": summary.CoverageStart, "rules_version": summary.RulesVersion,
		"seven_day": privateQuotaWindowDTO(summary.SevenDay), "five_hour": privateQuotaWindowDTO(summary.FiveHour), "gap_seconds": summary.GapSeconds, "invalid_samples": summary.InvalidSamples}
}

func GetPrivateQuotaSummary(c *gin.Context) {
	query, ok := readPrivateQuotaQuery(c)
	if !ok {
		return
	}
	summary, err := quota.SummarizeMonth(query.Report.Ledger, query.Month, query.Location)
	if err != nil {
		privateQuotaError(c, err)
		return
	}
	// 当前比例取最后一条原始观测而非高水位；最新值未知时不能回退到旧的成功比例。
	current := make(map[string]any)
	for _, row := range query.Report.Snapshots {
		current[strconv.FormatInt(row.WindowSeconds, 10)] = gin.H{"used_basis_points": row.UsedBasisPoints, "reset_at": row.ResetAt, "fetched_at": row.FetchedAt, "quality_code": row.QualityCode,
			"stale": time.Now().Unix()-row.FetchedAt > row.IntervalSeconds*2}
	}
	privateQuotaOK(c, gin.H{"subject": privateQuotaSubjectDTO(query.Report.Subject), "collection_enabled": query.Report.CollectionEnabled, "ledger_revision": query.Report.Subject.Version, "summary": privateQuotaSummaryDTO(summary), "current": current, "last_observed_at": query.Report.Ledger.LastObservedAt})
}

func GetPrivateQuotaCycles(c *gin.Context) {
	page, size, ok := privateQuotaPage(c)
	if !ok {
		return
	}
	query, ok := readPrivateQuotaQuery(c)
	if !ok {
		return
	}
	items := make([]gin.H, 0)
	for _, cycle := range query.Report.Ledger.Cycles {
		if cycle.WindowSeconds == query.Window && cycle.StartedAt < query.To && (cycle.EndedAt == 0 || cycle.EndedAt >= query.From) {
			items = append(items, privateQuotaCycleDTO(cycle))
		}
	}
	from := min((page-1)*size, len(items))
	to := min(from+size, len(items))
	privateQuotaOK(c, gin.H{"items": items[from:to], "total": len(items), "ledger_revision": query.Report.Subject.Version})
}

func GetPrivateQuotaTrend(c *gin.Context) {
	query, ok := readPrivateQuotaQuery(c)
	if !ok {
		return
	}
	values := make(map[string]int64)
	observed := make(map[string]bool)
	gaps := make(map[string]bool)
	for _, sample := range query.Report.Snapshots {
		if sample.WindowSeconds == query.Window && sample.FetchedAt >= query.From && sample.FetchedAt < query.To && sample.QualityCode == "" {
			observed[time.Unix(sample.FetchedAt, 0).In(query.Location).Format("2006-01-02")] = true
		}
	}
	for _, delta := range query.Report.Ledger.Deltas {
		if delta.WindowSeconds == query.Window && delta.FetchedAt >= query.From && delta.FetchedAt < query.To {
			values[time.Unix(delta.FetchedAt, 0).In(query.Location).Format("2006-01-02")] += delta.BasisPoints
		}
	}
	items := make([]gin.H, 0, 31)
	for day := time.Unix(query.From, 0).In(query.Location); day.Unix() < query.To; day = day.AddDate(0, 0, 1) {
		key := day.Format("2006-01-02")
		for _, gap := range query.Report.Ledger.Gaps {
			if gap.WindowSeconds == query.Window && gap.From < day.AddDate(0, 0, 1).Unix() && gap.To > day.Unix() {
				gaps[key] = true
			}
		}
		var value any
		if observed[key] {
			value = float64(values[key]) / 10000
		}
		items = append(items, gin.H{"date": key, "observed_units": value, "has_gap": gaps[key]})
	}
	privateQuotaOK(c, gin.H{"items": items, "ledger_revision": query.Report.Subject.Version})
}

func GetPrivateQuotaEvents(c *gin.Context) {
	page, size, ok := privateQuotaPage(c)
	if !ok {
		return
	}
	query, ok := readPrivateQuotaQuery(c)
	if !ok {
		return
	}
	status := c.Query("status")
	if status != "" && status != "pending" && status != "accepted" && status != "corrected" && status != "inconclusive" && status != "superseded" {
		privateQuotaError(c, model.ErrPrivateQuotaInputInvalid)
		return
	}
	rows := make([]model.PrivateQuotaEvent, 0)
	for i := len(query.Report.Events) - 1; i >= 0; i-- {
		row := query.Report.Events[i]
		if row.WindowSeconds != query.Window || row.Kind == "decision" || (status != "" && row.Status != status) {
			continue
		}
		// 未解决的早期候选也需可见，否则跨月后将无法处理。
		if row.ObservedTo >= query.To || row.ObservedTo < query.From && row.Status != "pending" {
			continue
		}
		rows = append(rows, row)
	}
	from := min((page-1)*size, len(rows))
	to := min(from+size, len(rows))
	items := make([]gin.H, 0, to-from)
	for _, row := range rows[from:to] {
		previews := make(map[string]any)
		if row.Status != "superseded" {
			for _, kind := range []quota.DecisionKind{quota.ConfirmReset, quota.PlanCorrection, quota.SegmentChange} {
				ledger, err := query.Report.PreviewDecision(row, kind)
				if err != nil {
					continue
				}
				summary, err := quota.SummarizeMonth(ledger, query.Month, query.Location)
				if err != nil {
					continue
				}
				previews[string(kind)] = privateQuotaSummaryDTO(summary)
			}
		}
		history := make([]gin.H, 0)
		for _, decision := range query.Report.Events {
			if decision.Kind == "decision" && decision.WindowSeconds == row.WindowSeconds && decision.BeforeSnapshotID == row.BeforeSnapshotID && decision.AfterSnapshotID == row.AfterSnapshotID {
				history = append(history, gin.H{"id": decision.ID, "decision": decision.Decision, "reason": decision.Reason, "actor_id": decision.ActorID, "created_at": decision.CreatedAt, "supersedes_id": decision.SupersedesEventID})
			}
		}
		items = append(items, gin.H{"id": row.ID, "kind": row.Kind, "status": row.Status, "version": row.Version, "observed_from": row.ObservedFrom, "observed_to": row.ObservedTo, "old_reset_at": row.OldResetAt, "new_reset_at": row.NewResetAt, "previews": previews, "decisions": history})
	}
	privateQuotaOK(c, gin.H{"items": items, "total": len(rows), "ledger_revision": query.Report.Subject.Version})
}

func DecidePrivateQuotaEvent(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	var input struct {
		Decision       quota.DecisionKind `json:"decision"`
		Reason         string             `json:"reason"`
		EventVersion   int64              `json:"event_version"`
		SubjectVersion int64              `json:"subject_version"`
	}
	if err != nil || id < 1 || c.ShouldBindJSON(&input) != nil {
		privateQuotaError(c, model.ErrPrivateQuotaInputInvalid)
		return
	}
	result, err := privateQuotaStore(c).DecideEvent(c.Request.Context(), id, quota.DecisionChange{Kind: input.Decision, Reason: input.Reason, EventVersion: input.EventVersion, SubjectVersion: input.SubjectVersion, ActorID: int64(c.GetInt("id"))})
	if err != nil {
		privateQuotaError(c, err)
		return
	}
	privateQuotaOK(c, gin.H{"ledger_revision": result.Revision})
}
