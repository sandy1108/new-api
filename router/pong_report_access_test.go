package router

import (
	"errors"
	"fmt"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/service/reportaccess"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func reportAccessDB(t *testing.T) (*gorm.DB, model.AuthSessionIdentity, model.User) {
	t.Helper()
	oldType, oldLogType := common.MainDatabaseType(), common.LogDatabaseType()
	dialect := common.DatabaseTypeSQLite
	var driver gorm.Dialector = sqlite.Open(filepath.Join(t.TempDir(), "reports.db") + "?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_txlock=immediate")
	config := &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)}
	if engine := os.Getenv("PONG_REPORT_TEST_DB"); engine != "" {
		// 仅测试显式启动的隔离数据库；所有临时库名均内部生成，不接受生产对象名。
		name := fmt.Sprintf("pong_report_test_%d", time.Now().UnixNano())
		dsn := os.Getenv("PONG_REPORT_TEST_DSN")
		require.NotEmpty(t, dsn)
		var adminDriver gorm.Dialector
		switch engine {
		case "mysql":
			dialect = common.DatabaseTypeMySQL
			adminDriver = mysql.Open(dsn)
		case "postgres":
			dialect = common.DatabaseTypePostgreSQL
			adminDriver = postgres.Open(dsn)
		default:
			t.Fatalf("unsupported test engine %q", engine)
		}
		control, err := gorm.Open(adminDriver, config)
		require.NoError(t, err)
		require.NoError(t, control.Exec("CREATE DATABASE "+name).Error)
		t.Cleanup(func() {
			require.NoError(t, control.Exec("DROP DATABASE "+name).Error)
			connection, err := control.DB()
			require.NoError(t, err)
			require.NoError(t, connection.Close())
		})
		if engine == "mysql" {
			driver = mysql.Open(strings.Replace(dsn, "/mysql?", "/"+name+"?", 1))
		} else {
			driver = postgres.Open(strings.Replace(dsn, "dbname=postgres", "dbname="+name, 1))
		}
	}
	common.SetDatabaseTypes(dialect, dialect)
	t.Cleanup(func() { common.SetDatabaseTypes(oldType, oldLogType) })
	db, err := gorm.Open(driver, config)
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(5)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.UserSession{}))
	root := model.User{Username: "report-root", AffCode: "report-root", Password: "synthetic-password-hash", Role: common.RoleRootUser, Status: common.UserStatusEnabled, AuthVersion: 1}
	viewer := model.User{Username: "report-viewer", AffCode: "report-viewer", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, AuthVersion: 1}
	require.NoError(t, db.Create(&root).Error)
	require.NoError(t, db.Create(&viewer).Error)
	identity := model.AuthSessionIdentity{UserID: root.Id, SessionID: "report-root-session", UserAuthVersion: 1, SessionVersion: 1}
	require.NoError(t, db.Create(&model.UserSession{SID: identity.SessionID, UserID: root.Id, UserAuthVersion: 1, Version: 1, Status: model.UserSessionStatusActive, RefreshHash: "synthetic", ExpiresAt: time.Now().Unix() + 3600}).Error)
	require.NoError(t, model.MigratePrivateReportAccessTables(db))
	return db, identity, viewer
}

