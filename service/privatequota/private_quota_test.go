package privatequota

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestCollectorUsesOnlyFreshDueObservationsAndBacksOff(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "collector.db")+"?_pragma=busy_timeout(5000)"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
	require.NoError(t, db.AutoMigrate(&model.Channel{}))
	require.NoError(t, model.MigratePrivateQuotaTables(db))
	const at = int64(1800050000)
	clock := at
	settings, err := model.InitializePrivateQuotaSettings(db, at)
	require.NoError(t, err)
	require.NoError(t, db.Create(&model.Channel{Id: 71, Type: constant.ChannelTypeCodex, Status: common.ChannelStatusEnabled,
		Name: "synthetic", Key: `{"account_id":"collector-account","access_token":"synthetic"}`}).Error)
	store := Store{DB: db, Clock: func() time.Time { return time.Unix(clock, 0) }}
	subject, err := store.CreateSubject(context.Background(), SubjectChange{Name: "Collector", SourceChannelID: 71, Enabled: true, ActorID: 42, Now: at})
	require.NoError(t, err)
	calls := 0
	var fetchErr error
	collector := Collector{Store: &store, Owner: "synthetic-collector", Fetch: func(ctx context.Context, subject model.PrivateQuotaSubject, key string) ([]Observation, int64, error) {
		calls++
		if fetchErr != nil {
			return nil, 3600, fetchErr
		}
		used := 3000
		return []Observation{{WindowSeconds: SevenDaySeconds, UsedBasisPoints: &used, ResetAt: at + 604800}}, 0, nil
	}}
	require.NoError(t, collector.Tick(context.Background()))
	assert.Zero(t, calls, "默认关闭不调用上游")
	_, err = model.UpdatePrivateQuotaSettings(db, model.PrivateQuotaSettingsChange{Enabled: true, IntervalMinutes: 10, Version: settings.Version, ActorID: 42, Now: at})
	require.NoError(t, err)
	require.NoError(t, collector.Tick(context.Background()))
	assert.Equal(t, 1, calls)
	require.NoError(t, collector.Tick(context.Background()))
	assert.Equal(t, 1, calls, "未到due不重复采集")
	clock += 600
	fetchErr = ErrUpstreamLimited
	require.NoError(t, collector.Tick(context.Background()))
	assert.Equal(t, 2, calls)
	var stored model.PrivateQuotaSubject
	require.NoError(t, db.First(&stored, subject.ID).Error)
	assert.Equal(t, clock+3600, stored.NextDueAt)
	assert.Equal(t, 1, stored.FailureCount)
	assert.Equal(t, "upstream_limited", stored.ErrorCode)
	assert.Equal(t, at, stored.LastSuccessAt)
	var count int64
	require.NoError(t, db.Model(&model.PrivateQuotaSnapshot{}).Count(&count).Error)
	assert.EqualValues(t, 1, count)
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	require.ErrorIs(t, collector.Tick(cancelled), context.Canceled)
	// 渠道换账号时应停采，不能把新账号接在旧账本上，也不能访问上游。
	clock = stored.NextDueAt
	require.NoError(t, db.Model(&model.Channel{}).Where("id = ?", 71).Update("key", `{"account_id":"different-account","access_token":"synthetic"}`).Error)
	collector.Fetch = nil
	require.NoError(t, collector.Tick(context.Background()))
	require.NoError(t, db.First(&stored, subject.ID).Error)
	assert.False(t, stored.Enabled)
	assert.Equal(t, "identity_changed", stored.ErrorCode)
	assert.Zero(t, stored.NextDueAt)
	assert.Equal(t, at, stored.LastSuccessAt)
}

func TestFreshProviderReadsOnlyUsageAndRetainsRetryAfter(t *testing.T) {
	status := http.StatusOK
	hits := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/backend-api/wham/usage", r.URL.Path)
		assert.Equal(t, "Bearer synthetic-access", r.Header.Get("Authorization"))
		w.Header().Set("Retry-After", "1200")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(`{"rate_limit":{"primary_window":{"used_percent":30,"limit_window_seconds":604800,"reset_at":1800600000}}}`))
	}))
	defer server.Close()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "provider.db")), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
	require.NoError(t, db.AutoMigrate(&model.Channel{}))
	require.NoError(t, model.MigratePrivateQuotaTables(db))
	settings, err := model.InitializePrivateQuotaSettings(db, 1800000000)
	require.NoError(t, err)
	source := server.URL
	require.NoError(t, db.Create(&model.Channel{Id: 72, Type: constant.ChannelTypeCodex, Status: common.ChannelStatusEnabled, BaseURL: &source, Key: `{"account_id":"synthetic-account","access_token":"synthetic-access"}`}).Error)
	store := Store{DB: db}
	subject, err := store.CreateSubject(context.Background(), SubjectChange{Name: "Provider", SourceChannelID: 72, ActorID: 42, Now: 1800000000})
	require.NoError(t, err)
	rows, retry, err := store.FetchFresh(context.Background(), subject, settings.FingerprintKey)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, 3000, *rows[0].UsedBasisPoints)
	assert.Zero(t, retry)
	status = http.StatusTooManyRequests
	_, retry, err = store.FetchFresh(context.Background(), subject, settings.FingerprintKey)
	require.ErrorIs(t, err, ErrUpstreamLimited)
	assert.EqualValues(t, 1200, retry)
	assert.Equal(t, 2, hits)
}

// 错误退避不能让普通上游故障停采一年；只有429尊重更长的Retry-After。
func TestCollectorFailureBackoffAndInFlightFencing(t *testing.T) {
	for _, scenario := range []string{"ordinary_failure", "limited", "second_worker", "cancel", "disable", "interval_change"} {
		t.Run(scenario, func(t *testing.T) {
			db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "collector.db")+"?_pragma=busy_timeout(5000)"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
			require.NoError(t, err)
			sqlDB, err := db.DB()
			require.NoError(t, err)
			t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
			require.NoError(t, db.AutoMigrate(&model.Channel{}))
			require.NoError(t, model.MigratePrivateQuotaTables(db))
			const at = int64(1800050000)
			settings, err := model.InitializePrivateQuotaSettings(db, at)
			require.NoError(t, err)
			require.NoError(t, db.Create(&model.Channel{Id: 73, Type: constant.ChannelTypeCodex, Status: common.ChannelStatusEnabled,
				Name: "synthetic", Key: `{"account_id":"inflight-account","access_token":"synthetic"}`}).Error)
			store := Store{DB: db, Clock: func() time.Time { return time.Unix(at, 0) }}
			subject, err := store.CreateSubject(context.Background(), SubjectChange{Name: "Inflight", SourceChannelID: 73, Enabled: true, ActorID: 42, Now: at})
			require.NoError(t, err)
			enabledSettings, err := model.UpdatePrivateQuotaSettings(db, model.PrivateQuotaSettingsChange{Enabled: true, IntervalMinutes: 10, Version: settings.Version, ActorID: 42, Now: at})
			require.NoError(t, err)
			entered, release := make(chan struct{}), make(chan struct{})
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			defer func() {
				select {
				case <-release:
				default:
					close(release)
				}
			}()
			collector := Collector{Store: &store, Owner: "first", Fetch: func(ctx context.Context, _ model.PrivateQuotaSubject, _ string) ([]Observation, int64, error) {
				close(entered)
				select {
				case <-ctx.Done():
					return nil, 0, ctx.Err()
				case <-release:
				}
				if scenario == "ordinary_failure" {
					return nil, 31536000, ErrUpstreamUnavailable
				}
				if scenario == "limited" {
					return nil, 31536000, ErrUpstreamLimited
				}
				used := 3000
				return []Observation{{WindowSeconds: SevenDaySeconds, UsedBasisPoints: &used, ResetAt: at + 604800}}, 0, nil
			}}
			result := make(chan error, 1)
			go func() { result <- collector.Tick(ctx) }()
			select {
			case <-entered:
			case <-time.After(5 * time.Second):
				t.Fatal("collector did not start")
			}
			switch scenario {
			case "second_worker":
				other := Collector{Store: &store, Owner: "second", Fetch: func(context.Context, model.PrivateQuotaSubject, string) ([]Observation, int64, error) {
					t.Error("second worker must not access upstream while lease is held")
					return nil, 0, ErrUpstreamUnavailable
				}}
				require.NoError(t, other.Tick(context.Background()))
			case "cancel":
				cancel()
			case "disable", "interval_change":
				_, err = model.UpdatePrivateQuotaSettings(db, model.PrivateQuotaSettingsChange{Enabled: scenario != "disable", IntervalMinutes: 20, Version: enabledSettings.Version, ActorID: 42, Now: at})
				require.NoError(t, err)
			}
			close(release)
			select {
			case err = <-result:
				if scenario == "cancel" {
					require.ErrorIs(t, err, context.Canceled)
				} else if scenario == "disable" || scenario == "interval_change" {
					require.ErrorIs(t, err, model.ErrPrivateQuotaLeaseLost)
				} else {
					require.NoError(t, err)
				}
			case <-time.After(5 * time.Second):
				t.Fatal("collector did not finish")
			}
			var stored model.PrivateQuotaSubject
			require.NoError(t, db.First(&stored, subject.ID).Error)
			var snapshots int64
			require.NoError(t, db.Model(&model.PrivateQuotaSnapshot{}).Count(&snapshots).Error)
			switch scenario {
			case "ordinary_failure":
				assert.EqualValues(t, at+86400, stored.NextDueAt)
				assert.Zero(t, snapshots)
			case "limited":
				assert.EqualValues(t, at+31536000, stored.NextDueAt)
				assert.Zero(t, snapshots)
			case "second_worker":
				assert.EqualValues(t, 1, snapshots)
			default:
				assert.Zero(t, snapshots)
				assert.Zero(t, stored.LastSuccessAt)
			}
			latest, err := model.GetPrivateQuotaSettings(db)
			require.NoError(t, err)
			assert.Empty(t, latest.LeaseOwner, "取消或失效后释放自身租约")
		})
	}
}

