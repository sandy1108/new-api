package clientaccess

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay/channel/codex"
	"gorm.io/gorm"
)

const ScopeCodexQuotaRead = "codex.quota.read"

var ErrInvalidGrant = errors.New("invalid client access grant")

type ScopeDefinition struct {
	Key          string `json:"key"`
	DisplayName  string `json:"display_name"`
	Description  string `json:"description"`
	ResourceType string `json:"resource_type"`
	MaxResources int    `json:"max_resources"`
	RiskLevel    string `json:"risk_level"`
	validator    func(*gorm.DB, string) error
}

type ResourceOption struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	Type int    `json:"type"`
}

var scopeCatalog = map[string]ScopeDefinition{
	ScopeCodexQuotaRead: {
		Key: ScopeCodexQuotaRead, DisplayName: "Codex 额度读取",
		Description:  "读取一个 ChatGPT Subscription (Codex) 渠道的额度",
		ResourceType: "channel", MaxResources: 1, RiskLevel: "read_only",
		validator: validateCodexChannel,
	},
}

// ScopeCatalog 只暴露已注册 Scope 的公开元数据，不暴露校验实现。
func ScopeCatalog() []ScopeDefinition {
	definitions := make([]ScopeDefinition, 0, len(scopeCatalog))
	for _, definition := range scopeCatalog {
		definitions = append(definitions, definition)
	}
	sort.Slice(definitions, func(i, j int) bool { return definitions[i].Key < definitions[j].Key })
	return definitions
}

// ListScopeResources 仅向 Root 管理界面返回安全的候选字段；最终绑定仍需重新校验。
func (store *Store) ListScopeResources(scope string) ([]ResourceOption, error) {
	if scope != ScopeCodexQuotaRead {
		return nil, ErrScopeDenied
	}
	var channels []model.Channel
	if err := store.DB.Where("type = ? AND status = ?", constant.ChannelTypeCodex, common.ChannelStatusEnabled).
		Order("id ASC").Find(&channels).Error; err != nil {
		return nil, err
	}
	options := make([]ResourceOption, 0, len(channels))
	for _, channel := range channels {
		if channel.ChannelInfo.IsMultiKey {
			continue
		}
		key, err := codex.ParseOAuthKey(strings.TrimSpace(channel.Key))
		if err != nil || strings.TrimSpace(key.AccessToken) == "" || strings.TrimSpace(key.AccountID) == "" {
			continue
		}
		options = append(options, ResourceOption{ID: channel.Id, Name: channel.Name, Type: channel.Type})
	}
	return options, nil
}

func validateCodexChannel(db *gorm.DB, resourceID string) error {
	id, err := strconv.Atoi(resourceID)
	if err != nil || id <= 0 || strconv.Itoa(id) != resourceID {
		return ErrResourceUnavailable
	}
	var channel model.Channel
	if err := db.First(&channel, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrResourceUnavailable
		}
		return err
	}
	if channel.Type != constant.ChannelTypeCodex || channel.Status != common.ChannelStatusEnabled || channel.ChannelInfo.IsMultiKey {
		return ErrResourceUnavailable
	}
	oauthKey, err := codex.ParseOAuthKey(strings.TrimSpace(channel.Key))
	if err != nil || strings.TrimSpace(oauthKey.AccessToken) == "" || strings.TrimSpace(oauthKey.AccountID) == "" {
		return ErrResourceUnavailable
	}
	return nil
}

func validateGrantInputs(db *gorm.DB, inputs []GrantInput, allowEmpty bool) ([]model.ClientAccessGrant, error) {
	if !allowEmpty && len(inputs) == 0 {
		return nil, ErrInvalidGrant
	}
	seen := make(map[string]struct{}, len(inputs))
	counts := make(map[string]int)
	grants := make([]model.ClientAccessGrant, 0, len(inputs))
	for _, input := range inputs {
		definition, ok := scopeCatalog[input.Scope]
		if !ok {
			return nil, ErrInvalidGrant
		}
		counts[input.Scope]++
		if counts[input.Scope] > definition.MaxResources {
			return nil, fmt.Errorf("%w: too many resources for scope %s", ErrInvalidGrant, input.Scope)
		}
		key := input.Scope + "\x00" + input.ResourceID
		if _, duplicate := seen[key]; duplicate {
			return nil, ErrInvalidGrant
		}
		seen[key] = struct{}{}
		if err := definition.validator(db, input.ResourceID); err != nil {
			if errors.Is(err, ErrResourceUnavailable) {
				return nil, ErrInvalidGrant
			}
			return nil, err
		}
		grants = append(grants, model.ClientAccessGrant{
			Scope: input.Scope, ResourceType: definition.ResourceType, ResourceId: input.ResourceID,
		})
	}
	return grants, nil
}
