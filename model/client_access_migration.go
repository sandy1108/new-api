package model

import "gorm.io/gorm"

// MigrateClientAccessTables 只迁移本功能拥有的独立表，避免触碰官方业务表。
func MigrateClientAccessTables(db *gorm.DB) error {
	return db.AutoMigrate(&ClientAccessCredential{}, &ClientAccessGrant{})
}
