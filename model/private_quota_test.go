package model

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// 每例使用独立真实SQLite文件，不替换全局DB，也不读取SQL_DSN或生产配置。
func privateQuotaTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "quota.db")+"?_pragma=busy_timeout(5000)"),
		&gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
	return db
}

func TestPrivateQuotaMigrationOwnsOnlyFourTables(t *testing.T) {
	db := privateQuotaTestDB(t)
	var version string
	require.NoError(t, db.Raw("SELECT sqlite_version()").Scan(&version).Error)
	t.Logf("实际SQLite版本：%s", version)
	require.NoError(t, db.AutoMigrate(&Channel{}, &Token{}))
	channel := Channel{Id: 17, Name: "legacy channel", Key: "synthetic-channel-key"}
	token := Token{Id: 23, UserId: 1, Key: "synthetic-model-key"}
	require.NoError(t, db.Create(&channel).Error)
	require.NoError(t, db.Create(&token).Error)
	before, err := db.Migrator().GetTables()
	require.NoError(t, err)
	privateQuotaMigrationContract(t, db)
	after, err := db.Migrator().GetTables()
	require.NoError(t, err)
	assert.ElementsMatch(t, append(before, "private_quota_settings", "private_quota_subjects", "private_quota_snapshots", "private_quota_events"), after)
	var storedChannel Channel
	var storedToken Token
	require.NoError(t, db.First(&storedChannel, channel.Id).Error)
	require.NoError(t, db.First(&storedToken, token.Id).Error)
	assert.Equal(t, channel.Key, storedChannel.Key)
	assert.Equal(t, token.Key, storedToken.Key)
	assert.Equal(t, channel.Name, storedChannel.Name)
}

