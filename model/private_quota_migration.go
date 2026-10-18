package model

import (
	"crypto/rand"
	"encoding/hex"
	"errors"

	"gorm.io/gorm"
)

// MigratePrivateQuotaTables只拥有四张私有表，不修改官方表和日志库。
func MigratePrivateQuotaTables(db *gorm.DB) error {
	return db.AutoMigrate(&PrivateQuotaSettings{}, &PrivateQuotaSubject{}, &PrivateQuotaSnapshot{}, &PrivateQuotaEvent{})
}

func InitializePrivateQuotaSettings(db *gorm.DB, now int64) (PrivateQuotaSettings, error) {
	if now < 1 {
		return PrivateQuotaSettings{}, ErrPrivateQuotaInputInvalid
	}
	safeDB := privateQuotaSensitiveDB(db)
	settings, err := GetPrivateQuotaSettings(safeDB)
	if err == nil {
		return settings, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return PrivateQuotaSettings{}, err
	}
	// 模块所有后续写入均要求健康单例已存在；有历史却失去单例时必须人工恢复原密钥。
	// 仅探测一行，避免启动时扫描整本历史；也拒绝id非1的异常设置行。
	for _, table := range []any{&PrivateQuotaSettings{}, &PrivateQuotaSubject{}, &PrivateQuotaSnapshot{}, &PrivateQuotaEvent{}} {
		var row struct{ ID int64 }
		result := safeDB.Model(table).Select("id").Limit(1).Find(&row)
		if result.Error != nil {
			return PrivateQuotaSettings{}, result.Error
		}
		if result.RowsAffected > 0 {
			// 另一个初始化者可能刚刚插入合法单例；只复用胜者，绝不覆盖其密钥。
			winner, winnerErr := GetPrivateQuotaSettings(safeDB)
			if winnerErr == nil {
				return winner, nil
			}
			if !errors.Is(winnerErr, gorm.ErrRecordNotFound) {
				return PrivateQuotaSettings{}, winnerErr
			}
			return PrivateQuotaSettings{}, ErrPrivateQuotaStateInvalid
		}
	}
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return PrivateQuotaSettings{}, err
	}
	settings = PrivateQuotaSettings{ID: 1, Enabled: false, IntervalMinutes: 10, Version: 1,
		FingerprintKey: hex.EncodeToString(key), KeyVersion: 1, UpdatedAt: now}
	// 不依赖方言专属upsert。唯一主键负责裁决；在独立语句之后读取胜者，避免
	// PostgreSQL在唯一键错误后事务已终止，或SQLite读事务升级写锁失败的问题。
	if err := safeDB.Create(&settings).Error; err != nil {
		winner, winnerErr := GetPrivateQuotaSettings(safeDB)
		if winnerErr == nil {
			return winner, nil
		}
		return PrivateQuotaSettings{}, err
	}
	return settings, nil
}