func TestParseQuotaObservationRejectsAmbiguousAndPreservesMissing(t *testing.T) {
	for _, body := range []string{`{}`, `{"rate_limit":{"primary_window":{"limit_window_seconds":604800},"secondary_window":{"limit_window_seconds":604800}}}`,
		`{"rate_limit":{"primary_window":{"limit_window_seconds":3600}}}`} {
		_, err := ParseObservations([]byte(body))
		require.Error(t, err)
	}
	rows, err := ParseObservations([]byte(`{"rate_limit":{"primary_window":{"used_percent":30.125,"limit_window_seconds":604800,"reset_at":1800600000},"secondary_window":{"limit_window_seconds":18000}}}`))
	require.NoError(t, err)
	require.Len(t, rows, 2)
	assert.Equal(t, 3013, *rows[0].UsedBasisPoints)
	assert.Nil(t, rows[1].UsedBasisPoints)
	assert.EqualValues(t, 60, RetryAfterSeconds(http.Header{"Retry-After": []string{"60"}}, time.Unix(1800000000, 0)))
	assert.EqualValues(t, 0, RetryAfterSeconds(http.Header{"Retry-After": []string{"bad"}}, time.Unix(1800000000, 0)))
}

func TestSubjectStoreSQLite(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "subjects.db")+"?_pragma=busy_timeout(5000)"),
		&gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
	quotaSubjectStoreContract(t, db)
}

// 账号串账、重复绑定、漏审计或失败后部分提交都会破坏此契约。
func quotaSubjectStoreContract(t *testing.T, db *gorm.DB) {
	t.Helper()
	require.NoError(t, db.AutoMigrate(&model.Channel{}))
	require.NoError(t, model.MigratePrivateQuotaTables(db))
	settings, err := model.InitializePrivateQuotaSettings(db, 1800000000)
	require.NoError(t, err)
	for _, id := range []int{17, 29} {
		require.NoError(t, db.Create(&model.Channel{Id: id, Type: constant.ChannelTypeCodex,
			Status: common.ChannelStatusEnabled, Name: "synthetic",
			Key: `{"account_id":"account-A","access_token":"synthetic-access"}`}).Error)
	}
	store := Store{DB: db}
	change := SubjectChange{Name: " My account ", SourceChannelID: 17, Enabled: true, ActorID: 42, Now: 1800000000}
	subject, err := store.CreateSubject(context.Background(), change)
	require.NoError(t, err)
	require.Positive(t, subject.ID)
	assert.Equal(t, "My account", subject.Name)
	assert.EqualValues(t, 1, subject.Version)
	assert.True(t, subject.Enabled)
	assert.EqualValues(t, change.Now, subject.NextDueAt)
	assert.EqualValues(t, RulesVersion, subject.RulesVersion)
	change.SourceChannelID = 29
	existing, err := store.CreateSubject(context.Background(), change)
	require.ErrorIs(t, err, ErrSubjectExists)
	assert.Equal(t, subject.ID, existing.ID)
	assert.EqualValues(t, 17, existing.SourceChannelID, "重复创建不隐式切换来源")
	change.Version = 1
	change.Name = "Renamed"
	change.Enabled = false
	change.Now += 600
	updated, err := store.UpdateSubject(context.Background(), subject.ID, change)
	require.NoError(t, err)
	assert.EqualValues(t, 2, updated.Version)
	assert.EqualValues(t, 29, updated.SourceChannelID)
	assert.Equal(t, subject.IdentityFingerprint, updated.IdentityFingerprint)
	assert.False(t, updated.Enabled)
	assert.Zero(t, updated.NextDueAt)
	_, err = store.UpdateSubject(context.Background(), subject.ID, change)
	require.ErrorIs(t, err, model.ErrPrivateQuotaConflict)
	change.Version = 2
	change.Enabled = true
	require.NoError(t, db.Model(&model.Channel{}).Where("id = ?", 29).
		Update("key", `{"account_id":"account-B","access_token":"synthetic-rotated"}`).Error)
	_, err = store.UpdateSubject(context.Background(), subject.ID, change)
	require.ErrorIs(t, err, ErrIdentityChanged)
	var stored model.PrivateQuotaSubject
	require.NoError(t, db.First(&stored, subject.ID).Error)
	assert.Equal(t, updated, stored, "换账号失败不能更改旧主体")
	var events []model.PrivateQuotaEvent
	require.NoError(t, db.Where("subject_id = ?", subject.ID).Order("id").Find(&events).Error)
	require.Len(t, events, 2)
	assert.Equal(t, "subject_created", events[0].Kind)
	assert.Equal(t, "subject_updated", events[1].Kind)
	assert.EqualValues(t, 42, events[1].ActorID)
	for _, event := range events {
		assert.NotContains(t, event.Reason, "account-A")
		assert.NotContains(t, event.Reason, "synthetic-access")
	}
	unchangedSettings, err := model.GetPrivateQuotaSettings(db)
	require.NoError(t, err)
	assert.Equal(t, settings, unchangedSettings, "主体操作只锁定设置行，不修改其版本、密钥或租约")

	// 强制事件插入失败，验证创建与更新都不会留下半笔业务变更。
	require.NoError(t, db.Callback().Create().Before("gorm:create").Register("quota_test_reject_audit", func(tx *gorm.DB) {
		if tx.Statement.Schema != nil && tx.Statement.Schema.Table == "private_quota_events" {
			tx.AddError(errors.New("synthetic audit failure"))
		}
	}))
	change.SourceChannelID = 17
	_, err = store.UpdateSubject(context.Background(), subject.ID, change)
	require.ErrorContains(t, err, "synthetic audit failure")
	require.NoError(t, db.First(&stored, subject.ID).Error)
	assert.Equal(t, updated, stored)
	change.SourceChannelID = 29
	_, err = store.CreateSubject(context.Background(), change)
	require.ErrorContains(t, err, "synthetic audit failure")
	var count int64
	require.NoError(t, db.Model(&model.PrivateQuotaSubject{}).Count(&count).Error)
	assert.EqualValues(t, 1, count)
	require.NoError(t, db.Callback().Create().Remove("quota_test_reject_audit"))

	// 两个独立数据库会话同时创建同一账号，恰好一个成功、另一个拿到已有ID。
	start := make(chan struct{})
	var wg sync.WaitGroup
	results := make([]model.PrivateQuotaSubject, 2)
	errs := make([]error, 2)
	for i := range 2 {
		wg.Go(func() {
			<-start
			separate := Store{DB: db.Session(&gorm.Session{NewDB: true})}
			results[i], errs[i] = separate.CreateSubject(context.Background(), change)
		})
	}
	close(start)
	wg.Wait()
	if errs[0] != nil {
		errs[0], errs[1] = errs[1], errs[0]
	}
	require.NoError(t, errs[0])
	require.ErrorIs(t, errs[1], ErrSubjectExists)
	assert.Equal(t, results[0].ID, results[1].ID)
	require.NoError(t, db.Model(&model.PrivateQuotaSubject{}).Count(&count).Error)
	assert.EqualValues(t, 2, count)
	require.NoError(t, db.Model(&model.PrivateQuotaEvent{}).Where("subject_id = ?", results[0].ID).Count(&count).Error)
	assert.EqualValues(t, 1, count, "并发重复绑定只能留下一条创建审计")
	updateChange := change
	updateChange.Version = 1
	start = make(chan struct{})
	for i := range 2 {
		wg.Go(func() {
			<-start
			separate := Store{DB: db.Session(&gorm.Session{NewDB: true})}
			_, errs[i] = separate.UpdateSubject(context.Background(), results[0].ID, updateChange)
		})
	}
	close(start)
	wg.Wait()
	if errs[0] != nil {
		errs[0], errs[1] = errs[1], errs[0]
	}
	require.NoError(t, errs[0])
	require.ErrorIs(t, errs[1], model.ErrPrivateQuotaConflict)
	require.NoError(t, db.Model(&model.PrivateQuotaEvent{}).Where("subject_id = ?", results[0].ID).Count(&count).Error)
	assert.EqualValues(t, 2, count, "同版本并发更新只能增加一条审计")
	// 来源被删后仍可暂停；停采操作不能依赖上游凭据仍然有效。
	pausedID := results[0].ID
	require.NoError(t, db.Delete(&model.Channel{}, "id = ?", 29).Error)
	change.Enabled = false
	change.Version = 2
	paused, err := store.UpdateSubject(context.Background(), pausedID, change)
	require.NoError(t, err)
	assert.False(t, paused.Enabled)
	assert.EqualValues(t, 3, paused.Version)
	change.Enabled = true
	change.Version = 3
	_, err = store.UpdateSubject(context.Background(), pausedID, change)
	require.ErrorIs(t, err, ErrSourceUnavailable)
	for _, bad := range []SubjectChange{
		{Name: "", SourceChannelID: 17, ActorID: 42, Now: 1800000000},
		{Name: strings.Repeat("长", 129), SourceChannelID: 17, ActorID: 42, Now: 1800000000},
		{Name: "bad\nname", SourceChannelID: 17, ActorID: 42, Now: 1800000000},
		{Name: "account", SourceChannelID: 0, ActorID: 42, Now: 1800000000},
		{Name: "account", SourceChannelID: 17, ActorID: 0, Now: 1800000000},
	} {
		_, err := store.CreateSubject(context.Background(), bad)
		require.ErrorIs(t, err, model.ErrPrivateQuotaInputInvalid)
	}
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = store.CreateSubject(cancelled, change)
	require.ErrorIs(t, err, context.Canceled)
	quotaObservationContract(t, db)
}

