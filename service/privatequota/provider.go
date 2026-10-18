package privatequota

import (
	"context"
	"errors"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay/channel/codex"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/service/codexquota"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var (
	ErrUpstreamLimited     = errors.New("private quota upstream limited")
	ErrUpstreamAuth        = errors.New("private quota upstream authentication failed")
	ErrUpstreamUnavailable = errors.New("private quota upstream unavailable")
	ErrInvalidResponse     = errors.New("private quota invalid response")
)

// ParseObservations只接受明确窗口；缺失数值保留未知，越界数值交给账本记录异常证据。
func ParseObservations(body []byte) ([]Observation, error) {
	windows, err := codexquota.Decode(body)
	if err != nil {
		return nil, ErrInvalidResponse
	}
	rows := make([]Observation, 0, 2)
	seen := make(map[int64]bool)
	for _, window := range windows {
		if window == nil {
			continue
		}
		if window.LimitWindowSeconds != SevenDaySeconds && window.LimitWindowSeconds != FiveHourSeconds || seen[window.LimitWindowSeconds] {
			return nil, ErrInvalidResponse
		}
		seen[window.LimitWindowSeconds] = true
		row := Observation{WindowSeconds: window.LimitWindowSeconds}
		if window.ResetAt != nil {
			row.ResetAt = *window.ResetAt
		}
		if window.UsedPercent != nil {
			value := *window.UsedPercent
			if math.IsNaN(value) || math.IsInf(value, 0) || value < -10000 || value > 10000 {
				return nil, ErrInvalidResponse
			}
			basisPoints := int(math.Round(value * 100))
			row.UsedBasisPoints = &basisPoints
		}
		rows = append(rows, row)
	}
	if len(rows) == 0 {
		return nil, ErrInvalidResponse
	}
	return rows, nil
}

func RetryAfterSeconds(headers http.Header, now time.Time) int64 {
	value := strings.TrimSpace(headers.Get("Retry-After"))
	if seconds, err := strconv.ParseInt(value, 10, 64); err == nil && seconds >= 0 && seconds <= 31536000 {
		return seconds
	}
	if until, err := http.ParseTime(value); err == nil {
		return min(31536000, max(0, int64(math.Ceil(until.Sub(now).Seconds()))))
	}
	return 0
}

// FetchFresh绕过客户端额度缓存，只读usage；绝不调用兑换/重置额度接口。
// 每次请求及刷新后都复核来源身份，OAuth刷新复用官方分布式锁。
func (store *Store) FetchFresh(ctx context.Context, subject model.PrivateQuotaSubject, fingerprintKey string) ([]Observation, int64, error) {
	var channel model.Channel
	read := func() error {
		err := store.DB.WithContext(ctx).Session(&gorm.Session{Logger: logger.Default.LogMode(logger.Silent)}).First(&channel, "id = ?", subject.SourceChannelID).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrSourceUnavailable
		}
		if err != nil {
			return err
		}
		return ValidateSubjectSource(subject, &channel, fingerprintKey)
	}
	if err := read(); err != nil {
		return nil, 0, err
	}
	for attempt := range 2 {
		oauth, err := codex.ParseOAuthKey(strings.TrimSpace(channel.Key))
		if err != nil {
			return nil, 0, ErrSourceUnavailable
		}
		client, err := service.GetHttpClientWithProxy(channel.GetSetting().Proxy)
		if err != nil {
			return nil, 0, ErrUpstreamUnavailable
		}
		fetchCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
		status, body, headers, err := service.FetchCodexWhamUsageWithHeaders(fetchCtx, client, channel.GetBaseURL(), oauth.AccessToken, oauth.AccountID)
		cancel()
		if ctx.Err() != nil {
			return nil, 0, ctx.Err()
		}
		if err != nil {
			return nil, 0, ErrUpstreamUnavailable
		}
		if status == http.StatusTooManyRequests {
			return nil, RetryAfterSeconds(headers, store.now()), ErrUpstreamLimited
		}
		if status == http.StatusUnauthorized || status == http.StatusForbidden {
			if attempt != 0 || strings.TrimSpace(oauth.RefreshToken) == "" {
				return nil, 0, ErrUpstreamAuth
			}
			if err := read(); err != nil {
				return nil, 0, err
			}
			_, _, err = service.RefreshCodexChannelCredential(ctx, channel.Id, service.CodexCredentialRefreshOptions{ResetCaches: true, StaleKey: channel.Key, RequireDistributedLock: true})
			if err != nil {
				return nil, 0, ErrUpstreamAuth
			}
			if err := read(); err != nil {
				return nil, 0, err
			}
			continue
		}
		if status < 200 || status >= 300 {
			return nil, RetryAfterSeconds(headers, store.now()), ErrUpstreamUnavailable
		}
		if err := read(); err != nil {
			return nil, 0, err
		}
		rows, err := ParseObservations(body)
		return rows, 0, err
	}
	return nil, 0, ErrUpstreamAuth
}