// 同一契约在SQLite与显式隔离的MySQL/PostgreSQL上实际执行。
func privateQuotaMigrationContract(t *testing.T, db *gorm.DB) {
	t.Helper()
	require.NoError(t, MigratePrivateQuotaTables(db))
	// 三个实际引擎都执行初始化竞争，不以SQLite证据替代外部数据库。
	type initResult struct {
		settings PrivateQuotaSettings
		err      error
	}
	results := make(chan initResult, 2)
	start := make(chan struct{})
	var workers sync.WaitGroup
	for range 2 {
		workers.Go(func() {
			<-start
			row, err := InitializePrivateQuotaSettings(db.Session(&gorm.Session{NewDB: true}), 1800000000)
			results <- initResult{row, err}
		})
	}
	close(start)
	workers.Wait()
	a, b := <-results, <-results
	require.NoError(t, a.err)
	require.NoError(t, b.err)
	require.Equal(t, a.settings.FingerprintKey, b.settings.FingerprintKey)
	settings := a.settings
	assert.False(t, settings.Enabled)
	assert.Equal(t, 10, settings.IntervalMinutes)
	assert.EqualValues(t, 1, settings.Version)
	assert.Len(t, settings.FingerprintKey, 64)
	key := settings.FingerprintKey

	subject := PrivateQuotaSubject{Provider: "codex", IdentityFingerprint: strings.Repeat("a", 64),
		Name: "synthetic account", SourceChannelID: 17, Version: 1, RulesVersion: 1}
	require.NoError(t, db.Create(&subject).Error)
	require.Positive(t, subject.ID)
	duplicate := subject
	duplicate.ID = 0
	require.Error(t, db.Create(&duplicate).Error)

	snapshot := PrivateQuotaSnapshot{SubjectID: subject.ID, SampleKey: "synthetic-batch-1", WindowSeconds: 604800,
		SegmentID: "plus-v1", FetchedAt: 1800000000, RecordedAt: 1800000001, ResetAt: 1800604800,
		IntervalSeconds: 600, SourceChannelID: 17, QualityCode: "missing_percent"}
	require.NoError(t, db.Create(&snapshot).Error)
	var stored PrivateQuotaSnapshot
	require.NoError(t, db.First(&stored, snapshot.ID).Error)
	assert.Nil(t, stored.UsedBasisPoints, "缺失百分比必须保留NULL")
	duplicateSnapshot := snapshot
	duplicateSnapshot.ID = 0
	require.Error(t, db.Create(&duplicateSnapshot).Error)
	shortSnapshot := duplicateSnapshot
	shortSnapshot.WindowSeconds = 18000
	require.NoError(t, db.Create(&shortSnapshot).Error)

	event := PrivateQuotaEvent{SubjectID: subject.ID, EventKey: "synthetic-event-1", Kind: "plan_change", Status: "pending", Version: 1, RulesVersion: 1}
	require.NoError(t, db.Create(&event).Error)
	event.ID = 0
	require.Error(t, db.Create(&event).Error)

	for range 2 {
		require.NoError(t, MigratePrivateQuotaTables(db))
	}
	again, err := InitializePrivateQuotaSettings(db, 1800000600)
	require.NoError(t, err)
	assert.Equal(t, key, again.FingerprintKey)
	assert.EqualValues(t, 1, again.Version)
	require.NoError(t, db.First(&stored, snapshot.ID).Error)
	assert.Nil(t, stored.UsedBasisPoints)
	assert.EqualValues(t, 600, stored.IntervalSeconds)
	for _, spec := range []struct {
		model any
		index string
	}{
		{&PrivateQuotaSubject{}, "uidx_private_quota_identity"},
		{&PrivateQuotaSnapshot{}, "uidx_private_quota_sample"},
		{&PrivateQuotaSnapshot{}, "idx_private_quota_observation"},
		{&PrivateQuotaEvent{}, "uidx_private_quota_event"},
	} {
		assert.True(t, db.Migrator().HasIndex(spec.model, spec.index))
	}

	view, err := UpdatePrivateQuotaSettings(db, PrivateQuotaSettingsChange{
		Enabled: true, IntervalMinutes: 20, Version: 1, ActorID: 42, Now: 1800001200})
	require.NoError(t, err)
	assert.True(t, view.Enabled)
	assert.EqualValues(t, 2, view.Version)
	_, err = UpdatePrivateQuotaSettings(db, PrivateQuotaSettingsChange{
		Enabled: false, IntervalMinutes: 30, Version: 1, ActorID: 42, Now: 1800001300})
	require.ErrorIs(t, err, ErrPrivateQuotaConflict)
	view, err = UpdatePrivateQuotaSettings(db, PrivateQuotaSettingsChange{
		Enabled: false, IntervalMinutes: 2, Version: 2, ActorID: 42, Now: 1800001400})
	require.NoError(t, err)
	assert.False(t, view.Enabled, "map更新必须保存显式false")
	assert.EqualValues(t, 3, view.Version)
	var audits []PrivateQuotaEvent
	require.NoError(t, db.Where("kind = ?", "settings_updated").Order("id").Find(&audits).Error)
	require.Len(t, audits, 2)
	assert.EqualValues(t, 42, audits[0].ActorID)
	assert.Zero(t, audits[0].SubjectID)
	assert.NotEmpty(t, audits[0].Reason)
	storedSettings, err := GetPrivateQuotaSettings(db)
	require.NoError(t, err)
	assert.Equal(t, key, storedSettings.FingerprintKey)
	privateQuotaLeaseContract(t, db)
	privateQuotaSettingsConcurrencyContract(t, db)
}

