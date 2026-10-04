package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
)

type CodexCredentialRefreshOptions struct {
	ResetCaches            bool
	StaleKey               string
	RequireDistributedLock bool
}

type CodexOAuthKey struct {
	IDToken      string `json:"id_token,omitempty"`
	AccessToken  string `json:"access_token,omitempty"`
	RefreshToken string `json:"refresh_token,omitempty"`

	AccountID   string `json:"account_id,omitempty"`
	LastRefresh string `json:"last_refresh,omitempty"`
	Email       string `json:"email,omitempty"`
	Type        string `json:"type,omitempty"`
	Expired     string `json:"expired,omitempty"`
}

func parseCodexOAuthKey(raw string) (*CodexOAuthKey, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, errors.New("codex channel: empty oauth key")
	}
	var key CodexOAuthKey
	if err := common.Unmarshal([]byte(raw), &key); err != nil {
		return nil, errors.New("codex channel: invalid oauth key json")
	}
	return &key, nil
}

func RefreshCodexChannelCredential(ctx context.Context, channelID int, opts CodexCredentialRefreshOptions) (*CodexOAuthKey, *model.Channel, error) {
	return refreshCodexChannelCredential(ctx, channelID, opts, RefreshCodexOAuthTokenWithProxy)
}

type codexOAuthRefreshFunc func(context.Context, string, string) (*CodexOAuthTokenResult, error)

func refreshCodexChannelCredential(ctx context.Context, channelID int, opts CodexCredentialRefreshOptions, refresh codexOAuthRefreshFunc) (*CodexOAuthKey, *model.Channel, error) {
	ch, err := model.GetChannelById(channelID, true)
	if err != nil {
		return nil, nil, err
	}
	if ch == nil {
		return nil, nil, fmt.Errorf("channel not found")
	}
	if ch.Type != constant.ChannelTypeCodex {
		return nil, nil, fmt.Errorf("channel type is not Codex")
	}
	initialKey := ch.Key
	unlock, err := acquireCodexRefreshLock(ctx, channelID, opts.RequireDistributedLock)
	if err != nil {
		return nil, nil, err
	}
	defer func() {
		if err := unlock(); err != nil {
			common.SysError("codex refresh lock release failed: " + err.Error())
		}
	}()

	// 锁内重读，避免等待期间另一个入口已经轮换了 Refresh Token。
	ch, err = model.GetChannelById(channelID, true)
	if err != nil {
		return nil, nil, err
	}
	if ch == nil || ch.Type != constant.ChannelTypeCodex {
		return nil, nil, fmt.Errorf("codex channel unavailable")
	}
	if ch.Key != initialKey || (opts.StaleKey != "" && ch.Key != opts.StaleKey) {
		latest, err := parseCodexOAuthKey(strings.TrimSpace(ch.Key))
		if err != nil {
			return nil, nil, err
		}
		return latest, ch, nil
	}

	oauthKey, err := parseCodexOAuthKey(strings.TrimSpace(ch.Key))
	if err != nil {
		return nil, nil, err
	}
	if strings.TrimSpace(oauthKey.RefreshToken) == "" {
		return nil, nil, fmt.Errorf("codex channel: refresh_token is required to refresh credential")
	}

	refreshCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	res, err := refresh(refreshCtx, oauthKey.RefreshToken, ch.GetSetting().Proxy)
	if err != nil {
		return nil, nil, err
	}

	oauthKey.AccessToken = res.AccessToken
	oauthKey.RefreshToken = res.RefreshToken
	oauthKey.LastRefresh = time.Now().Format(time.RFC3339)
	oauthKey.Expired = res.ExpiresAt.Format(time.RFC3339)
	if strings.TrimSpace(oauthKey.Type) == "" {
		oauthKey.Type = "codex"
	}

	if strings.TrimSpace(oauthKey.AccountID) == "" {
		if accountID, ok := ExtractCodexAccountIDFromJWT(oauthKey.AccessToken); ok {
			oauthKey.AccountID = accountID
		}
	}
	if strings.TrimSpace(oauthKey.Email) == "" {
		if email, ok := ExtractEmailFromJWT(oauthKey.AccessToken); ok {
			oauthKey.Email = email
		}
	}

	encoded, err := common.Marshal(oauthKey)
	if err != nil {
		return nil, nil, err
	}

	written, err := persistCodexKeyIfUnchanged(model.DB, ch.Id, ch.Key, string(encoded))
	if err != nil {
		return nil, nil, err
	}
	if !written {
		latestChannel, err := model.GetChannelById(channelID, true)
		if err != nil {
			return nil, nil, err
		}
		latestKey, err := parseCodexOAuthKey(strings.TrimSpace(latestChannel.Key))
		if err != nil {
			return nil, nil, err
		}
		return latestKey, latestChannel, nil
	}

	if opts.ResetCaches {
		model.InitChannelCache()
	}

	return oauthKey, ch, nil
}
