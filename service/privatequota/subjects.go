package privatequota

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/url"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay/channel/codex"
)

var (
	ErrSourceUnavailable = errors.New("private quota source unavailable")
	ErrIdentityChanged   = errors.New("private quota source identity changed")
)

// ChannelFingerprint只接受启用的Codex单密钥渠道；令牌正文与渠道ID不参与身份。
// 不访问网络，也不从JWT猜账号；缺少明确account_id时拒绝绑定。
func ChannelFingerprint(channel *model.Channel, encodedKey string) (string, error) {
	key, err := hex.DecodeString(encodedKey)
	if err != nil || len(key) != 32 {
		return "", model.ErrPrivateQuotaStateInvalid
	}
	if channel == nil || channel.Id < 1 || channel.Type != constant.ChannelTypeCodex ||
		channel.Status != common.ChannelStatusEnabled || channel.ChannelInfo.IsMultiKey {
		return "", ErrSourceUnavailable
	}
	oauth, err := codex.ParseOAuthKey(strings.TrimSpace(channel.Key))
	if err != nil || strings.TrimSpace(oauth.AccountID) == "" || strings.TrimSpace(oauth.AccessToken) == "" {
		return "", ErrSourceUnavailable
	}
	source := constant.GetChannelBaseURL(constant.ChannelTypeCodex)
	if channel.BaseURL != nil && strings.TrimSpace(*channel.BaseURL) != "" {
		source = strings.TrimSpace(*channel.BaseURL)
	}
	u, err := url.Parse(source)
	if err != nil || u.Opaque != "" || u.Hostname() == "" || u.User != nil ||
		u.RawQuery != "" || u.ForceQuery || u.Fragment != "" {
		return "", ErrSourceUnavailable
	}
	u.Scheme = strings.ToLower(u.Scheme)
	if u.Scheme != "https" && u.Scheme != "http" {
		return "", ErrSourceUnavailable
	}
	u.Host = strings.ToLower(u.Host)
	if u.Scheme == "https" && u.Port() == "443" || u.Scheme == "http" && u.Port() == "80" {
		u.Host = u.Hostname()
		if strings.Contains(u.Host, ":") {
			u.Host = "[" + u.Host + "]"
		}
	}
	// 只消除请求路径拼接时本就去掉的末尾斜杠；不擅自合并不同路径、协议或来源。
	source = strings.TrimRight(u.String(), "/")
	identity, err := common.Marshal([3]string{"codex", source, strings.TrimSpace(oauth.AccountID)})
	if err != nil {
		return "", ErrSourceUnavailable
	}
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write(identity)
	return hex.EncodeToString(mac.Sum(nil)), nil
}

// ValidateSubjectSource用于绑定和采集各阶段的身份复核，不修改主体或自动切换来源。
func ValidateSubjectSource(subject model.PrivateQuotaSubject, channel *model.Channel, encodedKey string) error {
	fingerprint, err := ChannelFingerprint(channel, encodedKey)
	if err != nil {
		return err
	}
	if subject.Provider != "codex" || !hmac.Equal([]byte(subject.IdentityFingerprint), []byte(fingerprint)) {
		return ErrIdentityChanged
	}
	return nil
}