// CAS与审计回滚在同一三库契约执行，配置调频不能留下部分due更新。
func privateQuotaSettingsConcurrencyContract(t *testing.T, db *gorm.DB) {
	t.Helper()
	settings, err := GetPrivateQuotaSettings(db)
	require.NoError(t, err)
	var before int64
	require.NoError(t, db.Model(&PrivateQuotaEvent{}).Where("kind = ?", "settings_updated").Count(&before).Error)
	start := make(chan struct{})
	results := make(chan error, 2)
	var workers sync.WaitGroup
	for range 2 {
		workers.Go(func() {
			<-start
			_, err := UpdatePrivateQuotaSettings(db.Session(&gorm.Session{NewDB: true}), PrivateQuotaSettingsChange{Enabled: true, IntervalMinutes: 20, Version: settings.Version, ActorID: 42, Now: 1800010000})
			results <- err
		})
	}
	close(start)
	workers.Wait()
	a, b := <-results, <-results
	if a == nil {
		require.ErrorIs(t, b, ErrPrivateQuotaConflict)
	} else {
		require.ErrorIs(t, a, ErrPrivateQuotaConflict)
		require.NoError(t, b)
	}
	var after int64
	require.NoError(t, db.Model(&PrivateQuotaEvent{}).Where("kind = ?", "settings_updated").Count(&after).Error)
	assert.Equal(t, before+1, after)
	current, err := GetPrivateQuotaSettings(db)
	require.NoError(t, err)
	require.NoError(t, db.Callback().Create().Before("gorm:create").Register("quota_reject_settings_audit", func(tx *gorm.DB) {
		if event, ok := tx.Statement.Dest.(*PrivateQuotaEvent); ok && event.Kind == "settings_updated" {
			tx.AddError(errors.New("synthetic settings audit failure"))
		}
	}))
	_, err = UpdatePrivateQuotaSettings(db, PrivateQuotaSettingsChange{Enabled: false, IntervalMinutes: 30, Version: current.Version, ActorID: 42, Now: 1800010600})
	require.Error(t, err)
	require.NoError(t, db.Callback().Create().Remove("quota_reject_settings_audit"))
	unchanged, err := GetPrivateQuotaSettings(db)
	require.NoError(t, err)
	assert.Equal(t, current, unchanged)
}

// 若去掉fence、版本或到期校验，旧worker会在接管/关闭后恢复写入权限。
func privateQuotaLeaseContract(t *testing.T, db *gorm.DB) {
	t.Helper()
	const now = int64(1800002000)
	_, err := ClaimPrivateQuotaLease(db, "worker-a", now)
	require.ErrorIs(t, err, ErrPrivateQuotaLeaseUnavailable, "默认关闭不能抢租约")
	settings, err := GetPrivateQuotaSettings(db)
	require.NoError(t, err)
	_, err = UpdatePrivateQuotaSettings(db, PrivateQuotaSettingsChange{
		Enabled: true, IntervalMinutes: 10, Version: settings.Version, ActorID: 42, Now: now})
	require.NoError(t, err)
	start := make(chan struct{})
	var wg sync.WaitGroup
	leases := make([]PrivateQuotaLease, 2)
	errs := make([]error, 2)
	for i, owner := range []string{"worker-a", "worker-b"} {
		wg.Go(func() {
			<-start
			leases[i], errs[i] = ClaimPrivateQuotaLease(db.Session(&gorm.Session{NewDB: true}), owner, now)
		})
	}
	close(start)
	wg.Wait()
	if errs[0] != nil {
		errs[0], errs[1], leases[0], leases[1] = errs[1], errs[0], leases[1], leases[0]
	}
	require.NoError(t, errs[0])
	require.ErrorIs(t, errs[1], ErrPrivateQuotaLeaseUnavailable)
	lease := leases[0]
	assert.EqualValues(t, 1, lease.Fence)
	assert.Equal(t, settings.Version+1, lease.SettingsVersion)
	assert.EqualValues(t, now+120, lease.Until)
	lease, err = RenewPrivateQuotaLease(db, lease, now+30)
	require.NoError(t, err)
	assert.EqualValues(t, now+150, lease.Until)
	lease, err = RenewPrivateQuotaLease(db, lease, now+20)
	require.NoError(t, err, "倒退的时间不能缩短租约或因MySQL零行更新误判失租")
	assert.EqualValues(t, now+150, lease.Until)
	_, err = ClaimPrivateQuotaLease(db, "next", now+149)
	require.ErrorIs(t, err, ErrPrivateQuotaLeaseUnavailable)
	_, err = RenewPrivateQuotaLease(db, lease, now+150)
	require.ErrorIs(t, err, ErrPrivateQuotaLeaseLost)
	newLease, err := ClaimPrivateQuotaLease(db, lease.Owner, now+150)
	require.NoError(t, err)
	assert.EqualValues(t, 2, newLease.Fence, "同名worker重新获得租约也必须增加fence")
	_, err = RenewPrivateQuotaLease(db, lease, now+151)
	require.ErrorIs(t, err, ErrPrivateQuotaLeaseLost)
	require.ErrorIs(t, ReleasePrivateQuotaLease(db, lease), ErrPrivateQuotaLeaseLost)
	require.NoError(t, ReleasePrivateQuotaLease(db, newLease))
	newLease, err = ClaimPrivateQuotaLease(db, "next", now+151)
	require.NoError(t, err)
	assert.EqualValues(t, 3, newLease.Fence)
	_, err = UpdatePrivateQuotaSettings(db, PrivateQuotaSettingsChange{
		Enabled: false, IntervalMinutes: 10, Version: newLease.SettingsVersion, ActorID: 42, Now: now + 152})
	require.NoError(t, err)
	_, err = RenewPrivateQuotaLease(db, newLease, now+153)
	require.ErrorIs(t, err, ErrPrivateQuotaLeaseLost)
	require.NoError(t, ReleasePrivateQuotaLease(db, newLease), "关闭后仍可释放自己持有的租约")
	for _, owner := range []string{"", " worker", "bad\nowner", strings.Repeat("a", 129)} {
		_, err = ClaimPrivateQuotaLease(db, owner, now)
		require.ErrorIs(t, err, ErrPrivateQuotaInputInvalid)
	}
	_, err = ClaimPrivateQuotaLease(db, "worker", 0)
	require.ErrorIs(t, err, ErrPrivateQuotaInputInvalid)
	body, err := common.Marshal(newLease)
	require.NoError(t, err)
	assert.JSONEq(t, `{}`, string(body), "内部租约不得被直接序列化到API")
}

