# New API 客户端访问凭据与 Codex 额度接口技术设计

## 设计状态

本文是“公共只读访问与 Codex 额度查询”V2 需求的正式技术设计。这里的“公共”指面向持有受限凭据的客户端，并非匿名公开。

- 当前阶段：核心实现、隔离测试环境中的真实 Root 管理操作及独立测试渠道的 Codex 上游取数已验证；正式候选镜像、生产备份与发布交接仍待完成
- 数据边界：仅使用独立客户端访问凭据与 Grant 表；未修改官方 Token/Channel 表结构
- 生产影响：当前阶段只涉及开发源码、文档和测试；不修改生产源码、生产数据库、Docker Compose 或生产容器
- 发布门槛：隔离数据库升级和运行验证已完成；用户明确接受本次跳过生产数据副本升级演练，但仍需完成生产备份、保留旧镜像、同镜像测试回归与独立 Agent 发布交接。本设计不授权当前会话直接切换生产容器

## 1. 背景与问题定义

New API 当前已经可以通过管理员渠道接口查询 ChatGPT Subscription（Codex）渠道的上游额度，但该能力属于渠道管理接口的一部分。现有接口要求管理后台身份，客户端需要传入内部渠道 ID，并且返回接近上游的原始响应，不适合作为 Android、HarmonyOS、手表或其他受信客户端长期使用的公共接口。

当前实现的事实边界如下：

- Codex 渠道类型是 `ChannelTypeCodex = 57`，显示名称为 `ChatGPT Subscription (Codex)`。
- 查询上游接口是 `/backend-api/wham/usage`，不是通用 ChatGPT 网页聊天额度接口。
- 上游稳定数据主要是使用比例、窗口时长和重置时间，不能承诺绝对 Token 数量或金额。
- 管理员接口位于 `/api/channel/:id/codex/usage` 所在的 `/api/channel` 路由组下，该路由组使用 `AdminAuth()` 和渠道权限。
- 查询遇到上游 401/403 时可能刷新 OAuth，并把新凭据写回 `Channel.Key`，因此“客户端只读”不等于“服务端绝无写操作”。
- 现有 `model.Token` 是模型调用 API Key；现有 `TokenAuthReadOnly()` 没有资源绑定和细粒度 Scope，不能直接表达“只能读取某一个 Codex 渠道”。

## 2. 设计目标

V2 提供一套面向多个受信客户端的通用 Scope 授权能力，第一阶段只开放 `codex.quota.read`，用于读取一个被明确绑定的 Codex 渠道额度。

目标包括：

1. 客户端无需持有管理后台 PAT、模型 API Key 或 Codex 上游 OAuth 凭据。
2. 每个客户端访问凭据可以拥有一个或多个由服务端注册的 Scope，每个 Scope 都有明确的资源约束。
3. 公共接口只返回稳定、裁剪后的额度 DTO，不返回上游原始响应或内部渠道对象。
4. 凭据可以创建、禁用、撤销、轮换和设置过期时间。
5. 服务端缓存、限流和并发合并能够避免客户端轮询放大成上游高频请求。
6. Codex 额度查询只是首个 Scope，凭据、授权记录、鉴权中间件和管理页面不使用 Codex 专属命名。
7. 新增代码集中在独立文件、包和前端 Feature 中；只有统一 OAuth 刷新竞争等正确性要求确实需要时，才触及现有业务文件。
8. 数据库只新增独立的凭据表和 Scope 授权表，不修改 `Token`、`Channel` 或日志表，不做历史数据回填。
9. 未来同步官方主线时，冲突主要限制在路由注册、迁移注册、OAuth 刷新协调、导航和生成文件等明确列出的接入点。

## 3. 非目标

本阶段不做以下事情：

- 不开放匿名访问。
- 不让客户端直接访问 Codex 上游。
- 不把现有模型 API Key 改造成公共管理凭据。
- 不复用管理员 Access Token 作为正式客户端凭据。
- 不暴露渠道 Access Token、Refresh Token、Account ID、完整 `Channel.Key` 或内部渠道 ID。
- 不开放额度重置、渠道修改、用户管理或系统配置写操作。
- 不把所有 OpenAI/ChatGPT 渠道都视为 Codex 额度渠道。
- 不在第一阶段开放管理员自行输入未知 Scope；Scope 必须由服务端白名单注册。
- 不在第一阶段开放写 Scope、通配 Scope（`*`）或“全部资源”授权。
- `codex.quota.read` 首期只允许绑定一个 Codex 渠道，不提供跨账号聚合。
- 不在本阶段迁移 Chrome 插件的管理后台认证方式；插件迁移另立任务。
- 不调整现有 `/api/channel/:id/codex/usage` 管理接口的返回契约。

## 4. 关键决策

### 4.1 凭据不复用现有 Token 表

采用独立的通用客户端访问凭据模型和 Scope 授权模型：

