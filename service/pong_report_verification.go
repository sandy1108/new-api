package service

import (
	"github.com/QuantumNous/new-api/service/reportaccess"
)

// VerificationScopePongReportGrants只用于Root浏览器再次验证，不是PAT Scope。
const VerificationScopePongReportGrants = "pong.report_grants.update"

func normalizePongReportVerification(raw []byte) (any, error) {
	change, err := reportaccess.ParseChange(raw, true)
	if err != nil {
		return nil, ErrVerificationContextInvalid
	}
	return change, nil
}

// 显式列为会话专用操作，配合官方完整性检查；空映射表示PAT始终拒绝，未新增PAT Scope。
func init() { accessTokenVerificationScopes[VerificationScopePongReportGrants] = "" }