// 去掉有效租约、主体版本/身份检查或批次事务，会造成迟到写入、串账或重复采样。
func quotaObservationContract(t *testing.T, db *gorm.DB) {
	t.Helper()
	ctx := context.Background()
	const at = int64(1800010000)
	clock := at + 10
	store := Store{DB: db, Clock: func() time.Time { return time.Unix(clock, 0) }}
	settings, err := model.GetPrivateQuotaSettings(db)
	require.NoError(t, err)
	_, err = model.UpdatePrivateQuotaSettings(db, model.PrivateQuotaSettingsChange{
		Enabled: true, IntervalMinutes: 10, Version: settings.Version, ActorID: 42, Now: at})
	require.NoError(t, err)
	require.NoError(t, db.Create(&model.Channel{Id: 41, Type: constant.ChannelTypeCodex,
		Status: common.ChannelStatusEnabled, Name: "synthetic-observation",
		Key: `{"account_id":"account-C","access_token":"synthetic-access"}`}).Error)
	subject, err := store.CreateSubject(ctx, SubjectChange{Name: "Ledger", SourceChannelID: 41, Enabled: true, ActorID: 42, Now: at})
	require.NoError(t, err)
	lease, err := model.ClaimPrivateQuotaLease(db, "observations", at)
	require.NoError(t, err)
	used := 3000
	batch := ObservationBatch{SubjectID: subject.ID, SubjectVersion: subject.Version, SourceChannelID: 41,
		SampleKey: "batch-1", StartedAt: at, FetchedAt: at + 5,
		Windows: []Observation{{WindowSeconds: SevenDaySeconds, UsedBasisPoints: &used, ResetAt: at + 604800},
			{WindowSeconds: FiveHourSeconds, ResetAt: at + 18000}}}
	result, err := store.SaveObservationBatch(ctx, lease, batch)
	require.NoError(t, err)
	assert.False(t, result.Duplicate)
	assert.EqualValues(t, 2, result.Revision)
	var snapshots []model.PrivateQuotaSnapshot
	require.NoError(t, db.Where("subject_id = ?", subject.ID).Order("id").Find(&snapshots).Error)
	require.Len(t, snapshots, 2)
	assert.NotEmpty(t, snapshots[0].SegmentID)
	assert.Equal(t, snapshots[0].SegmentID, snapshots[1].SegmentID)
	assert.Equal(t, &used, snapshots[0].UsedBasisPoints)
	assert.Nil(t, snapshots[1].UsedBasisPoints)
	assert.Equal(t, "missing_percent", snapshots[1].QualityCode)
	assert.EqualValues(t, 600, snapshots[0].IntervalSeconds)
	assert.Equal(t, at+5, snapshots[0].FetchedAt)
	assert.Equal(t, clock, snapshots[0].RecordedAt)
	var stored model.PrivateQuotaSubject
	require.NoError(t, db.First(&stored, subject.ID).Error)
	assert.Equal(t, at, stored.LastAttemptAt)
	assert.Equal(t, at+5, stored.LastSuccessAt)
	assert.Equal(t, at+600, stored.NextDueAt)
	result, err = store.SaveObservationBatch(ctx, lease, batch)
	require.NoError(t, err, "精确重试必须在旧subject version检查之前识别")
	assert.True(t, result.Duplicate)
	assert.EqualValues(t, 2, result.Revision)
	changed := batch
	changed.Windows = append([]Observation(nil), batch.Windows...)
	changed.Windows[0].ResetAt++
	_, err = store.SaveObservationBatch(ctx, lease, changed)
	require.ErrorIs(t, err, model.ErrPrivateQuotaConflict)
	next := batch
	next.SampleKey = "batch-2"
	next.SubjectVersion = 2
	next.StartedAt = at + 20
	next.FetchedAt = at + 25
	clock = at + 30
	// 强制第二个窗口写入失败，第一窗口及主体修订也必须回滚。
	require.NoError(t, db.Callback().Create().Before("gorm:create").Register("quota_test_reject_snapshot", func(tx *gorm.DB) {
		if row, ok := tx.Statement.Dest.(*model.PrivateQuotaSnapshot); ok && row.WindowSeconds == FiveHourSeconds {
			tx.AddError(errors.New("synthetic snapshot failure"))
		}
	}))
	_, err = store.SaveObservationBatch(ctx, lease, next)
	require.ErrorContains(t, err, "synthetic snapshot failure")
	require.NoError(t, db.Callback().Create().Remove("quota_test_reject_snapshot"))
	var count int64
	require.NoError(t, db.Model(&model.PrivateQuotaSnapshot{}).Where("subject_id = ?", subject.ID).Count(&count).Error)
	assert.EqualValues(t, 2, count)
	require.NoError(t, db.First(&stored, subject.ID).Error)
	assert.EqualValues(t, 2, stored.Version)
	result, err = store.SaveObservationBatch(ctx, lease, next)
	require.NoError(t, err)
	assert.EqualValues(t, 3, result.Revision)
	var newest model.PrivateQuotaSnapshot
	require.NoError(t, db.Where("subject_id = ?", subject.ID).Order("id DESC").Take(&newest).Error)
	assert.Equal(t, snapshots[0].SegmentID, newest.SegmentID, "服务端沿用分段，不由调用方指定")
	older := next
	older.SampleKey = "out-of-order"
	older.SubjectVersion = 3
	_, err = store.SaveObservationBatch(ctx, lease, older)
	require.ErrorIs(t, err, model.ErrPrivateQuotaConflict)
	// 全部缺失的响应只保留异常证据，不刷新最后成功时间；并发重试只提交一次。
	invalid := ObservationBatch{SubjectID: subject.ID, SubjectVersion: 3, SourceChannelID: 41,
		SampleKey: "all-missing", StartedAt: at + 31, FetchedAt: at + 32,
		Windows: []Observation{{WindowSeconds: SevenDaySeconds, ResetAt: at + 604800}}}
	clock = at + 35
	start := make(chan struct{})
	var wg sync.WaitGroup
	results := make([]ObservationResult, 2)
	errs := make([]error, 2)
	for i := range 2 {
		wg.Go(func() {
			<-start
			separate := Store{DB: db.Session(&gorm.Session{NewDB: true}), Clock: store.Clock}
			results[i], errs[i] = separate.SaveObservationBatch(ctx, lease, invalid)
		})
	}
	close(start)
	wg.Wait()
	require.NoError(t, errs[0])
	require.NoError(t, errs[1])
	assert.NotEqual(t, results[0].Duplicate, results[1].Duplicate)
	assert.EqualValues(t, 4, results[0].Revision)
	assert.EqualValues(t, 4, results[1].Revision)
	require.NoError(t, db.First(&stored, subject.ID).Error)
	assert.Equal(t, at+25, stored.LastSuccessAt)
	assert.Equal(t, 1, stored.FailureCount)
	assert.Equal(t, "invalid_observation", stored.ErrorCode)
	require.NoError(t, db.Model(&model.PrivateQuotaSnapshot{}).Where("subject_id = ?", subject.ID).Count(&count).Error)
	assert.EqualValues(t, 5, count)
	for _, bad := range []ObservationBatch{
		{SubjectID: subject.ID, SubjectVersion: 4, SourceChannelID: 41, SampleKey: "empty", StartedAt: at, FetchedAt: at + 5},
		{SubjectID: subject.ID, SubjectVersion: 4, SourceChannelID: 41, SampleKey: "bad-window", StartedAt: at, FetchedAt: at + 5,
			Windows: []Observation{{WindowSeconds: 1}}},
		{SubjectID: subject.ID, SubjectVersion: 4, SourceChannelID: 41, SampleKey: "double-window", StartedAt: at, FetchedAt: at + 5,
			Windows: []Observation{{WindowSeconds: SevenDaySeconds}, {WindowSeconds: SevenDaySeconds}}},
		{SubjectID: subject.ID, SubjectVersion: 4, SourceChannelID: 41, SampleKey: "future", StartedAt: at, FetchedAt: at + 99,
			Windows: []Observation{{WindowSeconds: SevenDaySeconds}}},
	} {
		_, err = store.SaveObservationBatch(ctx, lease, bad)
		require.ErrorIs(t, err, model.ErrPrivateQuotaInputInvalid)
	}
	// 模拟等待写锁期间已到期：时钟必须在设置行锁取得后读取。
	require.NoError(t, db.Callback().Update().After("gorm:update").Register("quota_test_lock_delay", func(tx *gorm.DB) {
		if tx.Statement.Table == "private_quota_settings" {
			clock = at + 120
		}
	}))
	_, err = store.SaveObservationBatch(ctx, lease, batch)
	require.ErrorIs(t, err, model.ErrPrivateQuotaLeaseLost)
	require.NoError(t, db.Callback().Update().Remove("quota_test_lock_delay"))
	clock = at + 35
	// 持久化之前重新读渠道，不能只依赖绑定时的校验。
	require.NoError(t, db.Model(&model.Channel{}).Where("id = ?", 41).
		Update("key", `{"account_id":"different-account","access_token":"synthetic"}`).Error)
	_, err = store.SaveObservationBatch(ctx, lease, next)
	require.ErrorIs(t, err, ErrIdentityChanged)
	require.NoError(t, db.Model(&model.Channel{}).Where("id = ?", 41).
		Update("key", `{"account_id":"account-C","access_token":"rotated-synthetic"}`).Error)
	paused, err := store.UpdateSubject(ctx, subject.ID, SubjectChange{Name: "Ledger", SourceChannelID: 41,
		Enabled: false, Version: 4, ActorID: 42, Now: clock})
	require.NoError(t, err)
	_, err = store.SaveObservationBatch(ctx, lease, next)
	require.ErrorIs(t, err, model.ErrPrivateQuotaConflict, "暂停后连幂等重试都不能绕过停采校验")
	_, err = store.UpdateSubject(ctx, subject.ID, SubjectChange{Name: "Ledger", SourceChannelID: 41,
		Enabled: true, Version: paused.Version, ActorID: 42, Now: clock})
	require.NoError(t, err)
	clock = at + 120
	_, err = store.SaveObservationBatch(ctx, lease, batch)
	require.ErrorIs(t, err, model.ErrPrivateQuotaLeaseLost)
	newLease, err := model.ClaimPrivateQuotaLease(db, lease.Owner, clock)
	require.NoError(t, err)
	_, err = store.SaveObservationBatch(ctx, lease, batch)
	require.ErrorIs(t, err, model.ErrPrivateQuotaLeaseLost)
	_, err = model.UpdatePrivateQuotaSettings(db, model.PrivateQuotaSettingsChange{Enabled: false,
		IntervalMinutes: 10, Version: newLease.SettingsVersion, ActorID: 42, Now: clock})
	require.NoError(t, err)
	_, err = store.SaveObservationBatch(ctx, newLease, batch)
	require.ErrorIs(t, err, model.ErrPrivateQuotaLeaseLost)
	require.NoError(t, model.ReleasePrivateQuotaLease(db, newLease))
	quotaEventStoreContract(t, db)
}