```text
ClientAccessCredential
table: client_access_credentials

ClientAccessGrant
table: client_access_grants
```

不采用以下方案：

| 方案 | 结论 | 原因 |
| --- | --- | --- |
| 直接复用 `model.Token` | 不采用 | 会把模型调用、额度、分组、模型限制和公共只读权限混在一起，必须改造所有 Token 鉴权和调用路径 |
| 复用用户 `AccessToken`/PAT | 不采用 | 管理权限过宽，不能安全绑定一个渠道，撤销和审计语义也不同 |
| 环境变量全局 Token | 仅适合临时原型 | 无法通过页面管理、无法按客户端独立撤销，也无法表达资源绑定 |
| 新建凭据表和 Scope 授权表 | 采用 | 凭据生命周期与具体能力解耦，未来增加 Scope 不需要修改凭据表或现有 Token/Channel 表 |

可以复用现有管理页面的表格、弹窗、复制按钮和渠道选择器，但不复用现有 Token 数据模型和 Token 鉴权中间件。

### 4.2 一个凭据可以拥有多个 Scope

凭据本身只表示客户端身份和生命周期；具体可以访问什么，由授权记录决定：

```text
一个 ClientAccessCredential
  -> 多个 ClientAccessGrant
       -> scope + resource_type + resource_id
```

第一阶段只有一个可选 Scope：

```text
scope         = codex.quota.read
resource_type = channel
resource_id   = 被绑定的 Codex 渠道 ID
```

同一个凭据以后可以增加其他 Scope，但客户端不能通过任意字符串创造 Scope。所有 Scope 都必须在服务端 Scope Catalog 中注册，并提供资源校验规则。

对于 `codex.quota.read`，V2 UI 和后端均限制一个凭据最多绑定一个 Codex 渠道。客户端请求不带 `channel_id`，服务端从 Grant 中解析资源。

这样可以避免：

- 客户端枚举渠道 ID；
- 凭据读取其他渠道；
- 公共接口暴露 New API 内部数据结构；
- 后续增加多渠道能力时破坏单渠道权限语义。

后续如果确实需要同一 Scope 绑定多个资源，可以调整该 Scope 的 `max_resources` 规则或新增聚合 Scope，不需要修改凭据表结构。

### 4.3 Scope Catalog：可扩展但不可任意配置

新增代码级 Scope Catalog，每个 Scope 定义至少包含：

```text
key
display_name
description
resource_type
max_resources
risk_level
validator
```

首期注册：

```text
key           = codex.quota.read
resource_type = channel
max_resources = 1
risk_level    = read_only
```

`resource_type` 表示授权目标所属的平台资源类别，不表示渠道的具体类型。这里直接使用 New API 已有的 `channel` 概念，不新增 `codex_channel` 类型。`codex.quota.read` 的资源 Validator 再使用原有渠道类型常量筛选：

```text
channel.Type == constant.ChannelTypeCodex // 57
channel.ChannelInfo.IsMultiKey == false
```

界面显示名称沿用平台对 `ChannelTypeCodex` 的名称映射，即 `ChatGPT Subscription (Codex)`；数据库 Grant 不保存该显示名称，也不把数值 `57` 当作 `resource_type`。这样将来官方调整渠道显示文案时，不需要迁移 Grant 数据。若官方改变类型常量或渠道结构，必须在上游同步检查中复核 Validator。

管理页面从 Scope Catalog 获取可选项，以复选框形式选择 Scope。当前页面只会显示 `Codex 额度读取`，未来新增注册项后可以显示多个 Scope，不需要改凭据数据模型。

“Scope 白名单”不等于“写死某个渠道 ID”。渠道 ID 可以变化，资源校验由对应 Scope 的 Validator 执行。前端筛选只负责改善操作体验，不能作为安全边界。

Scope Key 一旦发布即视为稳定安全契约，不能在同名 Key 下悄悄扩大权限。若未来语义发生破坏性变化，应新增 Scope Key；Catalog 中已移除或无法识别的 Scope Grant 必须拒绝访问，不能自动映射到其他权限。

### 4.4 扩展一个新 Scope 的固定步骤

未来增加新的只读查询能力时，应只需要：

1. 在 `service/clientaccess/scope_registry.go` 注册新的 Scope 定义。
2. 实现该 Scope 的资源列表和资源 Validator；无资源 Scope 可以明确声明 `resource_type=none`。
3. 为对应业务接口增加 `RequireClientScope("<scope>")`。
4. 在管理页面根据 Scope Catalog 自动展示复选框和必要的资源选择器。
5. 增加该 Scope 的授权、越权、资源失效和接口契约测试。

正常增加 Scope 不应修改凭据表、Grant 表、Token 生成逻辑或已有 Scope 的数据。首期不建设动态路由、用户自定义 Scope、通配授权或权限插件系统；当前扩展点只解决“以后新增白名单能力时不用重做凭据模型”，避免为了不确定的未来过度设计。

