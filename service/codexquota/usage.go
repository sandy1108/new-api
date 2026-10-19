// Package codexquota提供无网络、无身份数据的额度响应解析，供不同私有功能共享。
package codexquota

import (
	"errors"
	"github.com/QuantumNous/new-api/common"
)

type Window struct {
	UsedPercent        *float64 `json:"used_percent"`
	LimitWindowSeconds int64    `json:"limit_window_seconds"`
	ResetAt            *int64   `json:"reset_at"`
}

func Decode(body []byte) ([]*Window, error) {
	var data struct {
		RateLimit *struct {
			Primary   *Window `json:"primary_window"`
			Secondary *Window `json:"secondary_window"`
		} `json:"rate_limit"`
	}
	if err := common.Unmarshal(body, &data); err != nil {
		return nil, err
	}
	if data.RateLimit == nil {
		return nil, errors.New("missing codex rate limit")
	}
	return []*Window{data.RateLimit.Primary, data.RateLimit.Secondary}, nil
}
