package clientaccess

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay/channel/codex"
	"github.com/QuantumNous/new-api/service"
)

var ErrCodexUpstreamAuth = errors.New("codex upstream authentication unavailable")
var ErrCodexUpstreamLimited = errors.New("codex upstream rate limited")
var ErrCodexUpstreamUnavailable = errors.New("codex upstream unavailable")

type UsageWindow struct {
	UsedPercent      *float64 `json:"used_percent"`
	RemainingPercent *float64 `json:"remaining_percent"`
	WindowSeconds    int64    `json:"window_seconds"`
	ResetAt          *string  `json:"reset_at"`
}

type UsageWindows struct {
	FiveHour *UsageWindow `json:"five_hour"`
	SevenDay *UsageWindow `json:"seven_day"`
}

type CodexUsageData struct {
	Provider  string       `json:"provider"`
	QuotaKind string       `json:"quota_kind"`
	Status    string       `json:"status"`
	Windows   UsageWindows `json:"windows"`
	FetchedAt string       `json:"fetched_at"`
}

type upstreamWindow struct {
	UsedPercent        *float64 `json:"used_percent"`
	LimitWindowSeconds int64    `json:"limit_window_seconds"`
	ResetAt            *int64   `json:"reset_at"`
}

// NormalizeCodexUsage 只解析白名单字段；未知窗口不猜测为 5 小时或 7 天。
func NormalizeCodexUsage(body []byte, fetchedAt time.Time) (CodexUsageData, error) {
	var upstream struct {
		RateLimit *struct {
			PrimaryWindow   *upstreamWindow `json:"primary_window"`
			SecondaryWindow *upstreamWindow `json:"secondary_window"`
		} `json:"rate_limit"`
	}
	if err := common.Unmarshal(body, &upstream); err != nil {
		return CodexUsageData{}, err
	}
	if upstream.RateLimit == nil {
		return CodexUsageData{}, ErrCodexUpstreamUnavailable
	}
	data := CodexUsageData{
		Provider: "codex", QuotaKind: "subscription_codex", Status: "fresh",
		FetchedAt: fetchedAt.UTC().Format(time.RFC3339),
	}
	for _, source := range []*upstreamWindow{upstream.RateLimit.PrimaryWindow, upstream.RateLimit.SecondaryWindow} {
		if source == nil {
			continue
		}
		var target **UsageWindow
		switch source.LimitWindowSeconds {
		case 18000:
			target = &data.Windows.FiveHour
		case 604800:
			target = &data.Windows.SevenDay
		default:
			continue
		}
		if *target != nil {
			continue
		}
		window := &UsageWindow{UsedPercent: source.UsedPercent, WindowSeconds: source.LimitWindowSeconds}
		if source.UsedPercent != nil {
			remaining := max(0, min(100, 100-*source.UsedPercent))
			window.RemainingPercent = &remaining
		}
		if source.ResetAt != nil && *source.ResetAt > 0 {
			reset := time.Unix(*source.ResetAt, 0).UTC().Format(time.RFC3339)
			window.ResetAt = &reset
		}
		*target = window
	}
	return data, nil
}

// GetCodexUsage 每次先核验渠道，再读缓存；缓存未命中时用渠道级锁合并上游请求。
func (store *Store) GetCodexUsage(ctx context.Context, channelID int) (*CodexUsageData, error) {
	if err := validateCodexChannel(store.DB, strconv.Itoa(channelID)); err != nil {
		if errors.Is(err, ErrResourceUnavailable) {
			return nil, ErrResourceUnavailable
		}
		return nil, err
	}
	if cached, err := GetCachedCodexUsage(ctx, channelID); err != nil || cached != nil {
		return cached, err
	}
	unlock, err := acquireCodexUsageFetchLock(ctx, channelID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = unlock() }()
	if cached, err := GetCachedCodexUsage(ctx, channelID); err != nil || cached != nil {
		return cached, err
	}
	var channel model.Channel
	if err := store.DB.First(&channel, "id = ?", channelID).Error; err != nil {
		return nil, err
	}
	oauthKey, err := codex.ParseOAuthKey(strings.TrimSpace(channel.Key))
	if err != nil {
		return nil, ErrResourceUnavailable
	}
	client, err := service.GetHttpClientWithProxy(channel.GetSetting().Proxy)
	if err != nil {
		return nil, ErrCodexUpstreamUnavailable
	}
	fetch := func(ch *model.Channel, accessToken, accountID string) (int, []byte, error) {
		fetchCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
		defer cancel()
		return service.FetchCodexWhamUsage(fetchCtx, client, ch.GetBaseURL(), accessToken, accountID)
	}
	status, body, err := fetch(&channel, oauthKey.AccessToken, oauthKey.AccountID)
	if err != nil {
		return nil, ErrCodexUpstreamUnavailable
	}
	if status == http.StatusUnauthorized || status == http.StatusForbidden {
		if strings.TrimSpace(oauthKey.RefreshToken) == "" {
			return nil, ErrCodexUpstreamAuth
		}
		refreshed, latestChannel, refreshErr := service.RefreshCodexChannelCredential(ctx, channelID,
			service.CodexCredentialRefreshOptions{ResetCaches: true, StaleKey: channel.Key, RequireDistributedLock: true})
		if refreshErr != nil {
			return nil, ErrCodexUpstreamAuth
		}
		status, body, err = fetch(latestChannel, refreshed.AccessToken, refreshed.AccountID)
		if err != nil {
			return nil, ErrCodexUpstreamUnavailable
		}
	}
	if status == http.StatusTooManyRequests {
		return nil, ErrCodexUpstreamLimited
	}
	if status == http.StatusUnauthorized || status == http.StatusForbidden {
		return nil, ErrCodexUpstreamAuth
	}
	if status < 200 || status >= 300 {
		return nil, ErrCodexUpstreamUnavailable
	}
	data, err := NormalizeCodexUsage(body, time.Now())
	if err != nil {
		return nil, ErrCodexUpstreamUnavailable
	}
	if err := PutCachedCodexUsage(ctx, channelID, data); err != nil {
		return nil, err
	}
	return &data, nil
}