## 5. 总体架构

```text
受信客户端
    |
    | Authorization: Bearer <client-access-token>
    v
/api/client-access/v1/codex/usage
    |
    v
客户端访问凭据鉴权
    |-- 校验 Token Hash、状态、过期时间
    |-- 加载凭据关联的 Scope Grant
    |-- 校验当前路由需要的 Scope
    |-- 解析并校验 Scope 对应资源
    |-- 记录脱敏审计
    v
Codex 额度服务
    |-- 校验渠道类型和 OAuth 配置
    |-- 读取短时缓存
    |-- 同渠道并发合并
    |-- 必要时刷新 OAuth（服务端内部写操作）
    |-- 解析并标准化上游窗口
    v
裁剪后的稳定 DTO
```

现有管理员接口继续走现有 `AdminAuth()` 和渠道权限。客户端访问接口使用独立中间件和独立 Controller，不通过调用现有 Controller 来复用响应逻辑。

## 6. 源码目录与低耦合边界

### 6.1 新增文件和目录

遵循当前 Go 项目的顶层目录习惯，但每个职责使用独立文件或子包：

```text
model/
├── client_access_credential.go
├── client_access_grant.go
└── client_access_migration.go

constant/
└── client_access.go

middleware/
└── client_access.go

service/clientaccess/
├── credential.go
├── grant.go
├── scope_registry.go
├── codex_usage.go
├── cache.go
├── limiter.go
└── audit.go

service/
└── codex_refresh_coordination.go  # 现有与新接口共用，避免 Go 包循环依赖

controller/
├── client_access_credential.go
└── client_codex_usage.go

router/
└── client-access-router.go

web/src/features/client-access-credentials/
├── api.ts
├── types.ts
├── components/
├── pages/
└── __tests__/
```

目录命名使用小写和连字符/下划线，Go 包名使用 `clientaccess`；不新建 `mobile`、`android`、`harmony` 或 `codexcredential` 专用凭据包。

### 6.2 官方文件修改边界

实现阶段原则上不修改以下现有业务文件的内部逻辑：

- `model/token.go`
- `model/channel.go`
- `middleware/auth.go` 中现有 `TokenAuth()`、`TokenAuthReadOnly()` 逻辑
- `service/codex_wham_usage.go`
- `router/channel-router.go`
- 现有日志聚合 Controller、Model 和前端 `usage-summary` Feature

已识别的安全例外是 `middleware/audit.go`：PAT 审计必须跳过 `napi_client_` 前缀，避免客户端凭据作为 PAT 查询值进入 SQL 错误日志；它仍须经过独立客户端鉴权，不得因此获得后台权限。

已识别的正确性例外是现有 OAuth 刷新路径：`controller/codex_usage.go`、`service/codex_credential_refresh.go` 以及其定时/模型列表调用方可能同时刷新同一渠道。新公共接口不能另造一把只有自己遵守的锁。实施时应将刷新协调集中在 `service` 包的共享函数，由旧管理员额度 Controller 做最小委托；`service/clientaccess` 可以调用它，反向不得导入 `clientaccess`，避免 Go 包循环依赖。保留旧接口 URL、HTTP 状态及响应结构。`service/codex_wham_usage.go` 的底层读取函数可以直接复用。若统一刷新无法在隔离测试中证明安全，不上线公共接口的自动刷新，改为返回标准化暂不可用错误，待单独修复。

### 6.3 允许的最小官方接入点

预计保留以下必要接入点；OAuth 刷新协调的兼容性例外见 6.2：

1. `router/api-router.go` 增加一次 `registerClientAccessRoutes(apiRouter)` 调用。
2. `model/main.go` 在官方 `DB.AutoMigrate(...)` 完成后增加一次独立的客户端访问表迁移调用。
3. Web 侧增加一个导航入口和一个 TanStack Router 路由生成结果。
4. 如果权限目录需要展示 Root 专属能力，只新增独立的权限注册文件，不修改现有渠道权限语义。

这些接入点必须在设计文档和同步检查单中单独列出。以后合并官方主线时优先检查它们，不应把新功能逻辑塞进官方大文件。

## 7. 数据库设计

### 7.1 表名

```text
client_access_credentials
client_access_grants
```

表名使用通用客户端访问语义，避免与 `tokens`、`user_sessions`、`channels` 或 Codex 专属表混淆。增加新 Scope 时只新增 Scope Catalog 代码和授权记录，不需要为每个 Scope 建新凭据表。

### 7.2 凭据表字段

Go 字段遵循当前项目已有的 `Id`、`UserId`、`CreatedTime` 风格；数据库列使用 GORM 现有的 snake_case 规则。