func TestPrivateQuotaInitializationRefusesLostOrInvalidIdentityKey(t *testing.T) {
	for _, name := range []string{"malformed_key", "lost_settings_subjects", "lost_settings_snapshots", "lost_settings_events"} {
		t.Run(name, func(t *testing.T) {
			db := privateQuotaTestDB(t)
			require.NoError(t, MigratePrivateQuotaTables(db))
			switch name {
			case "malformed_key":
				_, err := InitializePrivateQuotaSettings(db, 1800000000)
				require.NoError(t, err)
				require.NoError(t, db.Model(&PrivateQuotaSettings{}).Where("id = ?", 1).Update("fingerprint_key", "invalid").Error)
			case "lost_settings_subjects":
				require.NoError(t, db.Create(&PrivateQuotaSubject{IdentityFingerprint: strings.Repeat("b", 64)}).Error)
			case "lost_settings_snapshots":
				require.NoError(t, db.Create(&PrivateQuotaSnapshot{SampleKey: "history", SegmentID: "v1"}).Error)
			case "lost_settings_events":
				require.NoError(t, db.Create(&PrivateQuotaEvent{EventKey: "history"}).Error)
			}
			_, err := InitializePrivateQuotaSettings(db, 1800000600)
			require.ErrorIs(t, err, ErrPrivateQuotaStateInvalid)
			var rows int64
			require.NoError(t, db.Model(&PrivateQuotaSettings{}).Count(&rows).Error)
			if name == "malformed_key" {
				assert.EqualValues(t, 1, rows)
				var settings PrivateQuotaSettings
				require.NoError(t, db.First(&settings, 1).Error)
				assert.Equal(t, "invalid", settings.FingerprintKey)
			} else {
				assert.Zero(t, rows, "有历史却失去密钥时不得生成新账本身份")
			}
		})
	}
}