func TestPongReportRoutesRequireSessionExactCapabilityAndRootProof(t *testing.T) {
	db, root, viewer := reportAccessDB(t)
	oldDB, oldLogDB, oldRedis, oldSecret := model.DB, model.LOG_DB, common.RedisEnabled, common.SessionSecret
	oldType, oldPassword := common.MainDatabaseType(), common.PasswordLoginEnabled
	model.DB, model.LOG_DB, common.RedisEnabled, common.SessionSecret = db, db, false, "synthetic-report-test-secret"
	common.PasswordLoginEnabled = true
	t.Cleanup(func() {
		model.DB, model.LOG_DB, common.RedisEnabled, common.SessionSecret = oldDB, oldLogDB, oldRedis, oldSecret
		common.SetMainDatabaseType(oldType)
		common.PasswordLoginEnabled = oldPassword
	})
	require.NoError(t, model.MigratePrivateQuotaTables(db))
	_, err := model.InitializePrivateQuotaSettings(db, time.Now().Unix())
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Log{}, &model.Channel{}, &model.AuditLog{}, &model.Option{}, &model.AuthFlow{}, &model.TwoFA{}, &model.PasskeyCredential{}, &model.UserAccessToken{}))
	viewerIdentity := model.AuthSessionIdentity{UserID: viewer.Id, SessionID: "viewer-real-session", UserAuthVersion: 1, SessionVersion: 1}
	require.NoError(t, db.Create(&model.UserSession{SID: viewerIdentity.SessionID, UserID: viewer.Id, UserAuthVersion: 1, Version: 1, Status: model.UserSessionStatusActive, RefreshHash: "synthetic-viewer", ExpiresAt: time.Now().Unix() + 3600}).Error)
	rootToken, _, err := service.IssueAccessToken(root)
	require.NoError(t, err)
	viewerToken, _, err := service.IssueAccessToken(viewerIdentity)
	require.NoError(t, err)
	engine := gin.New()
	registerPongReportRoutes(engine.Group("/api"))
	perform := func(method, path, token, body, proof string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, "http://reports.test"+path, strings.NewReader(body))
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Origin", "http://reports.test")
		req.Header.Set("X-PongAPI-Request", "1")
		req.Header.Set("X-Security-Proof", proof)
		out := httptest.NewRecorder()
		engine.ServeHTTP(out, req)
		return out
	}
	assert.Equal(t, 401, perform("GET", "/api/pong-reports/capabilities", "", "", "").Code)
	assert.Equal(t, 200, perform("GET", "/api/pong-reports/capabilities", viewerToken, "", "").Code)
	patRaw := model.AccessTokenPrefix + strings.Repeat("a", 64)
	pat := model.UserAccessToken{UserId: root.UserID, TokenHash: model.AccessTokenFingerprint(patRaw), Name: "synthetic-root-pat", Scopes: `["usage:read","profile:read"]`, ExpiresAt: time.Now().Unix() + 3600}
	require.NoError(t, db.Create(&pat).Error)
	for _, path := range []string{"/api/pong-reports/capabilities", "/api/pong-report-access/users", "/api/pong-reports/subscription/subjects"} {
		assert.Equal(t, 403, perform("GET", path, patRaw, "", "").Code, "PAT不能继承Root报表会话能力")
	}
	assert.Equal(t, 401, perform("GET", "/api/pong-reports/capabilities", "sk-synthetic-model-key", "", "").Code)
	assert.Equal(t, 401, perform("GET", "/api/pong-reports/capabilities", "napi_synthetic-client-credential", "", "").Code)
	usage := "/api/pong-reports/usage-summary?start_timestamp=1800000000&end_timestamp=1800003600&include_trend=true&scope=all"
	assert.Equal(t, 403, perform("GET", usage, viewerToken, "", "").Code)
	target := fmt.Sprintf("/api/pong-report-access/users/%d", viewer.Id)
	body := `{"usage_read_all":true,"subscription_read_all":false,"version":0}`
	assert.Equal(t, 403, perform("PUT", target, viewerToken, body, "").Code)
	assert.Equal(t, 403, perform("PUT", target, rootToken, body, "").Code)
	context := []byte(fmt.Sprintf(`{"user_id":%d,"usage_read_all":true,"subscription_read_all":false,"version":0}`, viewer.Id))
	binding, err := service.BindVerificationOperation(service.VerificationOperation{Scope: service.VerificationScopePongReportGrants, Context: context})
	require.NoError(t, err)
	proof, err := service.CompleteSecurityVerification(root, binding, service.VerificationMethodPassword)
	require.NoError(t, err)
	response := perform("PUT", target, rootToken, body, proof.ProofToken)
	require.Equal(t, 200, response.Code, response.Body.String())
	assert.Equal(t, 403, perform("PUT", target, rootToken, body, proof.ProofToken).Code, "proof不可重放")
	response = perform("GET", usage, viewerToken, "", "")
	assert.Equal(t, 200, response.Code, response.Body.String())
	assert.Contains(t, response.Header().Get("Cache-Control"), "no-store")
	assert.Equal(t, 403, perform("GET", "/api/pong-reports/subscription/subjects", viewerToken, "", "").Code)
	assert.Equal(t, 403, perform("GET", "/api/pong-report-access/users", viewerToken, "", "").Code)
	assert.NotEqual(t, 200, perform("POST", "/api/pong-reports/usage-summary", viewerToken, "{}", "").Code)
	// 授权只放开独立报表，旧管理接口与未知参数均不得借此获取数据。
	for _, query := range []string{"&scope=self&scope=all", "&user_id=1", "&channel_id=1", "&scope=invalid"} {
		assert.Equal(t, 400, perform("GET", usage+query, viewerToken, "", "").Code)
	}
	assert.Equal(t, 400, perform("GET", "/api/pong-reports/usage-summary?start_timestamp=1&end_timestamp=999999999&scope=all", viewerToken, "", "").Code)
	for _, row := range []model.Log{
		{UserId: viewer.Id, Username: "viewer", TokenName: "viewer-key", Type: model.LogTypeConsume, CreatedAt: 1800000010, PromptTokens: 10, CompletionTokens: 20},
		{UserId: root.UserID, Username: "root", TokenName: "root-key", Type: model.LogTypeConsume, CreatedAt: 1800000020, PromptTokens: 30, CompletionTokens: 40},
	} {
		require.NoError(t, db.Create(&row).Error)
	}
	var totals struct {
		Data struct {
			TotalTokens int64 `json:"total_tokens"`
		} `json:"data"`
	}
	response = perform("GET", usage, viewerToken, "", "")
	require.Equal(t, 200, response.Code, response.Body.String())
	require.NoError(t, common.Unmarshal(response.Body.Bytes(), &totals))
	assert.EqualValues(t, 100, totals.Data.TotalTokens)
	response = perform("GET", strings.ReplaceAll(usage, "scope=all", "scope=self"), viewerToken, "", "")
	require.NoError(t, common.Unmarshal(response.Body.Bytes(), &totals))
	assert.EqualValues(t, 30, totals.Data.TotalTokens, "self必须由服务端绑定当前用户")
	// proof的目标、勾选和版本绑定，不能拿一次验证改写另一项权限。
	context = []byte(fmt.Sprintf(`{"user_id":%d,"usage_read_all":true,"subscription_read_all":false,"version":1}`, viewer.Id))
	binding, err = service.BindVerificationOperation(service.VerificationOperation{Scope: service.VerificationScopePongReportGrants, Context: context})
	require.NoError(t, err)
	proof, err = service.CompleteSecurityVerification(root, binding, service.VerificationMethodPassword)
	require.NoError(t, err)
	assert.Equal(t, 403, perform("PUT", target, rootToken, `{"usage_read_all":true,"subscription_read_all":true,"version":1}`, proof.ProofToken).Code)
	// 不会以二次验证结果替代同源JSON护栏。
	req := httptest.NewRequest("PUT", "http://reports.test"+target, strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+rootToken)
	req.Header.Set("Origin", "https://evil.invalid")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-PongAPI-Request", "1")
	req.Header.Set("X-Security-Proof", proof.ProofToken)
	out := httptest.NewRecorder()
	engine.ServeHTTP(out, req)
	assert.Equal(t, 403, out.Code)
	// 切成账本单项授权，全部五个读取端点可用，但配置、保存和渠道引用不暴露。
	_, err = model.UpdatePrivateReportGrant(db, root, viewer.Id, model.PrivateReportGrantChange{SubscriptionReadAll: true, Version: 1})
	require.NoError(t, err)
	subject := model.PrivateQuotaSubject{Provider: "codex", Name: "Synthetic subscription", IdentityFingerprint: "synthetic-fingerprint-not-for-observers", SourceChannelID: 987654, Version: 7, Enabled: true}
	require.NoError(t, db.Create(&subject).Error)
	var before, after model.PrivateQuotaSettings
	require.NoError(t, db.First(&before, 1).Error)
	for _, endpoint := range []string{"subjects", "summary", "cycles", "trend", "events"} {
		path := "/api/pong-reports/subscription/" + endpoint
		if endpoint != "subjects" {
			path += fmt.Sprintf("?subject_id=%d&month=2026-10&window_seconds=604800", subject.ID)
		}
		response = perform("GET", path, viewerToken, "", "")
		require.Equal(t, 200, response.Code, response.Body.String())
		for _, forbidden := range []string{"source_channel_id", "identity_fingerprint", "fingerprint_key", "lease_owner", "next_due_at", "synthetic-fingerprint"} {
			assert.NotContains(t, response.Body.String(), forbidden)
		}
	}
	require.NoError(t, db.First(&after, 1).Error)
	assert.Equal(t, before, after, "一致性读锁不得改变采集配置、租约或版本")
	assert.Equal(t, 403, perform("GET", usage, viewerToken, "", "").Code)
	for _, path := range []string{"settings", "channels", "subjects/1", "events/1/decision"} {
		assert.Equal(t, 404, perform("PUT", "/api/pong-reports/subscription/"+path, viewerToken, "{}", "").Code)
	}
	_, err = model.UpdatePrivateReportGrant(db, root, viewer.Id, model.PrivateReportGrantChange{Version: 2})
	require.NoError(t, err)
	assert.Equal(t, 403, perform("GET", usage, viewerToken, "", "").Code)
}

