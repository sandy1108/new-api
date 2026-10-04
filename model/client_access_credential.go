package model

// ClientAccessCredential 仅保存客户端凭据的哈希和生命周期，不保存明文 Token 或具体授权。
type ClientAccessCredential struct {
	Id              int    `json:"id" gorm:"primaryKey"`
	Name            string `json:"name" gorm:"type:varchar(128);not null"`
	TokenHash       string `json:"-" gorm:"type:varchar(64);not null;uniqueIndex:idx_client_access_credentials_token_hash"`
	TokenPrefix     string `json:"token_prefix" gorm:"type:varchar(32);not null"`
	Status          int    `json:"status" gorm:"not null;default:1;index"`
	CreatedByUserId int    `json:"created_by_user_id" gorm:"not null;index"`
	CreatedTime     int64  `json:"created_time" gorm:"bigint;not null"`
	ExpiredTime     int64  `json:"expired_time" gorm:"bigint;not null;default:-1"`
	LastUsedTime    int64  `json:"last_used_time" gorm:"bigint;not null;default:0"`
	LastUsedIp      string `json:"last_used_ip" gorm:"type:varchar(64)"`
	RevokedTime     int64  `json:"revoked_time" gorm:"bigint;not null;default:0"`
	UpdatedTime     int64  `json:"updated_time" gorm:"bigint;not null"`
	Version         int64  `json:"version" gorm:"bigint;not null;default:1"`
}

func (ClientAccessCredential) TableName() string { return "client_access_credentials" }