// 丢失原子重放、决定版本或替代链检查会造成同一轮额度重复计数。
func quotaEventStoreContract(t *testing.T, db *gorm.DB) {
	t.Helper()
	ctx := context.Background()
	const at = int64(1800020000)
	clock := at
	store := Store{DB: db, Clock: func() time.Time { return time.Unix(clock, 0) }}
	settings, err := model.GetPrivateQuotaSettings(db)
	require.NoError(t, err)
	_, err = model.UpdatePrivateQuotaSettings(db, model.PrivateQuotaSettingsChange{Enabled: true, IntervalMinutes: 10,
		Version: settings.Version, ActorID: 42, Now: at})
	require.NoError(t, err)
	require.NoError(t, db.Create(&model.Channel{Id: 43, Type: constant.ChannelTypeCodex, Status: common.ChannelStatusEnabled,
		Name: "synthetic-events", Key: `{"account_id":"account-events","access_token":"synthetic"}`}).Error)
	subject, err := store.CreateSubject(ctx, SubjectChange{Name: "Events", SourceChannelID: 43, Enabled: true, ActorID: 42, Now: at})
	require.NoError(t, err)
	for i, percent := range []int{7000, 1000, 1200} {
		clock = at + int64(i)*600
		lease, err := model.ClaimPrivateQuotaLease(db, "events", clock)
		require.NoError(t, err)
		reset := at + 604800
		if i > 0 {
			reset += 600
		}
		_, err = store.SaveObservationBatch(ctx, lease, ObservationBatch{SubjectID: subject.ID, SubjectVersion: int64(i + 1), SourceChannelID: 43,
			SampleKey: fmt.Sprintf("events-%d", i), StartedAt: clock, FetchedAt: clock,
			Windows: []Observation{{WindowSeconds: SevenDaySeconds, UsedBasisPoints: &percent, ResetAt: reset}}})
		require.NoError(t, err)
		require.NoError(t, model.ReleasePrivateQuotaLease(db, lease))
	}
	ledger, revision, err := store.ReadLedger(ctx, subject.ID)
	require.NoError(t, err)
	require.Len(t, ledger.Events, 1)
	assert.EqualValues(t, 4, revision)
	assert.Equal(t, EventAccepted, ledger.Events[0].Status)
	var event model.PrivateQuotaEvent
	require.NoError(t, db.Where("subject_id = ? AND window_seconds > 0 AND kind <> ?", subject.ID, "decision").Take(&event).Error)
	assert.Equal(t, "early_reset", event.Kind)
	assert.Equal(t, "accepted", event.Status)
	assert.EqualValues(t, 2, event.Version, "pending到accepted必须使旧预览失效")
	change := DecisionChange{SubjectVersion: revision, EventVersion: event.Version, Kind: PlanCorrection, Reason: "确认只是口径校正", ActorID: 42}
	result, err := store.DecideEvent(ctx, event.ID, change)
	require.NoError(t, err)
	assert.EqualValues(t, 5, result.Revision)
	ledger, _, err = store.ReadLedger(ctx, subject.ID)
	require.NoError(t, err)
	require.Len(t, ledger.Cycles, 1)
	assert.Empty(t, ledger.Deltas)
	_, err = store.DecideEvent(ctx, event.ID, change)
	require.ErrorIs(t, err, model.ErrPrivateQuotaConflict)
	require.NoError(t, db.First(&event, event.ID).Error)
	change.SubjectVersion, change.EventVersion = 5, event.Version
	change.Kind, change.Reason = ConfirmReset, "再次核实为重置"
	// 决定插入后投影写入失败，决定和主体版本也必须一起回滚。
	require.NoError(t, db.Callback().Update().Before("gorm:update").Register("quota_event_reject", func(tx *gorm.DB) {
		if tx.Statement.Table == "private_quota_events" {
			tx.AddError(errors.New("synthetic projection failure"))
		}
	}))
	_, err = store.DecideEvent(ctx, event.ID, change)
	require.ErrorContains(t, err, "synthetic projection failure")
	require.NoError(t, db.Callback().Update().Remove("quota_event_reject"))
	var decisions int64
	require.NoError(t, db.Model(&model.PrivateQuotaEvent{}).Where("subject_id = ? AND kind = ?", subject.ID, "decision").Count(&decisions).Error)
	assert.EqualValues(t, 1, decisions)
	result, err = store.DecideEvent(ctx, event.ID, change)
	require.NoError(t, err)
	assert.EqualValues(t, 6, result.Revision)
	ledger, _, err = store.ReadLedger(ctx, subject.ID)
	require.NoError(t, err)
	require.Len(t, ledger.Cycles, 2)
	assert.Equal(t, ResetManual, ledger.Events[0].Kind)
	assert.EqualValues(t, 1200, ledger.Cycles[1].ObservedBasisPoints)
	var rows []model.PrivateQuotaEvent
	require.NoError(t, db.Where("subject_id = ? AND kind = ?", subject.ID, "decision").Order("id").Find(&rows).Error)
	require.Len(t, rows, 2)
	assert.Equal(t, rows[0].ID, rows[1].SupersedesEventID)
	assert.Equal(t, "确认只是口径校正", rows[0].Reason)
}