// 若错误地把普通角色映射为管理员，或授权保存在会话缓存，这些断言必须失败。
func TestPongReportGrantIsolationRevocationAndIdentity(t *testing.T) {
	db, root, viewer := reportAccessDB(t)
	store := reportaccess.Store{DB: db}
	viewerIdentity := model.AuthSessionIdentity{UserID: viewer.Id, SessionID: "viewer", UserAuthVersion: 1, SessionVersion: 1}
	caps, err := store.Capabilities(t.Context(), viewerIdentity)
	require.NoError(t, err)
	assert.False(t, caps.UsageReadAll)
	assert.False(t, caps.SubscriptionReadAll)
	assert.False(t, caps.CanManageGrants)

	grant, err := model.UpdatePrivateReportGrant(db, root, viewer.Id, model.PrivateReportGrantChange{UsageReadAll: true, Version: 0})
	require.NoError(t, err)
	assert.EqualValues(t, 1, grant.Version)
	caps, err = store.Capabilities(t.Context(), viewerIdentity)
	require.NoError(t, err)
	assert.True(t, caps.UsageReadAll)
	assert.False(t, caps.SubscriptionReadAll)
	assert.False(t, caps.CanManageGrants)
	var storedUser model.User
	require.NoError(t, db.First(&storedUser, viewer.Id).Error)
	assert.Equal(t, common.RoleCommonUser, storedUser.Role)

	_, err = model.UpdatePrivateReportGrant(db, root, viewer.Id, model.PrivateReportGrantChange{SubscriptionReadAll: true, Version: 0})
	require.ErrorIs(t, err, model.ErrPrivateReportConflict)
	grant, err = model.UpdatePrivateReportGrant(db, root, viewer.Id, model.PrivateReportGrantChange{Version: 1})
	require.NoError(t, err)
	assert.EqualValues(t, 2, grant.Version)
	caps, err = store.Capabilities(t.Context(), viewerIdentity)
	require.NoError(t, err)
	assert.False(t, caps.UsageReadAll, "撤权后新查询不能使用旧授权")

	_, err = model.UpdatePrivateReportGrant(db, root, viewer.Id, model.PrivateReportGrantChange{UsageReadAll: true, SubscriptionReadAll: true, Version: 2})
	require.NoError(t, err)
	require.NoError(t, db.Model(&model.User{}).Where("id = ?", viewer.Id).Update("auth_version", 2).Error)
	viewerIdentity.UserAuthVersion = 2
	caps, err = store.Capabilities(t.Context(), viewerIdentity)
	require.NoError(t, err)
	assert.False(t, caps.UsageReadAll, "安全代际变化不能通过重新登录复活旧授权")
	assert.False(t, caps.SubscriptionReadAll)

	_, err = model.UpdatePrivateReportGrant(db, root, root.UserID, model.PrivateReportGrantChange{UsageReadAll: true})
	require.ErrorIs(t, err, model.ErrPrivateReportTarget)
	_, err = model.UpdatePrivateReportGrant(db, viewerIdentity, viewer.Id, model.PrivateReportGrantChange{UsageReadAll: true, Version: 3})
	require.Error(t, err, "普通用户不可自授权")
}

