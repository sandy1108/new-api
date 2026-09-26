# New API 客户端访问凭据与 Codex 额度接口实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking. 本项目用户未要求子 Agent 时，选择当前会话内执行。

**Goal:** 增加独立的受限客户端访问凭据，首个白名单 Scope 为 `codex.quota.read`，让客户端安全读取所绑定 Codex 渠道的标准化额度。

**Architecture:** 凭据与资源授权分为两张新增主库表。独立的认证/Scope 中间件只保护新 `/api/client-access/v1` 路由；管理端走 Root 会话认证。额度读取复用现有 WHAM 请求函数，OAuth 刷新协调必须覆盖现有刷新入口，不能只保护新接口。

**Tech Stack:** Go、Gin、GORM、PostgreSQL/SQLite/MySQL、Redis、React、TypeScript、TanStack Router。

**Spec:** `docs/superpowers/specs/2026-09-23-client-access-credentials-and-codex-quota-design.md`

## Global Constraints

- 首期唯一 Scope：`codex.quota.read`；`resource_type=channel`；每凭据最多绑定一个 `constant.ChannelTypeCodex`（57）且非多 Key 的渠道。
- 公共 API：`GET /api/client-access/v1/codex/usage`；客户端不传渠道 ID，不使用管理 PAT 或模型 Token。
- 只新增 `client_access_credentials` 与 `client_access_grants` 两张主库表，不改官方 Token/Channel/Log 结构或生产数据。
- 管理接口首期 `RootAuth()`；创建和轮换时完整 Token 仅展示一次，持久化只存 SHA-256。
- 公共查询在生产和隔离测试环境都要求 Redis；不可用时失败关闭。主库授权判断每次实时读取；成功额度缓存 90 秒；认证后每凭据 30 次/分钟；`last_used_time` 至多每 5 分钟写一次。
- 不修改生产 Compose、生产容器、生产数据库；隔离测试必须使用独立数据目录。提交、推送和生产发布分别申请授权。
- 不把 `codex_channel` 作为资源类型。新增功能优先放独立文件；现有刷新链路的必要修改须附回归证据。

## 文件边界

新增：`model/client_access_credential.go`、`model/client_access_grant.go`、`model/client_access_migration.go`、`middleware/client_access.go`、`service/clientaccess/{scope_registry,credential,grant,cache,limiter,codex_usage,audit}.go`、`controller/{client_access_credential,client_codex_usage}.go`、`router/client-access-router.go`、`web/src/features/client-access-credentials/` 与相应测试。

最小修改：`model/main.go` 迁移调用；`router/api-router.go` 路由注册；`middleware/audit.go` 避免客户端凭据被当成 PAT 查库和记录明文；`controller/codex_usage.go` 委托共享刷新；必要时修改 `service/codex_credential_refresh.go` 以纳入所有现有刷新入口；新增共享 `service/codex_refresh_coordination.go`，避免 `service` 与 `service/clientaccess` 循环导入；Web 路由、导航和生成路由树。不得因便捷而改写原 Token 鉴权或旧管理员接口契约。

## Task 1：隔离基线与迁移模型

**Files:** 新增 `model/client_access_credential.go`、`model/client_access_grant.go`、`model/client_access_migration.go`、对应 `*_test.go`；修改 `model/main.go` 一处调用。

**Interfaces:** `MigrateClientAccessTables(db *gorm.DB) error`；Credential 含 `Id/Name/TokenHash/TokenPrefix/Status/CreatedByUserId/CreatedTime/ExpiredTime/LastUsedTime/RevokedTime/UpdatedTime/Version`；Grant 含 `CredentialId/Scope/ResourceType/ResourceId/CreatedTime/UpdatedTime`。