func TestSubjectStoreIsolatedSQLDatabases(t *testing.T) {
	if os.Getenv("PRIVATE_QUOTA_ISOLATED_TEST_DB") != "1" {
		t.Skip("需要显式隔离数据库授权，不读取SQL_DSN")
	}
	for _, dialect := range []string{"mysql", "postgres"} {
		t.Run(dialect, func(t *testing.T) {
			var driver gorm.Dialector
			if dialect == "mysql" {
				dsn := os.Getenv("PRIVATE_QUOTA_TEST_MYSQL_DSN")
				if dsn == "" {
					t.Skip("未配置隔离MySQL，不视为通过")
				}
				driver = mysql.Open(dsn)
			} else {
				dsn := os.Getenv("PRIVATE_QUOTA_TEST_POSTGRES_DSN")
				if dsn == "" {
					t.Skip("未配置隔离PostgreSQL，不视为通过")
				}
				driver = postgres.Open(dsn)
			}
			db, err := gorm.Open(driver, &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
			require.NoError(t, err)
			sqlDB, err := db.DB()
			require.NoError(t, err)
			t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
			query := "SELECT DATABASE()"
			if dialect == "postgres" {
				query = "SELECT current_database()"
			}
			var database string
			require.NoError(t, db.Raw(query).Scan(&database).Error)
			require.True(t, strings.HasPrefix(database, "new_api_private_quota_test_"))
			tables, err := db.Migrator().GetTables()
			require.NoError(t, err)
			require.Empty(t, tables, "只允许空的隔离库，不删旧表")
			var version string
			require.NoError(t, db.Raw("SELECT version()").Scan(&version).Error)
			t.Logf("实际引擎版本：%s", version)
			quotaSubjectStoreContract(t, db)
		})
	}
}

// 若将token或渠道ID混入身份，轮换或同账号多渠道就会错误裂账。
func TestChannelFingerprintKeepsAccountIdentityAcrossCredentialRotation(t *testing.T) {
	const key = "000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f"
	channel := model.Channel{Id: 17, Type: constant.ChannelTypeCodex, Status: common.ChannelStatusEnabled,
		Key: `{"account_id":"account-A","access_token":"old-access","refresh_token":"old-refresh"}`}
	fingerprint, err := ChannelFingerprint(&channel, key)
	require.NoError(t, err)
	// 使用独立OpenSSL HMAC计算的固定向量，不调用被测函数生成期望。
	assert.Equal(t, "2c0c11e00df685832bc833b8ba3a7ae62954e01fbc9bb4a1ff4cc4fdc0a9b885", fingerprint)
	for _, source := range []string{"", "https://chatgpt.com/", "HTTPS://CHATGPT.COM:443///"} {
		rotated := channel
		rotated.Id = 29
		rotated.BaseURL = &source
		rotated.Key = `{"account_id":" account-A ","access_token":"new-access","refresh_token":"new-refresh","email":"changed@example.invalid"}`
		got, err := ChannelFingerprint(&rotated, key)
		require.NoError(t, err)
		assert.Equal(t, fingerprint, got)
	}
	for _, tc := range []struct{ source, account string }{
		{"https://chatgpt.com", "account-B"},
		{"https://proxy.example.invalid", "account-A"},
		{"https://proxy.example.invalid/path", "account-A"},
	} {
		other := channel
		other.BaseURL = &tc.source
		body, err := common.Marshal(map[string]string{"account_id": tc.account, "access_token": "other-access"})
		require.NoError(t, err)
		other.Key = string(body)
		got, err := ChannelFingerprint(&other, key)
		require.NoError(t, err)
		assert.NotEqual(t, fingerprint, got)
	}
	assert.EqualValues(t, 17, channel.Id)
	assert.Contains(t, channel.Key, "old-access", "解析不得修改渠道或凭据")
}

func TestChannelFingerprintRejectsUnusableSourcesWithoutLeakingCredentials(t *testing.T) {
	const key = "000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f"
	for _, tc := range []struct {
		name string
		edit func(*model.Channel)
	}{
		{"非Codex", func(c *model.Channel) { c.Type = constant.ChannelTypeOpenAI }},
		{"停用", func(c *model.Channel) { c.Status = 2 }},
		{"无渠道ID", func(c *model.Channel) { c.Id = 0 }},
		{"多密钥", func(c *model.Channel) { c.ChannelInfo.IsMultiKey = true }},
		{"坏JSON", func(c *model.Channel) { c.Key = "secret-access-INVALID" }},
		{"无账号", func(c *model.Channel) { c.Key = `{"access_token":"secret-access"}` }},
		{"无AccessToken", func(c *model.Channel) { c.Key = `{"account_id":"secret-account"}` }},
		{"多KeyJSON", func(c *model.Channel) { c.Key = `[{"account_id":"secret-account","access_token":"secret-access"}]` }},
		{"相对URL", func(c *model.Channel) { s := "/relative"; c.BaseURL = &s }},
		{"非HTTP", func(c *model.Channel) { s := "file:///secret-account"; c.BaseURL = &s }},
		{"URL密码", func(c *model.Channel) { s := "https://secret-access@proxy.invalid"; c.BaseURL = &s }},
		{"URL查询", func(c *model.Channel) { s := "https://proxy.invalid?key=secret-access"; c.BaseURL = &s }},
		{"URL片段", func(c *model.Channel) { s := "https://proxy.invalid/#secret-access"; c.BaseURL = &s }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			channel := model.Channel{Id: 17, Type: constant.ChannelTypeCodex, Status: common.ChannelStatusEnabled,
				Key: `{"account_id":"secret-account","access_token":"secret-access"}`}
			tc.edit(&channel)
			got, err := ChannelFingerprint(&channel, key)
			require.ErrorIs(t, err, ErrSourceUnavailable)
			assert.Empty(t, got)
			assert.NotContains(t, err.Error(), "secret-")
		})
	}
	_, err := ChannelFingerprint(nil, key)
	require.ErrorIs(t, err, ErrSourceUnavailable)
	channel := model.Channel{Id: 17, Type: constant.ChannelTypeCodex, Status: common.ChannelStatusEnabled,
		Key: `{"account_id":"account-A","access_token":"access"}`}
	for _, bad := range []string{"", "not-hex", strings.Repeat("00", 31), strings.Repeat("00", 33)} {
		_, err := ChannelFingerprint(&channel, bad)
		require.ErrorIs(t, err, model.ErrPrivateQuotaStateInvalid)
	}
}

func TestValidateSubjectSourceRejectsChangedAccountAndKeepsHistory(t *testing.T) {
	const key = "000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f"
	subject := model.PrivateQuotaSubject{ID: 1, Provider: "codex", SourceChannelID: 17, Version: 3,
		IdentityFingerprint: "2c0c11e00df685832bc833b8ba3a7ae62954e01fbc9bb4a1ff4cc4fdc0a9b885"}
	channel := model.Channel{Id: 29, Type: constant.ChannelTypeCodex, Status: common.ChannelStatusEnabled,
		Key: `{"account_id":"account-A","access_token":"rotated"}`}
	require.NoError(t, ValidateSubjectSource(subject, &channel, key), "同身份允许显式换来源渠道")
	channel.Key = `{"account_id":"account-B","access_token":"other"}`
	require.ErrorIs(t, ValidateSubjectSource(subject, &channel, key), ErrIdentityChanged)
	subject.Provider = "other"
	channel.Key = `{"account_id":"account-A","access_token":"rotated"}`
	require.ErrorIs(t, ValidateSubjectSource(subject, &channel, key), ErrIdentityChanged)
	assert.EqualValues(t, 17, subject.SourceChannelID)
	assert.EqualValues(t, 3, subject.Version)
}