func TestPrivateQuotaInitializationKeepsOnlyOneWinnerAcrossConnections(t *testing.T) {
	first := privateQuotaTestDB(t)
	require.NoError(t, MigratePrivateQuotaTables(first))
	// 显式复用同一测试文件，但第二个连接池独立，模拟两个进程同时初始化。
	dsn := first.Dialector.(*sqlite.Dialector).DSN
	second, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	sqlDB, err := second.DB()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
	type result struct {
		settings PrivateQuotaSettings
		err      error
	}
	results := make(chan result, 2)
	start := make(chan struct{})
	var workers sync.WaitGroup
	for _, db := range []*gorm.DB{first, second} {
		workers.Go(func() {
			<-start
			settings, err := InitializePrivateQuotaSettings(db, 1800000000)
			results <- result{settings, err}
		})
	}
	close(start)
	workers.Wait()
	a, b := <-results, <-results
	require.NoError(t, a.err)
	require.NoError(t, b.err)
	assert.Equal(t, a.settings.FingerprintKey, b.settings.FingerprintKey)
	var count int64
	require.NoError(t, first.Model(&PrivateQuotaSettings{}).Count(&count).Error)
	assert.EqualValues(t, 1, count)
}

func TestPrivateQuotaSettingsAuditFailureRollsBackAndDoesNotTouchLease(t *testing.T) {
	db := privateQuotaTestDB(t)
	require.NoError(t, MigratePrivateQuotaTables(db))
	settings, err := InitializePrivateQuotaSettings(db, 1800000000)
	require.NoError(t, err)
	require.NoError(t, db.Model(&PrivateQuotaSettings{}).Where("id = ?", 1).
		Updates(map[string]any{"lease_owner": "synthetic-worker", "lease_until": 1800001000, "lease_version": 7}).Error)
	require.NoError(t, db.Migrator().DropTable(&PrivateQuotaEvent{}))
	_, err = UpdatePrivateQuotaSettings(db, PrivateQuotaSettingsChange{
		Enabled: true, IntervalMinutes: 20, Version: 1, ActorID: 42, Now: 1800000600})
	require.Error(t, err)
	stored, err := GetPrivateQuotaSettings(db)
	require.NoError(t, err)
	assert.False(t, stored.Enabled)
	assert.Equal(t, 10, stored.IntervalMinutes)
	assert.EqualValues(t, 1, stored.Version)
	assert.Equal(t, settings.FingerprintKey, stored.FingerprintKey)
	assert.Equal(t, "synthetic-worker", stored.LeaseOwner)
	assert.EqualValues(t, 7, stored.LeaseVersion)
	// 成功保存同样不能顺手清掉/抢占采集租约。
	require.NoError(t, MigratePrivateQuotaTables(db))
	_, err = UpdatePrivateQuotaSettings(db, PrivateQuotaSettingsChange{
		Enabled: true, IntervalMinutes: 1440, Version: 1, ActorID: 42, Now: 1800000600})
	require.NoError(t, err)
	stored, err = GetPrivateQuotaSettings(db)
	require.NoError(t, err)
	assert.Equal(t, 1440, stored.IntervalMinutes)
	assert.Equal(t, "synthetic-worker", stored.LeaseOwner)
	assert.EqualValues(t, 1800001000, stored.LeaseUntil)
	assert.EqualValues(t, 7, stored.LeaseVersion)
}

func TestPrivateQuotaConcurrentSettingsChangesHaveOneWinnerAndAudit(t *testing.T) {
	db := privateQuotaTestDB(t)
	require.NoError(t, MigratePrivateQuotaTables(db))
	_, err := InitializePrivateQuotaSettings(db, 1800000000)
	require.NoError(t, err)
	// 多个独立连接可同时竞争；不能依靠Go进程内互斥锁保护跨实例版本。
	dsn := db.Dialector.(*sqlite.Dialector).DSN
	other, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	sqlDB, err := other.DB()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
	start := make(chan struct{})
	results := make(chan error, 2)
	var workers sync.WaitGroup
	for _, conn := range []*gorm.DB{db, other} {
		workers.Go(func() {
			<-start
			_, err := UpdatePrivateQuotaSettings(conn, PrivateQuotaSettingsChange{
				Enabled: true, IntervalMinutes: 20, Version: 1, ActorID: 42, Now: 1800000600})
			results <- err
		})
	}
	close(start)
	workers.Wait()
	a, b := <-results, <-results
	if a == nil {
		require.ErrorIs(t, b, ErrPrivateQuotaConflict)
	} else {
		require.ErrorIs(t, a, ErrPrivateQuotaConflict)
		require.NoError(t, b)
	}
	var audits int64
	require.NoError(t, db.Model(&PrivateQuotaEvent{}).Count(&audits).Error)
	assert.EqualValues(t, 1, audits)
	stored, err := GetPrivateQuotaSettings(db)
	require.NoError(t, err)
	assert.EqualValues(t, 2, stored.Version)
}

