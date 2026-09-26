package model

// ClientAccessGrant 表达一个已注册 Scope 对一个明确资源的授权。
type ClientAccessGrant struct {
	Id           int    `json:"id" gorm:"primaryKey"`
	CredentialId int    `json:"credential_id" gorm:"not null;index;uniqueIndex:uniq_client_access_grant"`
	Scope        string `json:"scope" gorm:"type:varchar(128);not null;index;uniqueIndex:uniq_client_access_grant"`
	ResourceType string `json:"resource_type" gorm:"type:varchar(64);not null;uniqueIndex:uniq_client_access_grant"`
	ResourceId   string `json:"resource_id" gorm:"type:varchar(128);not null;uniqueIndex:uniq_client_access_grant"`
	CreatedTime  int64  `json:"created_time" gorm:"bigint;not null"`
	UpdatedTime  int64  `json:"updated_time" gorm:"bigint;not null"`
}

func (ClientAccessGrant) TableName() string { return "client_access_grants" }