// quotaSample 使用可手算的基点和采样时间，避免真实时钟或上游账号参与规则测试。
func quotaSample(id, at int64, used int, reset int64) Sample {
	return Sample{ID: id, SubjectID: 1, BatchID: "batch-" + time.Unix(id, 0).UTC().Format(time.RFC3339),
		WindowSeconds: 604800, SegmentID: "plus-v1", FetchedAt: at,
		UsedBasisPoints: &used, ResetAt: reset, IntervalSeconds: 600}
}

func TestBuildLedgerBaselineAndHighWater(t *testing.T) {
	const at = int64(1800000000)
	for _, tc := range []struct {
		name string
		used []int
		want int64
	}{
		{"接入前30%不计新增", []int{3000}, 0},
		{"微小回落不重复记增长", []int{3000, 3100, 3000, 3200}, 200},
		{"显式0到100计一整单位", []int{0, 10000}, 10000},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var samples []Sample
			for i, used := range tc.used {
				samples = append(samples, quotaSample(int64(i+1), at+int64(i)*600, used, at+604800))
			}
			ledger, err := BuildLedger(samples, nil)
			require.NoError(t, err)
			require.Len(t, ledger.Cycles, 1)
			assert.Equal(t, tc.want, ledger.Cycles[0].ObservedBasisPoints)
			assert.True(t, ledger.Cycles[0].Partial)
			assert.Empty(t, ledger.Events)
		})
	}
}

func TestBuildLedgerSortsAndDeduplicatesWithoutMutatingInput(t *testing.T) {
	const at = int64(1800000000)
	a := quotaSample(1, at, 3000, at+604800)
	b := quotaSample(2, at+600, 4000, at+604800)
	samples := []Sample{b, a, a}
	ledger, err := BuildLedger(samples, nil)
	require.NoError(t, err)
	require.Len(t, ledger.Cycles, 1)
	assert.EqualValues(t, 1000, ledger.Cycles[0].ObservedBasisPoints)
	assert.EqualValues(t, 2, samples[0].ID)
	assert.EqualValues(t, 1, samples[1].ID)
	assert.EqualValues(t, 1, samples[2].ID)
}

func TestBuildLedgerRejectsInvalidQuotaInsteadOfInventingZero(t *testing.T) {
	const at = int64(1800000000)
	for _, tc := range []struct {
		name string
		edit func(*Sample)
	}{
		{"缺失百分比", func(s *Sample) { s.UsedBasisPoints = nil }},
		{"负百分比", func(s *Sample) { n := -1; s.UsedBasisPoints = &n }},
		{"超过100%", func(s *Sample) { n := 10001; s.UsedBasisPoints = &n }},
		{"缺失重置时间", func(s *Sample) { s.ResetAt = 0 }},
		{"已过期重置时间", func(s *Sample) { s.ResetAt = s.FetchedAt }},
		{"未知窗口", func(s *Sample) { s.WindowSeconds = 3600 }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sample := quotaSample(1, at, 0, at+604800)
			tc.edit(&sample)
			ledger, err := BuildLedger([]Sample{sample}, nil)
			require.NoError(t, err)
			assert.Empty(t, ledger.Cycles)
			assert.Len(t, ledger.Rejected, 1)
		})
	}
}

func TestBuildLedgerConfirmsEarlyResetWithTwoFreshSamples(t *testing.T) {
	const at = int64(1800000000)
	samples := []Sample{
		quotaSample(1, at, 3000, at+604800),
		quotaSample(2, at+600, 4000, at+604800),
		quotaSample(3, at+1200, 200, at+605400),
	}
	ledger, err := BuildLedger(samples, nil)
	require.NoError(t, err)
	require.Len(t, ledger.Events, 1)
	assert.Equal(t, EventPending, ledger.Events[0].Status)
	require.Len(t, ledger.Cycles, 1)
	assert.EqualValues(t, 1000, ledger.Cycles[0].ObservedBasisPoints)

	samples = append(samples, quotaSample(4, at+1800, 500, at+605400))
	ledger, err = BuildLedger(samples, nil)
	require.NoError(t, err)
	require.Len(t, ledger.Events, 1)
	assert.Equal(t, EventAccepted, ledger.Events[0].Status)
	assert.Equal(t, ResetEarly, ledger.Events[0].Kind)
	assert.EqualValues(t, at+600, ledger.Events[0].ObservedFrom)
	assert.EqualValues(t, at+1200, ledger.Events[0].ObservedTo)
	assert.EqualValues(t, at+1800, ledger.Events[0].ConfirmedAt)
	require.Len(t, ledger.Cycles, 2)
	assert.EqualValues(t, 1000, ledger.Cycles[0].ObservedBasisPoints)
	assert.EqualValues(t, 500, ledger.Cycles[1].ObservedBasisPoints)
	assert.False(t, ledger.Cycles[1].Partial)
}

func TestBuildLedgerDoesNotCallLateObservationAnEarlyReset(t *testing.T) {
	const at = int64(1800000000)
	ledger, err := BuildLedger([]Sample{
		quotaSample(1, at, 8000, at+900),
		quotaSample(2, at+1200, 0, at+605700),
		quotaSample(3, at+1800, 300, at+605700),
	}, nil)
	require.NoError(t, err)
	require.Len(t, ledger.Events, 1)
	assert.Equal(t, ResetExpired, ledger.Events[0].Kind)
	assert.Equal(t, EventAccepted, ledger.Events[0].Status)
	require.Len(t, ledger.Cycles, 2)
	assert.EqualValues(t, 300, ledger.Cycles[1].ObservedBasisPoints)
}

func TestBuildLedgerKeepsAmbiguousChangesPending(t *testing.T) {
	const at = int64(1800000000)
	for _, tc := range []struct {
		name  string
		used  int
		reset int64
	}{
		{"只变时间不能发一份额度", 4100, at + 605400},
		{"只回落不能确认重置", 100, at + 604800},
		{"向前缩短计划不能假定新轮", 100, at + 604000},
		{"截止附近无法证明提前", 100, at + 605400},
	} {
		t.Run(tc.name, func(t *testing.T) {
			first := quotaSample(1, at, 4000, at+604800)
			if tc.name == "截止附近无法证明提前" {
				first.ResetAt = at + 630
			}
			ledger, err := BuildLedger([]Sample{first,
				quotaSample(2, at+600, tc.used, tc.reset),
				quotaSample(3, at+1200, tc.used+100, tc.reset),
			}, nil)
			require.NoError(t, err)
			require.Len(t, ledger.Cycles, 1)
			assert.Zero(t, ledger.Cycles[0].ObservedBasisPoints)
			require.Len(t, ledger.Events, 1)
			assert.Equal(t, EventPending, ledger.Events[0].Status)
		})
	}
}

func TestSummarizeMonthUsesShanghaiBoundaryAndKeepsWindowsSeparate(t *testing.T) {
	location, err := time.LoadLocation("Asia/Shanghai")
	require.NoError(t, err)
	before := time.Date(2026, 10, 31, 15, 55, 0, 0, time.UTC).Unix()
	a := quotaSample(1, before, 3000, before+604800)
	b := quotaSample(2, before+600, 3500, before+604800)
	c := quotaSample(3, before+600, 0, before+18000)
	c.WindowSeconds = 18000
	d := quotaSample(4, before+1200, 9000, before+18000)
	d.WindowSeconds = 18000
	ledger, err := BuildLedger([]Sample{a, b, c, d}, nil)
	require.NoError(t, err)
	october, err := SummarizeMonth(ledger, "2026-10", location)
	require.NoError(t, err)
	assert.Zero(t, october.SevenDay.ObservedBasisPoints)
	november, err := SummarizeMonth(ledger, "2026-11", location)
	require.NoError(t, err)
	assert.EqualValues(t, 500, november.SevenDay.ObservedBasisPoints)
	assert.EqualValues(t, 9000, november.FiveHour.ObservedBasisPoints)
	assert.Zero(t, november.SevenDay.UnusedBasisPoints)
	assert.Equal(t, "Asia/Shanghai", november.Timezone)
}