| 字段 | 类型/约束 | 用途 |
| --- | --- | --- |
| `id` | 主键 | 凭据内部标识 |
| `name` | `varchar` | 管理员给客户端起的名称 |
| `token_hash` | `varchar`、唯一索引 | 只保存客户端 Token 的 SHA-256 哈希 |
| `token_prefix` | `varchar` | 列表中用于识别的脱敏前缀 |
| `status` | `int`、索引 | enabled、disabled、revoked |
| `created_by_user_id` | `int`、索引 | 创建者审计主体 |
| `created_time` | `bigint` | 创建时间 |
| `expired_time` | `bigint` | 过期时间，使用项目现有 `-1` 永不过期约定 |
| `last_used_time` | `bigint` | 最近通过凭据认证的使用时间；限频更新，非授权依据 |
| `last_used_ip` | `varchar` | 脱敏或截断后的最近来源地址，可选 |
| `revoked_time` | `bigint` | 撤销时间 |
| `updated_time` | `bigint` | 管理记录更新时间 |
| `version` | `bigint`、默认 1 | Grant 更新的乐观并发版本；成功修改时递增 |

凭据表不包含 `scope`、`resource_type`、`channel_id` 或 Codex 专属字段。凭据轮换、禁用和撤销不会改变其 Grant；轮换只替换 Token Hash。

### 7.3 Scope 授权表字段

每个 Grant 表示“某个凭据获得一个 Scope，并被限制在一个明确资源上”。

| 字段 | 类型/约束 | 用途 |
| --- | --- | --- |
| `id` | 主键 | 授权记录标识 |
| `credential_id` | `int`、索引 | 所属客户端访问凭据 |
| `scope` | `varchar`、索引 | 例如 `codex.quota.read` |
| `resource_type` | `varchar` | 平台资源类别；首期为 `channel`，不表示渠道类型 |
| `resource_id` | `varchar` | 资源标识；Codex 渠道使用十进制渠道 ID 字符串 |
| `created_time` | `bigint` | 授权创建时间 |
| `updated_time` | `bigint` | 授权更新时间 |

建议使用唯一索引：

```text
(credential_id, scope, resource_type, resource_id)
```

Scope Catalog 决定一个 Scope 是否需要资源、资源类型以及最多允许绑定多少个资源。数据库只保存已验证的 Grant，不保存管理员随意输入的未知 Scope。

### 7.4 不建立到官方表的数据库外键

`client_access_grants.resource_id` 与官方资源存在业务绑定关系，但不建立到 `channels`、`users` 等官方表的数据库外键：

- 官方 `channels` 表可能继续演进；
- MySQL、PostgreSQL、SQLite 的约束行为和迁移顺序不同；
- 渠道删除或重建时，Grant 需要进入“资源不可用”状态，而不是触发级联删除；
- 将绑定校验放在 Service 层可以减少对官方 Channel 模型和迁移顺序的耦合。

`credential_id` 只关联本子系统自己的凭据表，可以通过同一事务维护 Grant，是否建立子系统内部外键由数据库兼容测试决定；无论是否建立，都不得向官方表添加约束。

创建 Grant、每次客户端查询和管理员查看状态时都必须通过 Scope Validator 重新校验资源存在、类型、状态和必要配置。

### 7.5 Token 生成与保存

服务端使用 `crypto/rand` 生成至少 256 bit 随机值，编码为 URL-safe 字符串，并加上可识别但不含权限信息的前缀，例如：

```text
napi_client_<random-value>
```

数据库只保存哈希和前缀。完整 Token 只在创建或轮换成功响应中返回一次。任何列表、详情、日志、审计或错误响应都不得返回完整值。

### 7.6 数据库归属

`client_access_credentials` 和 `client_access_grants` 属于 New API 主数据库 `DB`，不属于可选的日志数据库 `LOG_DB`：

- 凭据管理和 Scope 授权是业务配置，不是消费日志；
- 公共查询必须和主数据库中的 `channels` 配置保持一致；
- 日志数据库可能是 ClickHouse 或其他独立实现，不适合作为凭据状态存储；
- 凭据撤销、轮换和过期判断不能依赖日志库可用性。

客户端访问审计可以沿用现有日志/审计能力，但审计记录不能替代凭据表和 Grant 表，也不能把完整 Token 写入日志库。首期只读请求使用客户端访问路由的独立结构化应用日志，不增加第三张数据库表，也不把客户端凭据伪装成后台 PAT。日志只含凭据 ID、Scope、HTTP 方法、路由模板、状态、结果、稳定错误类型、耗时及按凭据 Token Hash 作 HMAC 的渠道资源指纹；不记录 Authorization、查询串、请求/响应正文或 OAuth 内容。`/api/client-access/` 的 Gin 访问日志同样隐藏整个查询串。

## 8. 管理页面与管理接口

### 8.1 页面位置

```text
系统设置 → 客户端访问凭据
```