- [ ] 在 `new-api-development` 确认功能分支、既有脏文件和隔离测试目录；记录基线 `git status`，不清理既有未跟踪文件。
- [ ] 写迁移失败测试：空库两表及唯一索引、二次迁移幂等、历史 Channel/Token 行和结构不变、数据库报错不吞掉；先运行 `go test ./model -run 'TestClientAccess' -count=1` 确认失败。
- [ ] 最小实现两张表及独立迁移注册；主库 DB，不用 LOG_DB；只为子系统字段建索引。
- [ ] 重跑目标测试；增加 PostgreSQL 和 MySQL/SQLite 方言验证，不以 SQLite 通过代替 PostgreSQL 生产兼容。

## Task 2：Scope 白名单、凭据和 Grant 生命周期

**Files:** 新增 `service/clientaccess/scope_registry.go`、`credential.go`、`grant.go` 和测试；必要的 `model` 查询放新增文件。

**Interfaces:** `ScopeDefinition` 描述 Key、显示名、资源类型、最多资源数和 Validator；`codex.quota.read` Validator 重新查询渠道存在、启用、`Type == constant.ChannelTypeCodex`、`IsMultiKey == false` 与必要 OAuth 字段。创建/轮换只返回一次明文，查询只返回脱敏视图。

- [ ] 先写失败测试：未知/通配 Scope、重复 Grant、非 Codex/多 Key/禁用/不存在渠道、空创建 Grant、错误过期时间；一个凭据多个已注册 Scope 的结构化测试可用测试专属 Scope，但生产 Catalog 只注册首期 Scope。
- [ ] 实现 `crypto/rand` 256-bit URL-safe Token、`napi_client_` 前缀、SHA-256 唯一 Hash、事务创建与脱敏查询。服务端自行推导 `resource_type`，不信任请求值。
- [ ] 先写失败测试：Grant 全量替换原子性、空 Grants 后无权限、`version` 旧值返回冲突、轮换后旧 Hash 失效、禁用可启用、撤销不可恢复。
- [ ] 实现版本 CAS 与事务更新，所有失败回滚；列表/详情带版本但不带明文 Token；运行 `go test ./service/clientaccess ./model -count=1`。

## Task 3：独立鉴权、中间件和管理 API

**Files:** 新增 `middleware/client_access.go`、`controller/client_access_credential.go`、`router/client-access-router.go` 与测试；`router/api-router.go` 增加一次注册。

**Interfaces:** `ClientAccessAuth()` 从 Bearer 值计算 Hash、主库读取状态/有效期；`RequireClientScope("codex.quota.read")` 读取实时 Grant 并校验资源；管理接口按设计文档 8.3 节注册，包括 `enable`。

- [ ] 写路由失败测试：无凭据/随机/禁用/过期/撤销均 401，已认证无 Scope 403，资源失效 503，管理路由非 Root 拒绝，客户端 Token 不能用于旧 `/api/token`、`/api/channel` 或 `/v1`。
- [ ] 实现独立中间件；不调用 `TokenAuthReadOnly()`；管理写路由挂 Root 鉴权、关键操作限流与审计；错误不泄露渠道存在性。
- [ ] 实现管理 API 与一次性 Token 返回，列表/详情统一脱敏；`PUT grants` 请求带版本，冲突返回 409；只输出公开 Scope 定义及安全的渠道选择字段。
- [ ] 运行 `go test ./middleware ./router ./controller ./service/clientaccess -count=1`，核对旧路由仍走原鉴权链。

## Task 4：OAuth 刷新协调安全门槛

**Files:** 新增共享 `service/codex_refresh_coordination.go` 和测试；必要时最小修改 `service/codex_credential_refresh.go`、`controller/codex_usage.go`。`service/clientaccess` 仅调用共享函数，不能被 `service` 反向导入。现有 `service/codex_wham_usage.go` 的读取函数只复用，不改变请求契约。

**Interfaces:** 所有已识别入口按渠道 ID 共享刷新协调；锁内重读 Key，写回执行旧 Key 条件更新。失败不覆盖新 Token，不向公共客户端暴露上游响应。

