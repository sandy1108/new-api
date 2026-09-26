package model

import (
	"os"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestClientAccessMigrationCreatesOnlyOwnedTablesAndIsIdempotent(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&Channel{}, &Token{}))
	require.NoError(t, db.Create(&Channel{Id: 17, Name: "preserved channel", Key: "secret"}).Error)
	require.NoError(t, db.Create(&Token{Id: 23, UserId: 1, Key: "preserved-token"}).Error)

	for range 2 {
		require.NoError(t, MigrateClientAccessTables(db))
	}
	require.True(t, db.Migrator().HasTable(&ClientAccessCredential{}))
	require.True(t, db.Migrator().HasTable(&ClientAccessGrant{}))
	require.True(t, db.Migrator().HasIndex(&ClientAccessCredential{}, "idx_client_access_credentials_token_hash"))
	require.True(t, db.Migrator().HasIndex(&ClientAccessGrant{}, "uniq_client_access_grant"))

	var channelCount, tokenCount int64
	require.NoError(t, db.Model(&Channel{}).Count(&channelCount).Error)
	require.NoError(t, db.Model(&Token{}).Count(&tokenCount).Error)
	require.EqualValues(t, 1, channelCount)
	require.EqualValues(t, 1, tokenCount)
}

// 只在显式声明的隔离测试数据库上执行；绝不自动连接现有生产数据库。
func TestClientAccessMigrationIsolatedSQLDatabases(t *testing.T) {
	if os.Getenv("CLIENT_ACCESS_ISOLATED_TEST_DB") != "1" {
		t.Skip("requires isolated test database opt-in")
	}
	for _, tc := range []struct {
		name string
		env  string
		open func(string) gorm.Dialector
	}{
		{"postgres", "CLIENT_ACCESS_TEST_POSTGRES_DSN", func(dsn string) gorm.Dialector { return postgres.Open(dsn) }},
		{"mysql", "CLIENT_ACCESS_TEST_MYSQL_DSN", func(dsn string) gorm.Dialector { return mysql.Open(dsn) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dsn := os.Getenv(tc.env)
			if dsn == "" {
				t.Skip(tc.env + " is not configured")
			}
			db, err := gorm.Open(tc.open(dsn), &gorm.Config{})
			require.NoError(t, err)
			sqlDB, err := db.DB()
			require.NoError(t, err)
			t.Cleanup(func() { _ = sqlDB.Close() })
			// 模拟已有官方业务表和历史行，再确认私有迁移不改写旧数据。
			require.NoError(t, db.AutoMigrate(&Channel{}, &Token{}))
			require.NoError(t, db.Create(&Channel{Id: 901, Name: "legacy channel", Key: "legacy-secret"}).Error)
			require.NoError(t, db.Create(&Token{Id: 902, UserId: 1, Key: "legacy-token"}).Error)
			require.NoError(t, MigrateClientAccessTables(db))
			require.NoError(t, MigrateClientAccessTables(db))
			require.True(t, db.Migrator().HasTable(&ClientAccessCredential{}))
			require.True(t, db.Migrator().HasTable(&ClientAccessGrant{}))
			require.True(t, db.Migrator().HasIndex(&ClientAccessCredential{}, "idx_client_access_credentials_token_hash"))
			require.True(t, db.Migrator().HasIndex(&ClientAccessGrant{}, "uniq_client_access_grant"))
			var channel Channel
			var token Token
			require.NoError(t, db.First(&channel, 901).Error)
			require.NoError(t, db.First(&token, 902).Error)
			require.Equal(t, "legacy-secret", channel.Key)
			require.Equal(t, "legacy-token", token.Key)
		})
	}
}

func TestClientAccessMigrationTokenHashAndGrantTupleAreUnique(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, MigrateClientAccessTables(db))

	first := ClientAccessCredential{Name: "first", TokenHash: "hash", TokenPrefix: "napi_client_", Status: 1, ExpiredTime: -1, Version: 1}
	require.NoError(t, db.Create(&first).Error)
	require.Error(t, db.Create(&ClientAccessCredential{Name: "second", TokenHash: "hash", Status: 1}).Error)

	grant := ClientAccessGrant{CredentialId: first.Id, Scope: "codex.quota.read", ResourceType: "channel", ResourceId: "17"}
	require.NoError(t, db.Create(&grant).Error)
	require.Error(t, db.Create(&ClientAccessGrant{CredentialId: first.Id, Scope: grant.Scope, ResourceType: grant.ResourceType, ResourceId: grant.ResourceId}).Error)
}

func TestClientAccessMigrationPropagatesDatabaseError(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())
	require.Error(t, MigrateClientAccessTables(db))
}