func TestPongReportGrantAuditFailureAndMigrationPreserveData(t *testing.T) {
	db, root, viewer := reportAccessDB(t)
	_, err := model.UpdatePrivateReportGrant(db, root, viewer.Id, model.PrivateReportGrantChange{SubscriptionReadAll: true})
	require.NoError(t, err)
	for range 2 {
		require.NoError(t, model.MigratePrivateReportAccessTables(db))
	}
	require.NoError(t, db.Callback().Create().Before("gorm:create").Register("reject_report_audit", func(tx *gorm.DB) {
		if tx.Statement.Table == "private_report_grant_events" {
			tx.AddError(errors.New("synthetic audit failure"))
		}
	}))
	_, err = model.UpdatePrivateReportGrant(db, root, viewer.Id, model.PrivateReportGrantChange{UsageReadAll: true, Version: 1})
	require.Error(t, err)
	require.NoError(t, db.Callback().Create().Remove("reject_report_audit"))
	var grant model.PrivateReportGrant
	require.NoError(t, db.Where("user_id = ?", viewer.Id).First(&grant).Error)
	assert.EqualValues(t, 1, grant.Version)
	assert.False(t, grant.UsageReadAll)
	assert.True(t, grant.SubscriptionReadAll)
	var count int64
	require.NoError(t, db.Model(&model.PrivateReportGrantEvent{}).Count(&count).Error)
	assert.EqualValues(t, 1, count)
}