- [ ] 为管理员额度接口、手动刷新、定时刷新、模型列表刷新与新公共查询建立并发测试；先证实当前路径会竞争或测试明确可覆盖其写回调用点。
- [ ] 实现带持有者随机值、过期时间和持有者校验释放的 Redis 锁；刷新前重读 Key；写回以旧 Key 做 CAS，CAS 未命中时重读。锁超时或 Redis 故障时公共查询返回 503，不回退为无锁刷新。
- [ ] 将现有刷新入口最小委托到共享路径，保留旧管理员接口 URL、状态码和响应结构；检查定时任务与模型列表路径不再绕过协调。
- [ ] 运行并发/旧接口回归测试。若统一协调不能被证明，停止公共接口自动刷新并在测试报告中标注功能缺口，不带风险继续发布。

## Task 5：额度 DTO、缓存与客户端接口

**Files:** 新增 `service/clientaccess/codex_usage.go`、`cache.go`、`limiter.go`、`controller/client_codex_usage.go` 与测试；路由注册沿 Task 3 文件。

**Interfaces:** `GET /api/client-access/v1/codex/usage` 仅输出设计文档 9.2 节裁剪 DTO；200 状态仅 `fresh/cached`，未知窗口为 `null`；失败返回 `success:false/code/message`。

- [ ] 写失败测试：5 小时/7 天按明确 `limit_window_seconds` 归类；缺失和未知窗口为 `null`；比例缺失不伪造；任何响应不含 `Channel.Key`、OAuth、Account ID、上游原文或 SQL 错误。
- [ ] 实现底层 `FetchCodexWhamUsage` 复用与标准化。上游 401/403、429、超时、非 JSON 和内部数据库错误按设计文档 9.3 节分类，不返回原始 body。
- [ ] 写失败测试：认证/Grant/资源检查先于缓存命中；撤销后即使缓存有数据也拒绝；90 秒共享缓存、同渠道并发合并、每凭据 30 次/分钟、Redis 失效关闭、`last_used_time` 限频写。
- [ ] 实现上述缓存与限流；生产 Redis 不可用不能放大上游请求。运行 `go test ./service/clientaccess ./controller ./router -count=1`。

## Task 6：Root 管理页面

**Files:** 新增 `web/src/features/client-access-credentials/` 页面、API/types/组件/测试；新增 `web/src/routes/_authenticated/...` 路由；按现有导航结构最小修改 `web/src/hooks/use-sidebar-data.ts` 或实际入口；生成 `web/src/routeTree.gen.ts`。

- [ ] 先看现有设置页、渠道选择器、TanStack Router、权限和 i18n 约定；写页面失败测试：仅 Root 可见入口，Scope 复选、Codex 渠道过滤、一次性 Token 弹窗、禁用/启用/撤销、Grant 版本冲突提示。
- [ ] 以独立 Feature 实现管理列表与表单；前端筛选不是安全边界，所有最终授权以后端 Validator 为准。完整 Token 不进入 localStorage、URL 或日志。
- [ ] 按项目现有脚本生成路由树，不手改生成文件；运行 `web/package.json` 中现有测试、类型检查和构建命令，并记录准确结果。

## Task 7：文档、隔离升级回归与交付门槛

**Files:** 新增 API 文档和 Postman 集合到项目既定目录；更新项目发布/测试记录；不得写真实凭据。

- [ ] 文档写明管理和客户端请求示例、错误码、固定 DTO、轮换/撤销/Grant 更新语义；Postman 只使用占位变量。
- [ ] 在独立 PostgreSQL/Redis 数据目录做空库首次启动、二次启动、带历史数据的升级、含新表后的旧版回滚启动演练。逐项比较旧表行数/关键 schema，保存脱敏证据。
- [ ] 验证 Root 创建、认证态额度读取、缓存、限流、轮换、撤销；复测管理员登录、模型 `/v1`、Responses、原 Codex 管理接口、日志统计、Chrome 插件所依赖接口。
- [ ] 做 `git diff --check`、精确范围 diff 审查和 Git 状态核对；标出既有未跟踪文件。若任何迁移或刷新回归失败，停止进入生产交接包。
- [ ] 用户验收测试环境后，另行制作生产交接包，包含备份、镜像 digest、数据库迁移、回滚、验证和绝对路径；不在本计划执行生产切换或推送。

