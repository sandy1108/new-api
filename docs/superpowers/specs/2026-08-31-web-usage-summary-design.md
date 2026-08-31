# New API Web 控制台 Token 用量统计集成设计

## 设计状态

本文档记录已确认的访问层边界和一期 Web 集成方案。当前仍处于设计评审阶段，尚未修改 Web 源码、后端代码、数据库或部署配置。

## 背景

Chrome 插件已经能够通过 New API 的服务端聚合接口读取日志用量，并在浏览器侧展示 Token、渠道和模型明细。但插件运行在 Web 控制台之外，需要 Content Script、已登录标签页探测、Access Token 刷新和兼容旧分页等机制。

Web 控制台本身已经运行在 New API 的同源页面中，并且已有统一的 Axios 请求实例和认证会话管理。因此，Web 控制台不应复制插件的外部扩展通信机制，而应直接复用现有前端公共封装。

## 目标

- 在 Web 控制台增加独立的 Token 用量统计页面。
- 直接调用现有的服务端聚合接口，避免读取数万条原始日志。
- 复用 Web 前端现有的 `api` 请求实例、认证会话、401 刷新和统一错误处理。
- 将新增代码集中在独立 feature 模块中，尽量不修改官方已有的日志页面。
- 保持管理员和普通用户的数据权限边界。
- 让未来同步官方主线时，冲突主要局限在导航、翻译和生成路由文件。

## 一期范围

一期只迁移 Chrome 插件中的日志用量统计部分：

- 今天、昨天、本周、上周、本月、上月、本季度七种时间范围。
- 管理员的“全部 / 仅自己”范围切换；普通用户固定查询自己的数据。
- 总请求数、输入 Token、输出 Token、总 Token 和 quota 展示。
- Token → 渠道 → 模型三级明细。
- 加载、空数据、刷新中、接口错误和会话失效状态。
- 使用 TanStack Query 做按范围和权限范围区分的缓存，并保留手动刷新能力。

一期不迁移 Codex 上游额度卡。Codex 额度查询依赖渠道发现、`ChannelRead` 权限和上游额度接口，单独作为二期 feature，避免与日志统计耦合。

## 非目标

- 不把 Chrome 扩展源码作为 Web 前端依赖，也不跨仓库导入插件文件。
- 不在 Web 控制台中实现 Content Script、标签页探测、Cookie 读取或手动 Access Token 刷新。
- 不保留插件的旧版分页 Provider 和大范围分页上限设置。
- 不修改现有 `usage-logs` 表格、分页、筛选和统计逻辑。
- 不修改 Go 后端聚合接口、数据库表、迁移、Docker Compose 或生产配置。
- 不通过前端扩大权限；管理员和普通用户仍分别使用后端规定的接口。

## 页面和模块结构

页面路径确定为 `/usage-summary`，导航名称确定为“Token Usage”。新增代码集中在以下目录：

```text
web/src/features/usage-summary/
├── api.ts                 # 服务端聚合接口调用
├── types.ts               # API 响应和页面模型
├── constants.ts           # 时间范围、查询默认值、缓存策略
├── components/
│   ├── summary-cards.tsx  # 总请求、Token、quota 卡片
│   ├── filters.tsx        # 时间范围和管理员范围切换
│   ├── token-table.tsx    # Token 汇总
│   └── token-detail.tsx   # 渠道和模型明细
├── lib/
│   ├── date-range.ts      # 时间范围转换
│   ├── format.ts          # Token、数量和 quota 格式化
│   └── selectors.ts       # 默认 Token、渠道和模型选择
└── __tests__/
    ├── date-range.test.ts
    ├── selectors.test.ts
    └── usage-summary.test.tsx
```

新增路由文件：

```text
web/src/routes/_authenticated/usage-summary/index.tsx
```

TanStack Router 会重新生成 `web/src/routeTree.gen.ts`。该文件属于生成产物，未来同步官方后应重新生成，不手工维护大段路由代码。

## 访问层设计

### 公共请求封装

`usage-summary/api.ts` 只使用现有的 `api` 实例，不自行创建 Axios 或 `fetch`：

- `baseURL` 使用当前 Web 页面同源地址。
- `withCredentials` 由公共客户端统一设置，浏览器按现有策略发送会话 Cookie。
- 请求拦截器统一附加当前会话的 Bearer Access Token。
- 401 由公共客户端触发已有的认证刷新和一次重试。
- 会话真正失效时沿用全局提示和登录页跳转。
- GET 请求继续使用现有的请求去重机制。

新 feature 不直接调用 `refreshAuthentication`，也不读取 `localStorage`、Cookie 或 Token。

### 后端路径选择

- 管理员选择“全部”：`GET /api/log/usage-summary`。
- 管理员选择“仅自己”：`GET /api/log/self/usage-summary`。
- 普通用户始终使用：`GET /api/log/self/usage-summary`。

前端根据当前登录用户角色和页面范围选择路径，但权限最终由后端 `AdminAuth` / `UserAuth` 决定。

### 查询参数

请求只发送服务端聚合接口支持的参数：

- `start_timestamp`
- `end_timestamp`