首期管理接口使用 `RootAuth()`，因为创建凭据会授予外部客户端访问内部资源的能力。未来如果需要交给普通管理员，再单独增加 `client_access_credential` 权限资源；不复用 `ChannelSensitiveWrite`，避免把渠道修改权限和客户端授权混为一谈。

### 8.2 创建流程

1. 管理员填写凭据名称和有效期。
2. 前端读取 Scope Catalog，以复选框展示可以授权的 Scope。
3. 管理员可以勾选一个或多个 Scope；当前只提供 `Codex 额度读取`。
4. 对需要绑定资源的 Scope，前端根据 Scope 定义加载对应资源选择器。
5. `codex.quota.read` 只展示 `ChannelTypeCodex` 且非多 Key 的渠道。
6. 后端根据 Scope Catalog 逐项校验 Scope、资源类型、资源数量和实际资源状态。
7. 服务端先在内存生成 Token，仅在同一个事务中保存 Credential Hash 和全部 Grants；事务提交成功后才向页面返回完整 Token。
8. 页面显示一次完整 Token，并明确提示“离开页面后不可恢复”。

创建请求的概念结构如下：

```json
{
  "name": "Android 主手机",
  "expired_time": -1,
  "grants": [
    {
      "scope": "codex.quota.read",
      "resource_id": "123"
    }
  ]
}
```

客户端不提交可信的 `resource_type`。服务端从 Scope Catalog 推导资源类型，并拒绝未知 Scope、重复 Grant、超出资源数量限制或资源不合法的请求。
创建时至少选择一个 Grant；后续更新可以把 Grants 清空，此时凭据仍为 enabled 但无任何访问权限。Grant 更新使用全量替换，必须携带列表响应中的 `version`；服务端在同一事务中以版本条件更新 Credential、删除旧 Grants 并插入新 Grants。版本不符返回 409，整次更新回滚，页面提示刷新后重试。任何校验失败不得留下部分授权。

### 8.3 管理接口建议

管理接口放在独立命名空间，不修改现有 `/api/token`：

```text
GET    /api/client-access/scopes
GET    /api/client-access/scopes/:scope/resources
GET    /api/client-access/credentials
GET    /api/client-access/credentials/:id
POST   /api/client-access/credentials
PUT    /api/client-access/credentials/:id/grants
POST   /api/client-access/credentials/:id/rotate
POST   /api/client-access/credentials/:id/disable
POST   /api/client-access/credentials/:id/enable
POST   /api/client-access/credentials/:id/revoke
```

管理响应只返回脱敏字段和 `version`。轮换在事务中替换 Hash 并递增版本，提交后新 Token 只展示一次，旧 Token 的下一次鉴权立即失效。更新 Grants 不更换 Token，但权限变更必须对下一次请求立即生效；不得缓存鉴权结果。`disable` 可再次启用，`revoke` 为不可逆终态，不允许恢复或轮换。所有管理写操作沿用 Root 审计，审计字段只含凭据 ID、操作、结果，不含 Token 或资源密钥。

## 9. 客户端接口契约

### 9.1 路径与认证

```text
GET /api/client-access/v1/codex/usage
Authorization: Bearer <client-access-token>
```

该路由声明所需 Scope 为 `codex.quota.read`。鉴权中间件先识别 Credential，再由 Scope 授权中间件读取对应 Grant；没有该 Scope 时返回 403。

不支持：

- Query 参数传 Token；
- Query 参数传 `channel_id`；
- 复用管理后台 Cookie；
- 使用模型 API Key 代替客户端访问凭据。

### 9.2 响应示例

```json
{
  "success": true,
  "data": {
    "provider": "codex",
    "quota_kind": "subscription_codex",
    "status": "fresh",
    "windows": {
      "five_hour": {
        "used_percent": 12.4,
        "remaining_percent": 87.6,
        "window_seconds": 18000,
        "reset_at": "2026-09-23T20:00:00Z"
      },
      "seven_day": {
        "used_percent": 32.1,
        "remaining_percent": 67.9,
        "window_seconds": 604800,
        "reset_at": "2026-09-29T00:00:00Z"
      }
    },
    "fetched_at": "2026-09-23T16:00:00Z"
  }
}
```

字段规则：

- `used_percent` 以当前上游字段为准；缺失时不得伪造。
- `remaining_percent` 只有在 `used_percent` 有效时计算，并限制在 0～100。
- `windows.five_hour`、`windows.seven_day` 固定存在，无法由上游 `limit_window_seconds` 明确识别时为 `null`；只按明确时长归类，不按 primary/secondary 位置或免费套餐猜测。
- 上游仅有未知窗口时，两个标准窗口均为 `null`；不伪造 5 小时/7 天值，也不把一份有效上游响应改称错误。
- `fetched_at` 表示服务端取得或缓存该结果的时间。
- HTTP 200 的 `status` 仅为 `fresh`（本次取得）或 `cached`（命中缓存）；200 响应不出现 `unavailable`/`upstream_error`。失败统一返回 `success:false`、稳定 `code` 与安全的 `message`，不附带旧缓存或部分数据；如未来需要 stale-while-error，再单独设计。