func TestPrivateQuotaSettingsRejectsInvalidChangesAndHidesInternalFields(t *testing.T) {
	db := privateQuotaTestDB(t)
	require.NoError(t, MigratePrivateQuotaTables(db))
	settings, err := InitializePrivateQuotaSettings(db, 1800000000)
	require.NoError(t, err)
	for _, interval := range []int{-1, 0, 1, 1441} {
		_, err := UpdatePrivateQuotaSettings(db, PrivateQuotaSettingsChange{
			IntervalMinutes: interval, Version: 1, ActorID: 42, Now: 1800000600})
		require.ErrorIs(t, err, ErrPrivateQuotaInputInvalid)
	}
	for _, change := range []PrivateQuotaSettingsChange{
		{IntervalMinutes: 10, Version: 0, ActorID: 42, Now: 1800000600},
		{IntervalMinutes: 10, Version: 1, ActorID: 0, Now: 1800000600},
		{IntervalMinutes: 10, Version: 1, ActorID: 42, Now: 0},
	} {
		_, err := UpdatePrivateQuotaSettings(db, change)
		require.ErrorIs(t, err, ErrPrivateQuotaInputInvalid)
	}
	settings.LeaseOwner = "synthetic-internal-owner"
	raw, err := common.Marshal(settings)
	require.NoError(t, err)
	assert.NotContains(t, string(raw), settings.FingerprintKey)
	assert.NotContains(t, string(raw), settings.LeaseOwner)
	assert.NotContains(t, string(raw), "fingerprint_key")
	assert.NotContains(t, string(raw), "lease_")
	var count int64
	require.NoError(t, db.Model(&PrivateQuotaEvent{}).Count(&count).Error)
	assert.Zero(t, count)
}

func TestPrivateQuotaMigrationPropagatesClosedDatabaseError(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())
	require.Error(t, MigratePrivateQuotaTables(db))
	_, err = InitializePrivateQuotaSettings(db, 1800000000)
	require.Error(t, err)
}

func TestPrivateQuotaIsolatedSQLDatabases(t *testing.T) {
	if os.Getenv("PRIVATE_QUOTA_ISOLATED_TEST_DB") != "1" {
		t.Skip("需要显式隔离数据库授权；不读取生产SQL_DSN")
	}
	for _, dialect := range []string{"mysql", "postgres"} {
		t.Run(dialect, func(t *testing.T) {
			var driver gorm.Dialector
			if dialect == "mysql" {
				dsn := os.Getenv("PRIVATE_QUOTA_TEST_MYSQL_DSN")
				if dsn == "" {
					t.Skip("未配置独立MySQL测试DSN，不能视为验证通过")
				}
				driver = mysql.Open(dsn)
			} else {
				dsn := os.Getenv("PRIVATE_QUOTA_TEST_POSTGRES_DSN")
				if dsn == "" {
					t.Skip("未配置独立PostgreSQL测试DSN，不能视为验证通过")
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
			require.True(t, strings.HasPrefix(database, "new_api_private_quota_test_"), "仅允许全新专用测试数据库")
			for _, table := range []any{&PrivateQuotaSettings{}, &PrivateQuotaSubject{}, &PrivateQuotaSnapshot{}, &PrivateQuotaEvent{}} {
				require.False(t, db.Migrator().HasTable(table), "必须使用空的隔离测试库，不删除已有数据")
			}
			var version string
			require.NoError(t, db.Raw("SELECT version()").Scan(&version).Error)
			t.Logf("实际引擎版本：%s", version)
			privateQuotaMigrationContract(t, db)
		})
	}
}
