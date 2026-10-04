package clientaccess

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/model"
	"gorm.io/gorm"
)

const (
	ClientTokenPrefix        = "napi_client_"
	CredentialStatusEnabled  = 1
	CredentialStatusDisabled = 2
	CredentialStatusRevoked  = 3
)

var ErrVersionConflict = errors.New("credential version conflict")
var ErrCredentialUnavailable = errors.New("credential unavailable")
var ErrInvalidInput = errors.New("invalid client access input")

type Store struct{ DB *gorm.DB }

func NewStore(db *gorm.DB) *Store { return &Store{DB: db} }

type GrantInput struct {
	Scope      string `json:"scope"`
	ResourceID string `json:"resource_id"`
}

type CreateInput struct {
	Name        string       `json:"name"`
	ExpiredTime int64        `json:"expired_time"`
	Grants      []GrantInput `json:"grants"`
}

type IssuedCredential struct {
	Credential model.ClientAccessCredential `json:"credential"`
	Token      string                       `json:"token"`
}

func generateToken() (string, error) {
	var random [32]byte
	if _, err := rand.Read(random[:]); err != nil {
		return "", err
	}
	return ClientTokenPrefix + base64.RawURLEncoding.EncodeToString(random[:]), nil
}

func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// CreateCredential 将凭据哈希和全部授权作为一个事务提交；明文仅在提交成功后返回。
func (store *Store) CreateCredential(createdBy int, input CreateInput) (*IssuedCredential, error) {
	name := strings.TrimSpace(input.Name)
	if store == nil || store.DB == nil || createdBy <= 0 || name == "" || len(name) > 128 {
		return nil, ErrInvalidInput
	}
	now := time.Now().Unix()
	if input.ExpiredTime != -1 && input.ExpiredTime <= now {
		return nil, ErrInvalidInput
	}
	token, err := generateToken()
	if err != nil {
		return nil, err
	}
	credential := model.ClientAccessCredential{
		Name: name, TokenHash: HashToken(token), TokenPrefix: token[:24],
		Status: CredentialStatusEnabled, CreatedByUserId: createdBy,
		CreatedTime: now, UpdatedTime: now, ExpiredTime: input.ExpiredTime, Version: 1,
	}
	err = store.DB.Transaction(func(tx *gorm.DB) error {
		grants, err := validateGrantInputs(tx, input.Grants, false)
		if err != nil {
			return err
		}
		if err := tx.Create(&credential).Error; err != nil {
			return err
		}
		for i := range grants {
			grants[i].CredentialId = credential.Id
			grants[i].CreatedTime = now
			grants[i].UpdatedTime = now
		}
		return tx.Create(&grants).Error
	})
	if err != nil {
		return nil, err
	}
	return &IssuedCredential{Credential: credential, Token: token}, nil
}

// FindCredentialByToken 每次从主库读取，保证事务提交后的禁用和轮换对新请求生效。
func (store *Store) FindCredentialByToken(token string) (*model.ClientAccessCredential, error) {
	if !strings.HasPrefix(token, ClientTokenPrefix) || store == nil || store.DB == nil {
		return nil, ErrCredentialUnavailable
	}
	var credential model.ClientAccessCredential
	if err := store.DB.Where("token_hash = ?", HashToken(token)).First(&credential).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrCredentialUnavailable
		}
		return nil, err
	}
	if credential.Status != CredentialStatusEnabled || (credential.ExpiredTime != -1 && credential.ExpiredTime <= time.Now().Unix()) {
		return nil, ErrCredentialUnavailable
	}
	return &credential, nil
}

// TouchLastUsed 使用数据库条件更新，跨实例同时使用也不会每请求写一行。
func (store *Store) TouchLastUsed(id int, at int64) error {
	return store.DB.Model(&model.ClientAccessCredential{}).
		Where("id = ? AND last_used_time <= ?", id, at-300).
		Update("last_used_time", at).Error
}

func (store *Store) RotateCredential(id int, expectedVersion int64) (*IssuedCredential, error) {
	token, err := generateToken()
	if err != nil {
		return nil, err
	}
	now := time.Now().Unix()
	var credential model.ClientAccessCredential
	err = store.DB.Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&model.ClientAccessCredential{}).
			Where("id = ? AND version = ? AND status <> ?", id, expectedVersion, CredentialStatusRevoked).
			Updates(map[string]any{
				"token_hash": HashToken(token), "token_prefix": token[:24],
				"updated_time": now, "version": gorm.Expr("version + 1"),
			})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ErrVersionConflict
		}
		return tx.First(&credential, id).Error
	})
	if err != nil {
		return nil, err
	}
	return &IssuedCredential{Credential: credential, Token: token}, nil
}

// SetStatus 将撤销视为终态；每次状态修改都消耗乐观版本。
func (store *Store) SetStatus(id int, expectedVersion int64, next int) (*model.ClientAccessCredential, error) {
	if next != CredentialStatusEnabled && next != CredentialStatusDisabled && next != CredentialStatusRevoked {
		return nil, errors.New("invalid credential status")
	}
	now := time.Now().Unix()
	var credential model.ClientAccessCredential
	err := store.DB.Transaction(func(tx *gorm.DB) error {
		changes := map[string]any{"status": next, "updated_time": now, "version": gorm.Expr("version + 1")}
		if next == CredentialStatusRevoked {
			changes["revoked_time"] = now
		}
		result := tx.Model(&model.ClientAccessCredential{}).
			Where("id = ? AND version = ? AND status <> ?", id, expectedVersion, CredentialStatusRevoked).
			Updates(changes)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ErrVersionConflict
		}
		return tx.First(&credential, id).Error
	})
	if err != nil {
		return nil, err
	}
	return &credential, nil
}