时间使用 Unix 秒，并采用服务端约定的闭区间。默认页面范围为“今天”，切换范围时由独立的 `date-range.ts` 生成参数。

一期不额外请求 `/api/status`。如果需要显示站点货币换算，优先复用已有状态查询缓存；状态不可用时显示原始 quota，不伪造金额。

## 数据流

```text
/_authenticated/usage-summary
  -> 页面 Hook / React Query
  -> usage-summary/api.ts
  -> 共享 api 实例
  -> /api/log/usage-summary 或 /api/log/self/usage-summary
  -> 服务端按 Token、渠道、模型聚合
  -> API 响应归一化
  -> 汇总卡片、Token 表格、渠道/模型明细
```

API 返回的扁平 `items` 由前端归一化为页面使用的 Token → 渠道 → 模型结构。前端不展开原始日志，也不执行分页和客户端全量读取。

## 页面行为

- 初始范围为“今天”。
- 管理员初始范围为“全部”，并可切换到“仅自己”；普通用户不显示管理员范围切换。
- 默认选中总 Token 消耗最高的 Token；点击 Token 后查看其渠道和模型明细。
- 切换时间范围或权限范围时，查询键必须变化，避免不同范围缓存混用。
- 刷新时保留上一次成功结果，避免页面内容闪烁。
- 空结果显示明确的“该范围内没有消费日志”，不能把未成功获取的数据显示为 0。
- 服务端聚合接口失败时显示错误，不自动退回旧分页接口，避免一次错误导致大量请求。

## 缓存和请求频率

- React Query 查询键至少包含功能名、管理员/自己的范围、开始时间和结束时间。
- 默认 `staleTime` 采用 10 分钟，与插件当前的统计刷新保护保持一致。
- 手动刷新只刷新当前范围和权限范围。
- 不在窗口获得焦点时自动重复请求，避免管理后台被多个标签页同时刷新。
- 服务端接口每个范围只需一次请求；响应大小取决于聚合维度数量，而不是原始日志条数。

## 错误和安全边界

- 401：交给共享 HTTP 客户端处理认证刷新；刷新失败时按现有全局行为跳转登录页。
- 403：显示无权限提示，不尝试切换到管理员接口或扩大查询范围。
- 404/接口格式错误：显示“当前服务端不支持用量聚合”或格式错误，不静默使用分页。
- 5xx/网络错误：保留上一次成功结果，并显示本次刷新失败原因。
- 普通用户接口忽略客户端可能传入的管理员筛选条件，权限边界以服务端为准。
- 页面不持久化 Access Token、Refresh Cookie、管理员 Token 或密码。

## 与官方主线的低冲突策略

新增内容优先放在新目录，不修改以下已有模块：

- `web/src/features/usage-logs/`
- Go 控制器、模型和路由
- 数据库迁移
- Dockerfile、Compose 和生产控制目录

预计必须修改的现有文件只有：

- `web/src/hooks/use-sidebar-data.ts`：增加一个导航项。
- i18n 资源：增加页面标题和统计文案，优先复用已有翻译键。
- `web/src/routeTree.gen.ts`：由路由生成器更新。

一期不把新页面加入可配置的 `sidebar_modules` 开关；如果后续确认需要隐藏/显示配置，再作为独立的小改动增加配置键和 URL 映射，避免把一期功能绑定到现有侧边栏配置结构。

未来 rebase 官方主线时，先重新生成路由树并检查导航/i18n 的小范围冲突；feature 目录本身应保持独立。

## 二期：Codex 额度模块

二期另建 `web/src/features/codex-quota/`，独立处理：

- Codex 渠道发现；
- `/api/channel/:id/codex/usage` 请求；
- 5 小时和 7 天窗口归一化；
- 额度缓存和刷新闸门；
- 计划线、节奏和不可用状态。

二期可以作为统计页的可选卡片或独立页面加载，但不能让日志统计依赖额度接口成功。

## 测试和验收标准

### 单元和组件测试

- 时间范围生成的起止秒数正确，跨日边界明确。
- API 响应能正确归一化为 Token、渠道、模型层级。
- 总请求数、输入/输出/总 Token 和 quota 与响应总计一致。
- 管理员“全部 / 仅自己”和普通用户路径选择正确。
- 空数据、加载中、刷新中、接口错误和 401 状态有明确行为。
- 点击 Token 和渠道后，明细内容按用户可见结果变化。

### 工程验证

在 `new-api-development` Worktree 中执行：

- 受影响 Vitest 测试；
- `bun run typecheck`；
- 涉及文件 lint；
- `bun run build` 或项目规定的构建检查。

随后在独立开发容器中构建和回归 Web 页面，验证生产容器、生产 Compose、Redis、PostgreSQL 和正式数据均未改变。生产发布必须另行走备份、镜像、回滚和交接包流程。

## 明确的成功标准

一期完成后，登录 Web 控制台的用户可以在 `/usage-summary` 查看服务端聚合的 Token 用量；管理员和普通用户只能看到后端允许的范围；刷新一个大时间范围最多产生一次聚合请求；现有使用日志页面行为不变；新增代码主要集中在独立 feature 目录；开发验证不触碰生产运行态。