func TestBuildLedgerManualDecisionReplaysAndSupersedesWithoutDoubleCounting(t *testing.T) {
	const at = int64(1800000000)
	samples := []Sample{quotaSample(1, at, 4000, at+604800),
		quotaSample(2, at+600, 4100, at+605400), quotaSample(3, at+1200, 4200, at+605400)}
	initial, err := BuildLedger(samples, nil)
	require.NoError(t, err)
	require.Len(t, initial.Events, 1)
	key := initial.Events[0].Key
	correction := Decision{ID: 1, EventKey: key, Kind: PlanCorrection, Reason: "确认只是计划校正"}
	corrected, err := BuildLedger(samples, []Decision{correction})
	require.NoError(t, err)
	require.Len(t, corrected.Cycles, 1)
	assert.EqualValues(t, 200, corrected.Cycles[0].ObservedBasisPoints)
	assert.True(t, corrected.Cycles[0].HasCorrection)
	assert.Equal(t, EventCorrected, corrected.Events[0].Status)

	replacement := Decision{ID: 2, EventKey: key, Kind: ConfirmReset, Reason: "人工确认发生重置", SupersedesID: 1}
	reset, err := BuildLedger(samples, []Decision{replacement, correction})
	require.NoError(t, err)
	require.Len(t, reset.Cycles, 2)
	assert.EqualValues(t, 4200, reset.Cycles[1].ObservedBasisPoints)
	assert.Equal(t, ResetManual, reset.Events[0].Kind)
	assert.EqualValues(t, 2, reset.Events[0].DecisionID)
	again, err := BuildLedger(samples, []Decision{correction, replacement})
	require.NoError(t, err)
	assert.Equal(t, reset, again)
}

func TestBuildLedgerPlanCorrectionDoesNotRepeatedlyFlagOldHighWater(t *testing.T) {
	const at = int64(1800000000)
	samples := []Sample{quotaSample(1, at, 7000, at+604800),
		quotaSample(2, at+600, 1000, at+604800), quotaSample(3, at+1200, 1200, at+604800),
		quotaSample(4, at+1800, 7100, at+604800)}
	initial, err := BuildLedger(samples, nil)
	require.NoError(t, err)
	require.Len(t, initial.Events, 1)
	ledger, err := BuildLedger(samples, []Decision{{ID: 1, EventKey: initial.Events[0].Key,
		Kind: PlanCorrection, Reason: "口径下调，不是重置"}})
	require.NoError(t, err)
	assert.Len(t, ledger.Events, 1)
	require.Len(t, ledger.Cycles, 1)
	assert.EqualValues(t, 100, ledger.Cycles[0].ObservedBasisPoints)
}

func TestBuildLedgerNewSegmentStartsNewBaseline(t *testing.T) {
	const at = int64(1800000000)
	a := quotaSample(1, at, 4000, at+604800)
	b := quotaSample(2, at+600, 1000, at+604800)
	b.SegmentID = "pro-v2"
	c := quotaSample(3, at+1200, 2000, at+604800)
	c.SegmentID = "pro-v2"
	ledger, err := BuildLedger([]Sample{a, b, c}, nil)
	require.NoError(t, err)
	require.Len(t, ledger.Cycles, 2)
	assert.EqualValues(t, 1000, ledger.Cycles[1].ObservedBasisPoints)
	assert.True(t, ledger.Cycles[1].Partial)
	assert.Equal(t, ChangeSegment, ledger.Cycles[0].EndedBy)
}

func TestBuildLedgerInterruptedEvidenceCannotAutomaticallyConfirmReset(t *testing.T) {
	const at = int64(1800000000)
	for _, name := range []string{"断采", "无效样本", "再次计划变化"} {
		t.Run(name, func(t *testing.T) {
			samples := []Sample{quotaSample(1, at, 4000, at+604800)}
			switch name {
			case "断采":
				samples = append(samples, quotaSample(2, at+3600, 100, at+605400), quotaSample(3, at+4200, 200, at+605400))
			case "无效样本":
				invalid := quotaSample(3, at+1200, 0, at+605400)
				invalid.UsedBasisPoints = nil
				samples = append(samples, quotaSample(2, at+600, 100, at+605400), invalid, quotaSample(4, at+1800, 200, at+605400))
			case "再次计划变化":
				samples = append(samples, quotaSample(2, at+600, 100, at+605400), quotaSample(3, at+1200, 0, at+606000))
			}
			ledger, err := BuildLedger(samples, nil)
			require.NoError(t, err)
			require.Len(t, ledger.Events, 1)
			assert.Equal(t, EventPending, ledger.Events[0].Status)
			assert.Len(t, ledger.Cycles, 1)
		})
	}
}

func TestBuildLedgerSameTimestampCannotSupplyTwoFreshConfirmations(t *testing.T) {
	const at = int64(1800000000)
	ledger, err := BuildLedger([]Sample{quotaSample(1, at, 4000, at+604800),
		quotaSample(2, at+600, 0, at+605400), quotaSample(3, at+600, 100, at+605400)}, nil)
	require.NoError(t, err)
	require.Len(t, ledger.Events, 1)
	assert.Equal(t, EventPending, ledger.Events[0].Status)
	assert.Len(t, ledger.Rejected, 1)
}

func TestSummarizeMonthOnlyCountsUnusedForEvaluableClosedCycles(t *testing.T) {
	location, err := time.LoadLocation("Asia/Shanghai")
	require.NoError(t, err)
	at := time.Date(2026, 10, 5, 0, 0, 0, 0, time.UTC).Unix()
	ledger, err := BuildLedger([]Sample{quotaSample(1, at, 5000, at+604800),
		quotaSample(2, at+600, 0, at+605400), quotaSample(3, at+1200, 0, at+605400),
		quotaSample(4, at+1800, 3000, at+605400), quotaSample(5, at+2400, 100, at+606000),
		quotaSample(6, at+3000, 200, at+606000)}, nil)
	require.NoError(t, err)
	summary, err := SummarizeMonth(ledger, "2026-10", location)
	require.NoError(t, err)
	assert.EqualValues(t, 3200, summary.SevenDay.ObservedBasisPoints)
	assert.EqualValues(t, 7000, summary.SevenDay.UnusedBasisPoints)
	assert.Equal(t, 2, summary.SevenDay.ResetCount)
	assert.Equal(t, 2, summary.SevenDay.EarlyResets)
	assert.Equal(t, 1, summary.SevenDay.EvaluatedCycles)
	assert.Equal(t, 1, summary.SevenDay.ExcludedCycles)
	require.NotNil(t, summary.SevenDay.Current)
	assert.Equal(t, 200, summary.SevenDay.Current.HighWaterBasisPoints)
}

func TestSummarizeMonthUnionsWindowGapsAndRespectsSampleInterval(t *testing.T) {
	location, err := time.LoadLocation("Asia/Shanghai")
	require.NoError(t, err)
	at := time.Date(2026, 10, 5, 0, 0, 0, 0, time.UTC).Unix()
	a, b := quotaSample(1, at, 3000, at+604800), quotaSample(2, at+3600, 4000, at+604800)
	c, d := quotaSample(3, at, 3000, at+18000), quotaSample(4, at+3600, 4000, at+18000)
	c.WindowSeconds, d.WindowSeconds = 18000, 18000
	ledger, err := BuildLedger([]Sample{a, b, c, d}, nil)
	require.NoError(t, err)
	summary, err := SummarizeMonth(ledger, "2026-10", location)
	require.NoError(t, err)
	assert.EqualValues(t, 3600, summary.GapSeconds)
	assert.True(t, ledger.Cycles[0].HasGap)

	a.IntervalSeconds, b.IntervalSeconds = 1800, 1800
	ledger, err = BuildLedger([]Sample{a, b}, nil)
	require.NoError(t, err)
	assert.Empty(t, ledger.Gaps)
}

func TestBuildLedgerRejectsConflictingBatchesAndMixedSubjects(t *testing.T) {
	const at = int64(1800000000)
	a := quotaSample(1, at, 3000, at+604800)
	for _, tc := range []struct {
		name string
		edit func(*Sample)
	}{
		{"同批次不同值", func(s *Sample) { n := 3500; s.UsedBasisPoints = &n }},
		{"混入其他主体", func(s *Sample) { s.SubjectID = 2 }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b := a
			tc.edit(&b)
			_, err := BuildLedger([]Sample{a, b}, nil)
			require.Error(t, err)
		})
	}
}

func TestBuildLedgerRejectsInvalidOrForkedDecisions(t *testing.T) {
	const at = int64(1800000000)
	samples := []Sample{quotaSample(1, at, 4000, at+604800), quotaSample(2, at+600, 4100, at+605400)}
	initial, err := BuildLedger(samples, nil)
	require.NoError(t, err)
	require.Len(t, initial.Events, 1)
	first := Decision{ID: 1, EventKey: initial.Events[0].Key, Kind: ConfirmReset, Reason: "确认重置"}
	for _, tc := range []struct {
		name      string
		decisions []Decision
	}{
		{"缺少理由", []Decision{{ID: 1, EventKey: first.EventKey, Kind: ConfirmReset}}},
		{"未知事件", []Decision{{ID: 1, EventKey: "missing", Kind: ConfirmReset, Reason: "无效引用"}}},
		{"替代分叉", []Decision{first,
			{ID: 2, EventKey: first.EventKey, Kind: PlanCorrection, Reason: "校正", SupersedesID: 1},
			{ID: 3, EventKey: first.EventKey, Kind: SegmentChange, Reason: "分段", SupersedesID: 1}}},
		{"无替代关系重复决定", []Decision{first,
			{ID: 2, EventKey: first.EventKey, Kind: PlanCorrection, Reason: "校正"}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := BuildLedger(samples, tc.decisions)
			require.Error(t, err)
		})
	}
}