## 实现后收尾复核（2026-09-26）

本节只跟踪本轮代码审查确认的剩余项，不改变上面的功能范围和生产边界。现有 `model.GetChannelById` 在记录删除时返回 `nil + error`，刷新协调函数已经先检查错误并返回；因此当前源码未证实 CAS 重读后会解引用 nil，不为不可达的 `nil + nil` 情形增加生产分支。

### Task 8：客户端额度错误分类与请求审计

**Files:** 修改 `service/clientaccess/cache.go`、`limiter.go`、`codex_usage.go`、`controller/client_codex_usage.go`、`middleware/client_access.go`、`router/client-access-router.go` 和 `service/codex_refresh_coordination.go`；新增 `middleware/client_access_audit.go` 及对应测试。

**Interfaces:** Redis 故障使用可识别的 sentinel，保持失败关闭并返回 503；渠道/上游已知故障仍返回 503；数据库、JSON 解码等未分类内部错误返回 500。审计采用客户端访问路由独立中间件的结构化应用日志，不新增数据库表、不复用后台 PAT 身份。

- [x] 为“渠道校验数据库故障返回 500”增加 Controller 回归测试；先运行并确认当前失败。
- [x] 为“Redis 不可用返回专用错误码且 HTTP 503”增加回归测试；先运行并确认未满足的断言。
- [x] 用 sentinel 区分 Redis 依赖故障与内部错误；确保缓存读取、写入、限流和 fetch-lock 的 Redis 错误仍为 503，保留未知内部错误 500。
- [x] 为客户端额度路由增加脱敏请求审计测试，覆盖成功、拒绝/服务错误、panic、凭据 ID、Scope、资源指纹和错误类型；断言日志与 panic 响应不含 Authorization、完整资源 ID、panic 原文、OAuth 内容或上游响应。
- [x] 为 OAuth Key CAS 更新增加显式 opt-in 的 SQLite/PostgreSQL/MySQL 方言回归；真实 MySQL 8 复现 `key` 未引用的 SQL 语法错误，改用 GORM `clause.Eq` 后通过 PostgreSQL 15 与 MySQL 8。
- [x] 重跑 `go test -count=1 ./service/clientaccess ./service ./middleware ./controller ./router ./model`；修改的 Go 文件通过 `gofmt`，`git diff --check` 通过。

### Task 9：实现状态与知识记录对齐

**Files:** 修改本计划、技术设计文档、`docs/development/client-access-credentials-api.md` 和 `docs/development/release-log.md`；依项目知识库约定更新对应的 Obsidian New API 项目记录。

- [x] API 文档明确：客户端自身限流为 429；上游限流/Redis/渠道暂不可用为 503；服务端数据库、panic 或其他内部错误为 500，并列出稳定错误码。
- [x] 技术设计文档已按当前实现与验证更新；未做的真实管理端、真实上游和完整隔离运行验收继续标记为待完成。
- [x] 发布日志追加 2026-09-26 本轮复核、修正与验证结果，注明开发分支未提交/未推送/未发布，不能把源码测试写成生产验收。
- [x] 读取 `memory-management` 与 `fast-note-sync-for-obsidian` 技能后，更新现有项目笔记和编码导航并读回确认，未记录任何凭据。

## 执行检查点

1. Task 1～3 完成后复核两表迁移与鉴权安全，发现结构性问题先修正设计。
2. Task 4 是独立安全门槛，未通过不得把自动刷新纳入发布候选。
3. Task 5～7 完成后仅能认定开发/隔离回归状态；生产发布须用户单独批准。
