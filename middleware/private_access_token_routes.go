package middleware

// 私有接口单独声明后台 PAT 边界，避免每次同步上游时修改其路由权限表。
// 客户端额度读取使用独立的 ClientAccessAuth，不属于后台 PAT 路由。
func init() {
	accessTokenRouteRules["GET /api/log/usage-summary"] = accessTokenScopeRule("log:read")
	accessTokenRouteRules["GET /api/log/self/usage-summary"] = accessTokenScopeRule("usage:read")

	for _, key := range []string{
		"GET /api/client-access/scopes",
		"GET /api/client-access/scopes/:scope/resources",
		"GET /api/client-access/credentials",
		"GET /api/client-access/credentials/:id",
		"POST /api/client-access/credentials",
		"PUT /api/client-access/credentials/:id/grants",
		"POST /api/client-access/credentials/:id/rotate",
		"POST /api/client-access/credentials/:id/disable",
		"POST /api/client-access/credentials/:id/enable",
		"POST /api/client-access/credentials/:id/revoke",
	} {
		accessTokenRouteRules[key] = accessTokenSessionRule
	}
}