func TestPongReportStrictInput(t *testing.T) {
	for _, body := range []string{
		`{"usage_read_all":true,"subscription_read_all":false,"version":0,"role":100}`,
		`{"usage_read_all":true,"usage_read_all":false,"subscription_read_all":false,"version":0}`,
		`{"usage_read_all":null,"subscription_read_all":false,"version":0}`,
		`{"usage_read_all":true,"subscription_read_all":false,"version":-1}`,
		`{"usage_read_all":true,"subscription_read_all":false,"version":0} {}`,
	} {
		_, err := reportaccess.ParseChange([]byte(body), false)
		require.Error(t, err, body)
	}
	change, err := reportaccess.ParseChange([]byte(`{"usage_read_all":true,"subscription_read_all":false,"version":0}`), false)
	require.NoError(t, err)
	assert.True(t, change.UsageReadAll)
}

func TestPongReportDisabledDeletedAndSessionRevoked(t *testing.T) {
	db, root, viewer := reportAccessDB(t)
	_, err := model.UpdatePrivateReportGrant(db, root, viewer.Id, model.PrivateReportGrantChange{UsageReadAll: true, SubscriptionReadAll: true})
	require.NoError(t, err)
	store := reportaccess.Store{DB: db}
	identity := model.AuthSessionIdentity{UserID: viewer.Id, SessionID: "viewer", UserAuthVersion: 1, SessionVersion: 1}
	require.NoError(t, db.Model(&model.User{}).Where("id = ?", viewer.Id).Update("status", common.UserStatusDisabled).Error)
	_, err = store.Capabilities(t.Context(), identity)
	require.ErrorIs(t, err, model.ErrPrivateReportForbidden)
	require.NoError(t, db.Delete(&model.User{}, viewer.Id).Error)
	_, err = store.Capabilities(t.Context(), identity)
	require.ErrorIs(t, err, gorm.ErrRecordNotFound)
	require.NoError(t, db.Model(&model.UserSession{}).Where("sid = ?", root.SessionID).Update("status", model.UserSessionStatusRevoked).Error)
	_, err = model.UpdatePrivateReportGrant(db, root, viewer.Id, model.PrivateReportGrantChange{Version: 1})
	require.Error(t, err, "已撤销的Root会话不能改授权")
}

// 相同版本并发写必须至多一个成功，另一个明确冲突，且审计与最终授权一致。
func TestPongReportConcurrentGrantCAS(t *testing.T) {
	db, root, viewer := reportAccessDB(t)
	start := make(chan struct{})
	results := make(chan error, 2)
	var wait sync.WaitGroup
	for _, usage := range []bool{true, false} {
		wait.Go(func() {
			<-start
			_, err := model.UpdatePrivateReportGrant(db, root, viewer.Id, model.PrivateReportGrantChange{UsageReadAll: usage, SubscriptionReadAll: !usage})
			results <- err
		})
	}
	close(start)
	wait.Wait()
	close(results)
	var success, conflict int
	for err := range results {
		if err == nil {
			success++
		} else if errors.Is(err, model.ErrPrivateReportConflict) {
			conflict++
		} else {
			t.Errorf("unexpected concurrent error: %v", err)
		}
	}
	assert.Equal(t, 1, success)
	assert.Equal(t, 1, conflict)
	var count int64
	require.NoError(t, db.Model(&model.PrivateReportGrantEvent{}).Count(&count).Error)
	assert.EqualValues(t, 1, count)
}
