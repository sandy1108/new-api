package clientaccess

import (
	"errors"
	"time"

	"github.com/QuantumNous/new-api/model"
	"gorm.io/gorm"
)

var ErrScopeDenied = errors.New("scope not granted")
var ErrResourceUnavailable = errors.New("authorized resource unavailable")

func (store *Store) GetGrants(credentialID int) ([]model.ClientAccessGrant, error) {
	var grants []model.ClientAccessGrant
	err := store.DB.Where("credential_id = ?", credentialID).Order("id ASC").Find(&grants).Error
	return grants, err
}

// AuthorizeScope 不缓存授权；每次请求重新核验 Grant 和渠道实际状态。
func (store *Store) AuthorizeScope(credentialID int, scope string) (string, error) {
	definition, known := scopeCatalog[scope]
	if !known {
		return "", ErrScopeDenied
	}
	var grants []model.ClientAccessGrant
	if err := store.DB.Where("credential_id = ? AND scope = ?", credentialID, scope).Find(&grants).Error; err != nil {
		return "", err
	}
	if len(grants) == 0 {
		return "", ErrScopeDenied
	}
	if len(grants) != 1 || grants[0].ResourceType != definition.ResourceType {
		return "", ErrResourceUnavailable
	}
	if err := definition.validator(store.DB, grants[0].ResourceId); err != nil {
		if errors.Is(err, ErrResourceUnavailable) {
			return "", ErrResourceUnavailable
		}
		return "", err
	}
	return grants[0].ResourceId, nil
}

// UpdateGrants 原子替换全部授权；版本条件防止管理员并发编辑时丢失更新。
func (store *Store) UpdateGrants(id int, expectedVersion int64, inputs []GrantInput) (*model.ClientAccessCredential, error) {
	now := time.Now().Unix()
	var credential model.ClientAccessCredential
	err := store.DB.Transaction(func(tx *gorm.DB) error {
		grants, err := validateGrantInputs(tx, inputs, true)
		if err != nil {
			return err
		}
		result := tx.Model(&model.ClientAccessCredential{}).
			Where("id = ? AND version = ? AND status <> ?", id, expectedVersion, CredentialStatusRevoked).
			Updates(map[string]any{"updated_time": now, "version": gorm.Expr("version + 1")})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ErrVersionConflict
		}
		if err := tx.Where("credential_id = ?", id).Delete(&model.ClientAccessGrant{}).Error; err != nil {
			return err
		}
		for i := range grants {
			grants[i].CredentialId = id
			grants[i].CreatedTime = now
			grants[i].UpdatedTime = now
		}
		if len(grants) > 0 {
			if err := tx.Create(&grants).Error; err != nil {
				return err
			}
		}
		return tx.First(&credential, id).Error
	})
	if err != nil {
		return nil, err
	}
	return &credential, nil
}