响应不得包含：

- `access_token`；
- `refresh_token`；
- `account_id`；
- `Channel.Key`；
- 完整上游 JSON；
- 管理员用户信息；
- 内部数据库错误和 SQL 信息。

### 9.3 错误分类

公共接口至少区分：

- 凭据缺失、无效、禁用、撤销或过期：401；
- 缺少路由要求的 Scope：403；
- 绑定渠道不存在、禁用或配置不完整：503；
- 客户端访问所依赖的 Redis 不可用：503，`code=CLIENT_ACCESS_REDIS_UNAVAILABLE`；
- 等待同渠道额度抓取锁超时：503，`code=CODEX_USAGE_FETCH_BUSY`；
- Codex 上游认证失败或刷新无法安全完成：503，`code=CODEX_UPSTREAM_AUTH_UNAVAILABLE`；
- 上游限流：503，`code=CODEX_UPSTREAM_RATE_LIMITED`，可带经过边界校验的 `Retry-After`；客户端凭据自身限流才返回 429；
- 上游暂时不可用：503，`code=CODEX_UPSTREAM_UNAVAILABLE`；
- 服务端数据库、panic 或其他内部异常：500，响应不得包含内部错误详情或 panic 原文。

错误消息不能泄露“其他渠道是否存在”等枚举信息。

## 10. 缓存、限流与 OAuth 刷新

### 10.1 缓存与授权即时性

- 额度结果按 `channel_id` 缓存，首期 TTL 固定 90 秒；缓存命中前仍须完成凭据、Grant 和资源实时校验。
- 缓存内容只保存标准化 DTO，不保存上游 OAuth 凭据。
- 凭据、Grant 和资源授权判定每次从主数据库读取，不走现有 Token/Channel 缓存；“立即失效”指管理事务提交后的下一次新请求，在途请求不追溯取消。
- 首期公共查询在生产和隔离测试环境都要求 Redis；缓存与每凭据限流使用同一共享 Redis。Redis 不可用时公共查询失败关闭（503），不回退为进程内缓存或直连上游。这样不需要依赖“是否单实例”的运行时推断。

### 10.2 并发合并

- 同一个渠道在缓存未命中时，同一进程内只允许一个上游请求。
- 多实例环境使用按渠道划分、带过期时间与随机持有者值的 Redis 锁；释放时仅持有者可删除。未取得锁者短暂等待并重读缓存；超时返回 503，不另发上游请求。
- 锁应按渠道资源维度，而不是按客户端 Token 维度，避免多个客户端重复刷新同一 OAuth。

### 10.3 OAuth 刷新

- `codex.quota.read` 接口只调用额度读取能力，不能暴露刷新接口。
- 上游 401/403 时，服务端可以使用现有 Refresh Token 执行内部刷新。
- 所有已识别的刷新入口必须共用按渠道维度的刷新协调；锁内重新读取最新渠道 Key，发现其他请求已刷新时直接复用；写回用旧 Key 条件更新（CAS），影响行数为 0 时重读，不覆盖较新的 Refresh Token。刷新后使渠道缓存失效。Redis 锁失效或 CAS 失败不能盲目重试旧 Refresh Token。定时刷新、模型列表刷新和管理员刷新也要纳入回归。
- 首期对已通过认证的凭据按凭据 ID 限制至多 30 次/分钟；限流在 Scope/渠道查询前执行，Redis 不可用时生产公共查询失败关闭。`last_used_time` 至多每凭据每 5 分钟写一次，写入失败只记录脱敏告警，不改变本次授权结果。
- 客户端额度请求审计只记录凭据 ID、Scope、静态路由模板、HTTP 结果、错误类型、耗时和渠道资源的脱敏 HMAC 指纹，不记录 Authorization、查询参数、请求/响应正文或完整 OAuth 内容。
- 刷新失败不得把上游响应原文返回给客户端。

## 11. 安全边界

必须满足：

- 生产环境只通过 HTTPS 使用公共接口。
- 每个客户端尽量使用独立凭据，不在多个设备间长期共享同一个 Token。
- 服务端每次请求都校验凭据状态、过期时间、Scope Grant 和绑定资源。
- 客户端不能通过修改请求参数访问其他渠道。
- 客户端访问凭据不能通过 `/api/token`、模型 `/v1/...` 或管理员路由使用。
- 日志、错误、缓存、Postman 示例和文档不能写入真实凭据。
- 管理页面创建凭据必须通过 Root 身份和现有安全验证策略。
- 渠道被禁用、删除或 OAuth 配置失效时，公共查询应进入明确的不可用状态。