func TestSummarizeMonthRejectsInvalidMonthAndTimezone(t *testing.T) {
	location, err := time.LoadLocation("Asia/Shanghai")
	require.NoError(t, err)
	for _, month := range []string{"", "2026-1", "2026-13", "2026-10-01"} {
		_, err := SummarizeMonth(Ledger{}, month, location)
		require.Error(t, err, month)
	}
	_, err = SummarizeMonth(Ledger{}, "2026-10", nil)
	require.Error(t, err)
	_, err = SummarizeMonth(Ledger{}, "2026-10", time.UTC)
	require.Error(t, err)
}

func TestBuildLedgerEmptyInput(t *testing.T) {
	ledger, err := BuildLedger(nil, nil)
	require.NoError(t, err)
	assert.Equal(t, RulesVersion, ledger.RulesVersion)
	assert.Zero(t, ledger.CoverageStart)
	assert.Empty(t, ledger.Cycles)
	assert.Empty(t, ledger.Events)
}

func TestBuildLedgerZeroUsageCanConfirmExpiredWindow(t *testing.T) {
	const at = int64(1800000000)
	ledger, err := BuildLedger([]Sample{quotaSample(1, at, 0, at+900),
		quotaSample(2, at+1200, 0, at+605700), quotaSample(3, at+1800, 0, at+605700)}, nil)
	require.NoError(t, err)
	require.Len(t, ledger.Events, 1)
	assert.Equal(t, ResetExpired, ledger.Events[0].Kind)
	assert.Equal(t, EventAccepted, ledger.Events[0].Status)
	require.Len(t, ledger.Cycles, 2)
	assert.Zero(t, ledger.Cycles[1].ObservedBasisPoints)
}

func TestBuildLedgerSmallPlanDriftDoesNotCreateReset(t *testing.T) {
	const at = int64(1800000000)
	ledger, err := BuildLedger([]Sample{quotaSample(1, at, 3000, at+604800),
		quotaSample(2, at+600, 3100, at+604860), quotaSample(3, at+1200, 3200, at+604800)}, nil)
	require.NoError(t, err)
	assert.Empty(t, ledger.Events)
	require.Len(t, ledger.Cycles, 1)
	assert.EqualValues(t, 200, ledger.Cycles[0].ObservedBasisPoints)
}

func TestBuildLedgerLongOutageDoesNotInventMissedResets(t *testing.T) {
	const at = int64(1800000000)
	const later = at + 30*86400
	ledger, err := BuildLedger([]Sample{quotaSample(1, at, 8000, at+604800),
		quotaSample(2, later, 1000, later+604800), quotaSample(3, later+600, 1100, later+604800)}, nil)
	require.NoError(t, err)
	require.Len(t, ledger.Events, 1)
	assert.Equal(t, EventPending, ledger.Events[0].Status)
	assert.Len(t, ledger.Cycles, 1)
	assert.Len(t, ledger.Gaps, 1)
	assert.Empty(t, ledger.Deltas)
}

func TestBuildLedgerManualSegmentChangeStartsBaseline(t *testing.T) {
	const at = int64(1800000000)
	samples := []Sample{quotaSample(1, at, 7000, at+604800),
		quotaSample(2, at+600, 1000, at+604800), quotaSample(3, at+1200, 1200, at+604800)}
	initial, err := BuildLedger(samples, nil)
	require.NoError(t, err)
	require.Len(t, initial.Events, 1)
	ledger, err := BuildLedger(samples, []Decision{{ID: 1, EventKey: initial.Events[0].Key,
		Kind: SegmentChange, Reason: "人工确认订阅口径已改变"}})
	require.NoError(t, err)
	require.Len(t, ledger.Cycles, 2)
	assert.Equal(t, ChangeSegment, ledger.Cycles[0].EndedBy)
	assert.True(t, ledger.Cycles[1].Partial)
	assert.EqualValues(t, 200, ledger.Cycles[1].ObservedBasisPoints)
	assert.Equal(t, ChangeSegment, ledger.Events[0].Kind)
}

func TestBuildLedgerCanCorrectPreviouslyAcceptedReset(t *testing.T) {
	const at = int64(1800000000)
	samples := []Sample{quotaSample(1, at, 7000, at+604800),
		quotaSample(2, at+600, 1000, at+605400), quotaSample(3, at+1200, 1200, at+605400)}
	initial, err := BuildLedger(samples, nil)
	require.NoError(t, err)
	require.Len(t, initial.Events, 1)
	assert.Equal(t, EventAccepted, initial.Events[0].Status)
	ledger, err := BuildLedger(samples, []Decision{{ID: 1, EventKey: initial.Events[0].Key,
		Kind: PlanCorrection, Reason: "重审证据，确认只是口径下调"}})
	require.NoError(t, err)
	require.Len(t, ledger.Cycles, 1)
	assert.Empty(t, ledger.Deltas)
	assert.Equal(t, EventCorrected, ledger.Events[0].Status)
}

func TestBuildLedgerInvalidEvidenceBeforeBoundaryPreventsAutomaticReset(t *testing.T) {
	const at = int64(1800000000)
	invalid := quotaSample(2, at+600, 0, at+604800)
	invalid.UsedBasisPoints = nil
	ledger, err := BuildLedger([]Sample{quotaSample(1, at, 7000, at+604800), invalid,
		quotaSample(3, at+1200, 1000, at+605400), quotaSample(4, at+1800, 1200, at+605400)}, nil)
	require.NoError(t, err)
	require.Len(t, ledger.Events, 1)
	assert.Equal(t, EventPending, ledger.Events[0].Status)
	assert.Len(t, ledger.Cycles, 1)
	assert.Len(t, ledger.Rejected, 1)
}

func TestSummarizeMonthClipsGapsAndReportsPending(t *testing.T) {
	location, err := time.LoadLocation("Asia/Shanghai")
	require.NoError(t, err)
	start := time.Date(2026, 11, 1, 0, 0, 0, 0, location).Unix()
	ledger, err := BuildLedger([]Sample{quotaSample(1, start-1800, 7000, start+603000),
		quotaSample(2, start+1800, 1000, start+604800), quotaSample(3, start+2400, 1200, start+604800)}, nil)
	require.NoError(t, err)
	october, err := SummarizeMonth(ledger, "2026-10", location)
	require.NoError(t, err)
	assert.EqualValues(t, 1800, october.GapSeconds)
	assert.Zero(t, october.SevenDay.PendingEvents)
	november, err := SummarizeMonth(ledger, "2026-11", location)
	require.NoError(t, err)
	assert.EqualValues(t, 1800, november.GapSeconds)
	assert.Equal(t, 1, november.SevenDay.PendingEvents)
	assert.EqualValues(t, start+1800, november.SevenDay.PendingSince)
	assert.Zero(t, november.SevenDay.ObservedBasisPoints)
}

func TestBuildLedgerPendingWeeklyWindowDoesNotBlockFiveHourWindow(t *testing.T) {
	const at = int64(1800000000)
	shortBaseline := quotaSample(2, at, 0, at+18000)
	shortBaseline.WindowSeconds = FiveHourSeconds
	shortGrowth := quotaSample(4, at+600, 300, at+18000)
	shortGrowth.WindowSeconds = FiveHourSeconds
	samples := []Sample{quotaSample(1, at, 3000, at+604800), shortBaseline,
		quotaSample(3, at+600, 200, at+605400), shortGrowth}
	pending, err := BuildLedger(samples, nil)
	require.NoError(t, err)
	require.Len(t, pending.Events, 1)
	assert.Equal(t, EventPending, pending.Events[0].Status)
	require.Len(t, pending.Deltas, 1)
	assert.Equal(t, FiveHourSeconds, pending.Deltas[0].WindowSeconds)
	assert.EqualValues(t, 300, pending.Deltas[0].BasisPoints)

	samples = append(samples, quotaSample(5, at+1200, 200, at+605400))
	confirmed, err := BuildLedger(samples, nil)
	require.NoError(t, err)
	require.Len(t, confirmed.Cycles, 3)
	assert.Equal(t, EventAccepted, confirmed.Events[0].Status)
	assert.EqualValues(t, 200, confirmed.Cycles[2].ObservedBasisPoints)
}
