# 客户端访问凭据与 Codex 额度接口

> 状态：本功能已在 rc.40 私有版本部署生产并经用户验收；rc.41 同步正在隔离测试，尚未发布生产。本文中的 URL 应替换为实际目标环境地址；不要把真实凭据写入仓库、截图或命令历史。

## 不同身份不能混用

| 身份 | 用途 | 本功能是否接受 |
| --- | --- | --- |
| Root 后台登录会话 | 管理客户端凭据 | 管理接口接受；rc.41 起仅限会话 |
| 官方 `nap_...` 后台 PAT | 按官方 Scope 访问后台 API | rc.41 私有凭据管理接口不接受 |
| `napi_client_...` 客户端访问凭据 | 调用白名单公共查询 | 仅 `/api/client-access/v1/*` 接受 |
| `/v1` 模型 API Key | 模型调用 | 不接受 |

管理页面位于「系统设置 → 客户端访问凭据」。创建/轮换时完整凭据只返回一次；后台只保存 SHA-256 哈希。`codex.quota.read` 是首期唯一 Scope，每个凭据最多绑定一个已启用、非多 Key、带 OAuth 凭据的官方 Codex 类型渠道。前端筛选只是辅助，后端每次请求重新校验授权与渠道状态。

## Root 管理接口

均使用共享后台登录会话（当前前端通过会话 JWT 调用，刷新使用 Cookie）；不是客户端访问凭据。rc.41 起这些管理路由声明为 session-only，后台 PAT 即使具有官方 Scope 也不能管理私有凭据；公共额度查询仍单独使用 `ClientAccessAuth`。响应格式为 `{ "success": true, "data": ... }`，失败时包含稳定的 `code`。

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| GET | `/api/client-access/scopes` | 可授权 Scope 元数据 |
| GET | `/api/client-access/scopes/:scope/resources` | 指定 Scope 的安全资源候选，仅含 ID、名称、类型 |
| GET | `/api/client-access/credentials?page=1&limit=50` | 分页列表，只含脱敏前缀 |
| GET | `/api/client-access/credentials/:id` | 单个凭据及当前 Grants，不含明文/哈希 |
| POST | `/api/client-access/credentials` | 创建；本次响应含完整 `token` |
| PUT | `/api/client-access/credentials/:id/grants` | 按 `version` 原子替换全部 Grants；版本冲突 409 |
| POST | `/api/client-access/credentials/:id/rotate` | 按 `version` 轮换；本次响应含新 `token`，旧 Token 立即失效 |
| POST | `/api/client-access/credentials/:id/disable` | 按 `version` 禁用 |
| POST | `/api/client-access/credentials/:id/enable` | 按 `version` 启用 |
| POST | `/api/client-access/credentials/:id/revoke` | 按 `version` 永久撤销，不可重新启用 |

创建请求示例：

```json
{
  "name": "AI Dashboard",
  "expired_time": -1,
  "grants": [{ "scope": "codex.quota.read", "resource_id": "17" }]
}
```

`expired_time` 是 Unix 秒；`-1` 表示长期有效。修改 Grants 时传入详情响应中的 `credential.version` 和完整的新 `grants` 数组。轮换、启停、撤销的请求体均为 `{ "version": 1 }`，实际版本须取最新详情/列表值。不要用旧版本覆盖别的管理员的更新。

## 客户端公共查询

```http
GET /api/client-access/v1/codex/usage
Authorization: Bearer napi_client_<创建或轮换时取得的完整凭据>
```

不传渠道 ID，也不接受 query 参数形式的 Token；服务端使用此凭据绑定的唯一渠道。成功响应示意：

```json
{
  "success": true,
  "data": {
    "provider": "codex",
    "quota_kind": "subscription_codex",
    "status": "fresh",
    "windows": {
      "five_hour": { "used_percent": 12.5, "remaining_percent": 87.5, "window_seconds": 18000, "reset_at": "2026-09-25T12:00:00Z" },
      "seven_day": null
    },
    "fetched_at": "2026-09-25T08:00:00Z"
  }
}
```

`status` 可为 `fresh` 或 `cached`，缓存上限 90 秒。无法识别的窗口返回 `null`；不把缺失额度推断为 0 或 100%。不会返回上游 OAuth Token、账号 ID、原始响应或错误正文。每凭据限流 30 次/分钟，缓存与限流都依赖 Redis；Redis 不可用返回 503，不直接放行到上游。

错误状态按来源区分：

| HTTP | `code` | 含义 |
| --- | --- | --- |
| 401 | `CLIENT_ACCESS_INVALID_CREDENTIAL` | 缺少、无效、停用、撤销或过期的客户端访问凭据 |
| 403 | `CLIENT_ACCESS_SCOPE_DENIED` | 凭据未获路由要求的 Scope |
| 429 | `CLIENT_ACCESS_RATE_LIMITED` | 客户端凭据自身超过 30 次/分钟限制 |
| 500 | `CLIENT_ACCESS_INTERNAL_ERROR` | 服务端数据库、panic 或未分类内部异常；响应不含内部错误详情或 panic 原文 |
| 503 | `CLIENT_ACCESS_RESOURCE_UNAVAILABLE` | 绑定渠道不存在、禁用或 OAuth 配置不完整 |
| 503 | `CLIENT_ACCESS_REDIS_UNAVAILABLE` | 限流、缓存或跨实例协调所依赖的 Redis 不可用 |
| 503 | `CODEX_UPSTREAM_AUTH_UNAVAILABLE` | 上游认证失败或 OAuth 刷新无法安全完成 |
| 503 | `CODEX_UPSTREAM_RATE_LIMITED` | Codex 上游限流，不等于客户端自身 429 限流 |
| 503 | `CODEX_UPSTREAM_UNAVAILABLE` | 上游超时或暂时不可用 |
| 503 | `CODEX_USAGE_FETCH_BUSY` | 等待同一渠道的额度抓取锁超时 |

管理端并发更新版本冲突为 HTTP 409、`code=CLIENT_ACCESS_VERSION_CONFLICT`。客户端应按 HTTP 状态处理，不应在 429/503 时高频重试。

客户端查询的结构化审计日志只记录凭据 ID、Scope、路由模板、HTTP 结果、稳定错误类型、耗时和按凭据 Hash 计算的渠道资源指纹；不记录 Authorization、查询参数、响应正文或 OAuth 内容。Gin 访问日志对 `/api/client-access/` 路由会隐藏整个查询串；调用方仍应只通过 Authorization 头传凭据。

## 部署及安全边界

- 私有表仅为 `client_access_credentials` 和 `client_access_grants`，不在官方 Token/Channel 表加字段。
- 迁移前先备份生产数据库；首次发布只能先在隔离测试环境验证，生产切换需另行批准。
- 生成的完整客户端凭据仅保存于客户端安全存储；不放 URL、浏览器 localStorage、日志或 Postman 导出文件。
- 测试客户端凭据不能用于 `/api/token`、`/api/channel` 或 `/v1`；旧后台 PAT 入口会快速拒绝 `napi_client_` 前缀。