## 12. 数据库迁移与兼容策略

### 12.1 迁移方式

新增独立迁移函数，例如：

```text
model.MigrateClientAccessTables(DB)
```

由 `model/main.go` 在官方主迁移完成后调用；具体实现位于新增的 `model/client_access_migration.go`，不把新模型直接塞入官方大段 `AutoMigrate` 列表。

迁移只执行：

- 创建 `client_access_credentials` 表；
- 创建 `client_access_grants` 表；
- 创建两个表自身的索引；
- 必要时为本子系统表增加后续字段。

迁移不得：

- 改写 `tokens`、`channels`、`users` 或 `logs`；
- 回填或转换现有 Token；
- 删除旧字段；
- 添加跨表级联删除；
- 自动删除客户端访问凭据或 Grant 记录。

### 12.2 兼容性要求

- 新版本第一次启动：新增表成功，旧数据行数不变。
- 新版本第二次启动：迁移幂等，不重复建表或破坏索引。
- 旧版本代码理论上可忽略新增表，但“可回滚启动”必须由生产备份的隔离演练证明；在演练通过前不能承诺无损回滚。回滚时保留新增表，不自动删表；旧版不会提供新接口。
- 如果迁移失败，应用启动失败并保留数据库现场，不做半自动回滚。不同数据库的 DDL 事务语义可能不同，备份和人工核查是恢复依据。
- 生产发布前必须使用生产数据库备份在独立环境演练。

### 12.3 迁移测试矩阵

至少验证：

- 新建空库；
- 带真实结构和代表性历史数据的升级库；
- PostgreSQL 生产数据库类型；
- 项目支持的 SQLite/MySQL 测试类型；
- 连续两次启动；
- 含客户端访问凭据和多个 Grant 时的禁用、撤销、轮换和授权更新；
- 保留新表后回滚应用版本的启动行为。

## 13. 与官方同步的低冲突策略

每次同步官方前，专门检查以下四类边界：

1. `model/main.go` 的迁移入口是否发生重排。
2. `router/api-router.go` 的路由注册区域是否发生变化。
3. `middleware`、`service/authz` 和 Redis/限流公共能力是否发生兼容性变化。
4. Web 路由、侧边栏和权限目录生成文件是否发生变化。

同步时的原则：

- 先合并官方主线，再复核路由、迁移、OAuth 刷新、前端导航等接入点。
- 不把客户端访问凭据和 Scope 逻辑挪进官方 Token/Channel 文件。
- 不直接修改官方 Codex 管理 Controller 的响应契约。
- 不把自定义表加入官方已有表的字段或约束。
- 每次同步后重新生成前端路由树，不手工维护大段生成文件。
- 为新增目录和表建立独立测试，官方测试失败时区分上游既有失败与私有功能回归。
- 生产候选镜像使用不可变标签和 digest，数据库迁移与镜像发布分开验收。

## 14. 测试设计

### 14.1 模型和迁移测试

- Token Hash 唯一性。
- Token 不落明文。
- 表和索引创建成功。
- 多 Scope Grant 的唯一性和事务一致性。
- 空 Grant 更新、旧版本号并发更新返回 409、撤销终态与轮换原子性。
- 未注册 Scope 不能写入。
- Scope Catalog 的 `resource_type` 和 `max_resources` 校验正确。
- 迁移重复执行幂等。
- 既有官方表和数据行数不变。
- 过期、禁用、撤销状态判断正确。

### 14.2 鉴权和越权测试

- 缺少 Authorization 返回 401。
- 随机 Token、过期 Token、撤销 Token、禁用 Token 均不能访问。
- 有效 Token 只能访问自身 Grant 授权的 Scope 和资源。
- 同一凭据勾选多个 Scope 时，各 Scope 独立授权、独立拒绝。
- 缺少 `codex.quota.read` 的有效凭据访问 Codex 接口返回 403。
- 修改 URL、查询参数或请求体不能切换渠道。
- 管理事务提交后的下一次新请求必须按新权限判定；并发中的旧请求不作追溯承诺。
- 一个 Grant 不能读取另一个资源。
- 客户端访问 Token 不能访问 `/api/channel`、`/api/token`、`/v1/...` 或管理页面 API。

### 14.3 Codex Provider 测试

- Codex 类型渠道通过。
- 非 Codex 渠道拒绝。
- 多 Key Codex 渠道拒绝。
- 缺少 `access_token` 或 `account_id` 拒绝。
- 5 小时、7 天和未知窗口映射正确。
- 上游 401/403 刷新成功、刷新失败和并发刷新行为正确。
- 管理员额度查询、定时刷新、模型列表刷新与公共查询并发时，不得用旧 Key 覆盖新 Key。
- 上游 429、超时和非 JSON 响应不会泄露原文。

### 14.4 API 和管理页面测试

- 创建接口只返回一次完整 Token。
- 列表和详情始终脱敏。
- Scope Catalog 只返回可公开的定义，不暴露 Validator 内部实现。
- `codex.quota.read` 的可绑定资源列表只列出 `ChannelTypeCodex` 渠道，且不返回 `Channel.Key`。
- Scope 复选框和条件资源选择器能正确生成 Grants。
- 更新 Grants 后下一次请求立即使用新权限。
- 轮换后旧 Token 立即失效。
- 禁用、撤销、过期和渠道不可用状态在 UI/API 中一致。
- 页面不影响现有 Token 管理页和 Codex 渠道管理页。

### 14.5 隔离环境回归

在 `new-api-development` 的独立测试容器和独立 PostgreSQL/Redis 数据目录中验证：

- 首次启动迁移；
- 第二次启动幂等；
- 管理员创建客户端访问凭据并勾选 `codex.quota.read`；
- 认证态查询 Codex 额度；
- 缓存命中和并发合并；
- 撤销后访问失败；
- 现有后台登录、模型调用、Responses/Codex 路由和日志统计仍正常。

生产容器、生产 Compose、生产 PostgreSQL、生产 Redis 和生产数据在本阶段不得触碰。

## 15. 实施顺序

设计确认后按以下顺序实施：

1. 创建功能分支，并确认开发/测试容器与生产完全隔离。
2. 新增常量、模型、独立迁移函数和迁移测试。
3. 新增客户端访问凭据 Service、Scope Catalog、Grant 管理、Hash 生成和管理 Controller。
4. 新增 `ClientAccessAuth` 和 `RequireClientScope` 中间件。
5. 抽取可复用的 Codex 查询/标准化 Service，同时保持现有管理员接口不变。
6. 新增客户端 Codex 查询 Controller 和路由注册。
7. 新增管理页面、Scope 复选框、条件资源选择、一次性 Token 展示、Grant 更新、撤销和轮换。
8. 增加 API 文档、Postman 集合和脱敏调用示例。
9. 执行 Go、前端、迁移、鉴权、越权和隔离容器回归。
10. 用户验收后再制作生产交接包；生产发布另行备份、审批和验证。

## 16. 验收标准

只有以下条件全部满足，V2 才能进入生产发布评估：

- [ ] 客户端访问凭据独立于模型 API Token 和管理 PAT。
- [ ] 凭据生命周期与 Scope/资源授权分表存储。
- [ ] 同一个凭据可以安全持有多个已注册 Scope。
- [ ] 未知 Scope、通配 Scope和任意资源授权均被拒绝。
- [ ] `codex.quota.read` 首期最多绑定一个 Codex 渠道。
- [ ] 客户端不需要也不能传入渠道 ID。
- [ ] 后端严格校验 `ChannelTypeCodex` 和非多 Key 约束。
- [ ] 完整 Token 只在创建/轮换时展示一次，数据库只保存 Hash。
- [ ] 客户端响应不含任何上游敏感字段和原始响应。
- [ ] 5 小时、7 天和未知窗口的语义明确。
- [ ] 缓存、限流和并发刷新不会放大上游请求。
- [ ] 生产 Redis 不可用时公共查询失败关闭，且不会绕过限流或直接高频访问上游。
- [ ] 两张新增表为独立表，迁移不修改官方既有数据表。
- [ ] 新旧版本连续启动和回滚启动行为已在隔离数据库验证。
- [ ] 管理员登录、模型调用、Codex 路由、日志统计和 Chrome 插件回归正常。
- [ ] 官方同步时只需处理明确记录的少量接入点。
- [ ] API 文档、Postman 集合、测试报告和发布记录完整。

## 17. 已确认的设计决策

以下基础模型来自此前需求讨论；实现阶段若发现与源码或隔离测试冲突，必须显式反馈，不得静默改变：

1. 独立 `client_access_credentials` 和 `client_access_grants` 两张表。
2. 独立 `ClientAccessAuth` 和 `RequireClientScope` 鉴权链。
3. 一个凭据可以拥有多个白名单 Scope；Scope 和资源以 Grant 表达。
4. 管理页面首期 Root 专用。
5. 客户端 API 使用 `/api/client-access/v1/codex/usage`。
6. 首期只注册 `codex.quota.read`，管理页面按 Scope Catalog 复选授权，不开放用户自定义 Scope。
7. `codex.quota.read` 的 Scope 定义限制 `resource_type=channel`、`max_resources=1`，其 Validator 使用现有 `ChannelTypeCodex` 常量限定渠道类型。
8. 迁移只新增两张表和索引，不改现有官方表。
9. 保留现有管理员 Codex 接口和 Chrome 插件路径不变。
10. 为正确性允许对现有刷新链路做最小共享化修改，必须以旧接口契约和已有刷新入口回归证明兼容；不能以“低冲突”为由留下并发覆盖风险。
