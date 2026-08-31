# Web 控制台 Token 用量统计集成实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans (or the repository-approved inline execution workflow) to implement this plan task-by-task. 本计划按当前会话 inline 执行，不启用子 Agent；每个任务完成后都要运行该任务列出的验证并保留本地提交证据。

**Goal:** 在 New API Web 控制台新增独立的 `/usage-summary` 页面，直接调用已经上线的日志用量聚合接口，提供按时间范围、Token、渠道和模型查看用量的能力，同时保持普通用户/管理员权限边界和现有日志页面不变。

**Architecture:** 新功能全部集中在 `web/src/features/usage-summary/`，通过现有 `web/src/lib/api.ts` 导出的共享 Axios 实例访问同源后端。页面只消费服务端返回的聚合行，在浏览器内做轻量的 Token → 渠道 → 模型归一化；认证刷新、Cookie、Bearer Token、401 重试和统一错误提示继续由现有 HTTP 客户端负责。路由和侧边栏只做最小接线，生成的路由树由 TanStack Router 重新生成。

**Tech Stack:** React 19、TypeScript、TanStack Router、TanStack Query v5、Axios 共享 `api` 实例、Zustand `useAuthStore`、i18next、Tailwind/Base UI、Vitest、React Testing Library、Bun、Rsbuild。

## 当前实施状态（2026-08-31）

- Tasks 1–9 的源码、测试和接线已完成，并已进入 `upgrade/upstream-main-20260831` 开发分支；Task 10 的 Node 等价测试、类型、定向 lint、定向格式检查和 Dockerfile 构建均已验证。
- Task 11 的隔离 HTTP/UI 回归已完成：管理员全部/仅自己、七个时间范围、刷新、空状态、旧日志页和请求路径均有真实浏览器证据；临时容器、网络和卷已清理。
- 宿主机 `Rsbuild`/`build:check` 仍受当前依赖目录缺少 `@lobehub/ui`、`antd` 阻塞；未安装依赖或修改锁文件。全仓 lint 的官方既有问题也未被带入本功能修复。
- Task 12 的文档内容已更新并完成扫描复核；最后的 commit、合并和推送属于独立 Git 发布动作，已获用户确认并按分支规范执行。

以下原始任务步骤保留作为实施设计记录；当前事实状态以本节和 `docs/development/release-log.md` 最新章节为准。

---

## 范围和固定边界

- 只实现日志用量统计一期；Codex 5 小时/7 天额度另建 `web/src/features/codex-quota/`，不在本计划内。
- 管理员可以在“全部”和“仅自己”之间切换；普通用户固定调用自己的接口，客户端状态不能扩大权限。
- 后端契约固定为：
  - `GET /api/log/usage-summary`（`AdminAuth`）；
  - `GET /api/log/self/usage-summary`（`UserAuth`）；
  - 查询参数 `start_timestamp`、`end_timestamp`，Unix 秒，服务端按闭区间筛选；
  - 返回 `data.total_requests`、`total_input_tokens`、`total_output_tokens`、`total_tokens`、`total_quota` 和扁平 `items`。
- 不修改 Go 控制器/模型/路由、数据库迁移、`web/src/features/usage-logs/`、Dockerfile、任何 Compose 或生产控制目录。
- 不引入插件源码，不实现 Content Script、标签页探测、Cookie 读取、Token 刷新、旧分页回退或 `/api/status` 额外请求。
- 生产容器、Redis、PostgreSQL、生产数据和生产 Compose 在整个开发阶段不得被重建、重启或修改。

## 文件总览

新增代码集中在以下文件；若实现中发现无需某个文件，必须在提交前删掉它并在计划记录中说明原因：

```text
web/src/features/usage-summary/
├── index.tsx
├── api.ts
├── types.ts
├── constants.ts
├── hooks/
│   └── use-usage-summary.ts
├── components/
│   ├── filters.tsx
│   ├── summary-cards.tsx
│   ├── token-table.tsx
│   └── token-detail.tsx
├── lib/
│   ├── date-range.ts
│   ├── format.ts
│   └── selectors.ts
└── __tests__/
    ├── api.test.ts
    ├── date-range.test.ts
    ├── format.test.ts
    ├── query.test.tsx
    ├── selectors.test.ts
    └── usage-summary.test.tsx

web/src/routes/_authenticated/usage-summary/index.tsx
```

预计只修改以下既有文件：

```text
web/src/hooks/use-sidebar-data.ts
web/src/i18n/static-keys.ts
web/src/i18n/locales/{en,zh,zh-TW,fr,ru,ja,vi}.json  # 通过现有 i18n 工具同步
web/src/routeTree.gen.ts                              # 生成产物，不手工维护
```

---

### Task 1: 建立 API 类型、时间范围常量和格式化契约

**Files:**

- Create: `web/src/features/usage-summary/types.ts`
- Create: `web/src/features/usage-summary/constants.ts`
- Create: `web/src/features/usage-summary/lib/format.ts`
- Test: `web/src/features/usage-summary/__tests__/format.test.ts`

- [ ] **Step 1: 先写格式化失败测试**

  在实现格式化函数前，验证有限整数使用本地千分位、非有限数值按约定显示 `0`，并覆盖 quota 与普通数量走同一安全格式化路径。

- [ ] **Step 2: 定义后端响应和页面模型类型**

  在 `types.ts` 中定义以下稳定契约，不使用 `any`：

  ```ts
  export type UsageSummaryScope = 'all' | 'self'
  export type UsageSummaryRangeId =
    | 'today'
    | 'yesterday'
    | 'week'
    | 'last-week'
    | 'month'
    | 'last-month'
    | 'quarter'

  export interface UsageSummaryItem {
    user_id: number
    username: string
    token_id: number
    token_name: string
    channel_id: number
    channel_name?: string
    model_name: string
    requests: number
    input_tokens: number
    output_tokens: number
    total_tokens: number
    quota: number
  }

  export interface UsageSummaryData {
    total_requests: number
    total_input_tokens: number
    total_output_tokens: number
    total_tokens: number
    total_quota: number
    items: UsageSummaryItem[]
  }

  export interface UsageSummaryEnvelope {
    success: boolean
    message?: string
    data?: UsageSummaryData
  }

  export interface UsageSummaryRange {
    id: UsageSummaryRangeId
    startTimestamp: number
    endTimestamp: number
    cacheKey: string
  }

  export interface UsageMetrics {
    requests: number
    inputTokens: number
    outputTokens: number
    totalTokens: number
    quota: number
  }

  export interface UsageModelBucket extends UsageMetrics {
    name: string
  }

  export interface UsageChannelBucket extends UsageMetrics {
    id: number
    name: string
    key: string
    models: UsageModelBucket[]
  }

  export interface UsageTokenBucket extends UsageMetrics {
    id: number
    name: string
    key: string
    userId: number
    username: string
    channels: UsageChannelBucket[]
  }

  export interface UsageSummaryViewModel {
    totals: UsageMetrics
    tokens: UsageTokenBucket[]
  }
  ```

  页面层只使用这些归一化模型，避免把后端 snake_case 字段散落在 TSX 中；模型只挂在渠道下，Token 只挂渠道列表。

- [ ] **Step 3: 固定选项和缓存策略**

  在 `constants.ts` 中导出只读的七个范围选项，label 使用 i18n key；默认范围为 `today`，默认管理员范围为 `all`，查询 `staleTime` 为 `10 * 60 * 1000`。不要在常量中写已经翻译好的中文，也不要增加未被页面使用的配置开关。

  ```ts
  export const USAGE_SUMMARY_STALE_TIME = 10 * 60 * 1000
  export const DEFAULT_USAGE_SUMMARY_RANGE: UsageSummaryRangeId = 'today'
  export const DEFAULT_USAGE_SUMMARY_SCOPE: UsageSummaryScope = 'all'
  export const USAGE_SUMMARY_RANGES = [
    { id: 'today', labelKey: 'Today' },
    { id: 'yesterday', labelKey: 'Yesterday' },
    { id: 'week', labelKey: 'This Week' },
    { id: 'last-week', labelKey: 'Last Week' },
    { id: 'month', labelKey: 'This Month' },
    { id: 'last-month', labelKey: 'Last Month' },
    { id: 'quarter', labelKey: 'This Quarter' },
  ] as const
  ```

- [ ] **Step 4: 实现最小数字格式化函数**

  在 `lib/format.ts` 中实现 `formatUsageNumber(value: number): string` 和 `formatUsageQuota(value: number): string`，统一使用 `Intl.NumberFormat`，对非有限值显示 `0`，不把 quota 擅自换算为 USD/CNY。所有页面文案仍通过组件的 `t()` 输出。

- [ ] **Step 5: 运行格式化测试、静态检查并提交类型层**

  运行：

  ```bash
  cd /Users/zhangyipeng/MyCodingSpace/ServiceTools/new-api-development/web
  bun run typecheck
  bunx oxlint -c .oxlintrc.json src/features/usage-summary/types.ts src/features/usage-summary/constants.ts src/features/usage-summary/lib/format.ts
  bun run test -- src/features/usage-summary/__tests__/format.test.ts
  ```

  预期：命令退出码均为 0。提交：`feat: 建立 Web 用量统计类型契约`。

### Task 2: 实现本地日历时间范围并先写单元测试

**Files:**

- Create: `web/src/features/usage-summary/lib/date-range.ts`
- Test: `web/src/features/usage-summary/__tests__/date-range.test.ts`

- [ ] **Step 1: 写固定时钟的失败测试**

  使用 `new Date('2026-08-05T14:32:18+08:00')`，至少覆盖：今天从本地零点到当前时刻、昨天完整自然日、本周从周一到当前时刻、上周完整周、本月从月初到当前时刻、上月完整月、本季度从季度首日到当前时刻。断言 Unix 秒、range id 和 `cacheKey`，不依赖执行机器当前时间。

- [ ] **Step 2: 运行测试确认实现尚不存在时失败**

  运行：

  ```bash
  bun run test -- src/features/usage-summary/__tests__/date-range.test.ts
  ```

  预期：在实现函数前因模块/导出不存在而失败；保留失败原因，不用放宽断言来“通过”。

- [ ] **Step 3: 实现 `createUsageSummaryRange`**

  在 `date-range.ts` 使用浏览器本地日历（与既有 `lib/time.ts` 一致），显式以周一为周起点；当前范围的结束时间为 `now`，昨天/上周/上月的结束时间为下一个边界减 1 秒。核心接口固定为：

  ```ts
  export function createUsageSummaryRange(
    id: UsageSummaryRangeId,
    now: Date = new Date()
  ): UsageSummaryRange
  ```

  不使用字符串拼接时区、不读取全局插件状态、不把“上月/上周”实现成滚动 30/7 天。

- [ ] **Step 4: 运行范围测试并提交**

  运行同一 Vitest 命令，预期所有范围用例通过；提交：`feat: 增加用量统计日历范围计算`。

### Task 3: 归一化 Token → 渠道 → 模型并保护聚合不变量

**Files:**

- Modify: `web/src/features/usage-summary/types.ts`（补充页面聚合模型）
- Create: `web/src/features/usage-summary/lib/selectors.ts`
- Test: `web/src/features/usage-summary/__tests__/selectors.test.ts`

- [ ] **Step 1: 写最小聚合失败测试**

  使用两条相同 Token/渠道/模型的服务端聚合行和一条同名 Token 但不同 `user_id` 的行，断言：

  - `requests`、输入/输出/总 Token、quota 均按服务端聚合行的数量求和；
  - 管理员“全部”查询时不同用户不会因相同 `token_name` 合并；
  - Token、渠道、模型均按 `totalTokens` 降序排列；
  - 缺失名称使用明确回退（`未命名令牌`、`渠道 #<id>`/`未记录渠道`、`未记录模型`）；
  - 空 `items` 返回空 Token 列表和全 0 页面总计。

- [ ] **Step 2: 实现不展开原始日志的归一化**

  在 `selectors.ts` 导出：

  ```ts
  export function aggregateUsageItems(
    items: UsageSummaryItem[]
  ): UsageSummaryViewModel

  export function selectDefaultToken(
    tokens: UsageTokenBucket[],
    selectedKey?: string
  ): UsageTokenBucket | null
  ```

  Token key 必须包含 `user_id`、`token_id` 和 `token_name`；渠道 key 必须包含 `channel_id` 和最终展示名；模型按最终名称分组。每行只累加一次 `requests`，禁止把 `requests = 3000` 展开成 3000 个对象。`total_tokens` 以输入加输出重新计算，页面总计同时使用后端总计做契约校验；发现不一致时保留服务端结果并由测试暴露问题，不静默修正成看似成功的 0。

- [ ] **Step 3: 运行聚合测试并提交**

  运行：

  ```bash
  bun run test -- src/features/usage-summary/__tests__/selectors.test.ts
  ```

  预期：所有聚合、排序、空数据和名称回退用例通过；提交：`feat: 归一化用量统计层级明细`。

### Task 4: 通过共享 `api` 实例接入两个聚合接口

**Files:**

- Create: `web/src/features/usage-summary/api.ts`
- Test: `web/src/features/usage-summary/__tests__/api.test.ts`

- [ ] **Step 1: 写 endpoint 和参数契约测试**

  使用 `vi.spyOn(api, 'get')` 验证：管理员 `all` 调用 `/api/log/usage-summary`，管理员 `self` 和普通用户调用 `/api/log/self/usage-summary`；两者只传 `{ params: { start_timestamp, end_timestamp } }`，不传分页参数、不调用 `/api/status`、不读取 Cookie/localStorage、不创建第二个 Axios 客户端。

- [ ] **Step 2: 实现 `getUsageSummary` 和响应校验**

  在 `api.ts` 只从 `@/lib/api` 导入 `api`，实现：

  ```ts
  export interface GetUsageSummaryParams {
    scope: UsageSummaryScope
    range: UsageSummaryRange
  }

  export async function getUsageSummary(
    params: GetUsageSummaryParams
  ): Promise<UsageSummaryData>
  ```

  请求成功但 `success !== true`、`data` 缺失或 `items` 不是数组时抛出可识别的 `Error`，让页面进入错误态；禁止自动调用旧分页接口。HTTP 401/403/5xx 原样交给现有拦截器和 React Query 处理。

- [ ] **Step 3: 运行 API 测试并提交**

  运行：`bun run test -- src/features/usage-summary/__tests__/api.test.ts`。预期所有路径、参数和格式错误用例通过；提交：`feat: 接入日志用量聚合接口`。

### Task 5: 封装 React Query、权限范围和刷新行为

**Files:**

- Create: `web/src/features/usage-summary/hooks/use-usage-summary.ts`
- Modify: `web/src/features/usage-summary/constants.ts`
- Test: `web/src/features/usage-summary/__tests__/query.test.tsx`

- [ ] **Step 1: 定义查询键和范围权限策略**

  在 Hook 中读取 `useAuthStore((state) => state.auth.user)`，以 `user.role >= ROLE.ADMIN` 判断是否允许管理员范围切换；普通用户无论本地状态如何都强制使用 `self`。Hook 签名固定为 `useUsageSummary(range: UsageSummaryRange, requestedScope: UsageSummaryScope)`，并返回实际生效的 `scope` 与 `canManageScope`。查询键固定包含功能名、scope、range id、起止秒：

  ```ts
  ['usage-summary', scope, range.id, range.startTimestamp, range.endTimestamp]
  ```

- [ ] **Step 2: 实现查询和手动刷新**

  `useQuery` 的 `queryFn` 调用 Task 4 的 `getUsageSummary`，设置 `staleTime: USAGE_SUMMARY_STALE_TIME`、`refetchOnWindowFocus: false`、`placeholderData: (previous) => previous`。返回 `data`、`isLoading`、`isFetching`、`isError`、`error` 和 `refetch`，页面刷新按钮只调用当前 query 的 `refetch`。

- [ ] **Step 3: 检查 Hook 类型和提交**

  运行 `bun run typecheck` 和涉及文件的 `bunx oxlint`，预期均退出 0；提交：`feat: 增加用量统计查询与权限范围`。

### Task 6: 实现筛选栏和五项汇总卡片

**Files:**

- Create: `web/src/features/usage-summary/components/filters.tsx`
- Create: `web/src/features/usage-summary/components/summary-cards.tsx`

- [ ] **Step 1: 实现可访问的筛选栏**

  `filters.tsx` 使用现有 Select/Tabs/Button，范围选项来自 `USAGE_SUMMARY_RANGES` 并通过 `t(option.labelKey)` 渲染。管理员显示“全部/仅自己”切换，普通用户不渲染该控件；每个控件有可访问 label，切换时调用父组件回调，不直接修改 Query Cache。

- [ ] **Step 2: 实现汇总卡片**

  `summary-cards.tsx` 展示请求数、输入 Token、输出 Token、总 Token、总 quota 五项，所有数字经过 `lib/format.ts`，不把空数据伪装成成功的 0 状态。卡片只接收明确的页面总计 props，不读取 Store 或发请求。

- [ ] **Step 3: 运行组件级静态检查并提交**

  运行 `bun run typecheck` 和 `bunx oxlint -c .oxlintrc.json src/features/usage-summary/components/filters.tsx src/features/usage-summary/components/summary-cards.tsx`；预期通过。提交：`feat: 增加用量统计筛选和汇总卡片`。

### Task 7: 实现 Token 表格和渠道/模型明细

**Files:**

- Create: `web/src/features/usage-summary/components/token-table.tsx`
- Create: `web/src/features/usage-summary/components/token-detail.tsx`

- [ ] **Step 1: 实现 Token 列表选择行为**

  Token 列表按 `selectDefaultToken` 默认选中最高总 Token 项；每行使用语义化按钮或可操作表格行，暴露 `aria-selected`，键盘可以改变选中项。点击后只更新父组件选中的 key，不重新请求接口。

- [ ] **Step 2: 实现渠道/模型明细**

  `token-detail.tsx` 只渲染当前 Token 的渠道和模型层级，展示请求数、输入/输出/总 Token 与 quota；没有子项时显示明确的空明细文案。不得把原始日志再次展开，不得修改 `usage-logs` 现有表格。

- [ ] **Step 3: 为可见交互写行为测试**

  在 `usage-summary.test.tsx` 中覆盖最高 Token 默认选中、点击另一 Token 后明细变化、键盘可操作性和空明细状态，使用角色/label/`aria-selected` 查询，不断言 Tailwind class 或私有 state。

### Task 8: 组装独立页面和所有运行状态

**Files:**

- Create: `web/src/features/usage-summary/index.tsx`
- Test: `web/src/features/usage-summary/__tests__/usage-summary.test.tsx`

- [ ] **Step 1: 组装 `UsageSummary` 页面**

  使用现有 `SectionPageLayout`，页面结构固定为标题、筛选/刷新操作、汇总卡片、Token 表格和明细。页面把 Task 2 的范围、Task 5 的查询结果和 Task 3 的归一化模型串起来；组件之间通过明确 props 传递，不跨模块读取插件状态。

- [ ] **Step 2: 覆盖加载、刷新、空、错误和权限行为**

  使用独立 `QueryClient` 和最小 i18n 测试包装器，至少验证：

  - 初始 loading 显示 skeleton，查询完成后显示卡片和列表；
  - 刷新期间保留上一次成功内容并显示刷新中状态；
  - `items=[]` 显示“该范围内没有消费日志”，不只显示五个 0；
  - API 错误显示错误文案和可再次点击的刷新按钮，不触发旧分页；
  - 401 交给共享客户端（测试只验证页面不自行刷新 Token）；403 显示无权限，不切换到管理员接口；
  - 非管理员不出现“全部”切换，管理员切换后 query key 和请求路径变化。

- [ ] **Step 3: 运行页面测试并提交**

  运行：

  ```bash
  bun run test -- src/features/usage-summary/__tests__/usage-summary.test.tsx
  ```

  预期：所有用户可见状态和交互断言通过；提交：`feat: 增加 Web Token 用量统计页面`。

### Task 9: 注册路由、侧边栏和国际化，保持官方文件改动最小

**Files:**

- Create: `web/src/routes/_authenticated/usage-summary/index.tsx`
- Modify: `web/src/hooks/use-sidebar-data.ts`
- Modify: `web/src/i18n/static-keys.ts`
- Modify: `web/src/i18n/locales/en.json`
- Modify: `web/src/i18n/locales/zh.json`
- Modify: `web/src/i18n/locales/zh-TW.json`
- Modify: `web/src/i18n/locales/fr.json`
- Modify: `web/src/i18n/locales/ru.json`
- Modify: `web/src/i18n/locales/ja.json`
- Modify: `web/src/i18n/locales/vi.json`
- Generated: `web/src/routeTree.gen.ts`

- [ ] **Step 1: 建立认证子路由**

  路由文件只做一件事：

  ```tsx
  import { createFileRoute } from '@tanstack/react-router'
  import { UsageSummary } from '@/features/usage-summary'

  export const Route = createFileRoute('/_authenticated/usage-summary/')({
    component: UsageSummary,
  })
  ```

  认证由父级 `/_authenticated` 路由负责，不在新页面重复实现登录跳转。

- [ ] **Step 2: 增加一条侧边栏入口**

  在 `use-sidebar-data.ts` 的 General 组增加 `Token Usage` → `/usage-summary`，复用现有 lucide 图标；不要把它放入 admin 组或新增 `sidebar_modules` 配置键，因为普通用户也有自己的统计权限。

- [ ] **Step 3: 更新 i18n 并检查差异**

  在 `static-keys.ts` 登记动态 label 和页面状态 key；英文 key 作为 flat JSON 的源键，中文提供正式译文，其他语言使用项目现有 `bun run i18n:sync` 同步流程。运行：

  ```bash
  cd /Users/zhangyipeng/MyCodingSpace/ServiceTools/new-api-development/web
  bun run i18n:sync
  git diff -- src/i18n/locales src/i18n/static-keys.ts
  ```

  只接受新增/更新本功能所需 key，不接受脚本造成的无关重排或删除；发现无关差异先恢复该部分再继续。

- [ ] **Step 4: 生成并验证路由树**

  运行 `bun run build` 触发 TanStack Router 生成 `src/routeTree.gen.ts`，确认包含 `/_authenticated/usage-summary/`，不手工编辑大段生成代码。提交：`feat: 接入用量统计页面路由和导航`。

### Task 10: 前端全量验证和代码自审

**Files:**

- Review: `web/src/features/usage-summary/`
- Review: `web/src/routes/_authenticated/usage-summary/index.tsx`
- Review: `web/src/hooks/use-sidebar-data.ts`
- Review: `web/src/i18n/static-keys.ts`
- Review: `web/src/routeTree.gen.ts`

- [ ] **Step 1: 运行受影响测试和完整前端测试**

  运行：

  ```bash
  cd /Users/zhangyipeng/MyCodingSpace/ServiceTools/new-api-development/web
  bun run test -- src/features/usage-summary/__tests__
  bun run test
  ```

  预期：新增测试和既有 Vitest 测试均退出 0；若 Bun 运行时出现已知依赖导入差异，必须记录原始输出并用项目已验证的等价 Node 运行方式复核，不能修改生产代码迁就测试运行时。

- [ ] **Step 2: 运行类型、lint、格式和生产构建检查**

  运行：

  ```bash
  bun run typecheck
  bun run lint
  bun run format:check
  bun run build:check
  ```

  预期：类型、lint、格式检查和 Rsbuild 构建全部退出 0；检查 `git diff --check` 无空白错误。

- [ ] **Step 3: 按需求自审差异**

  使用 `git diff --stat`、`git diff -- web/src/features/usage-summary web/src/hooks/use-sidebar-data.ts web/src/i18n web/src/routes/_authenticated/usage-summary` 逐行确认：没有修改后端、数据库、Compose、生产路径或插件代码；没有 Cookie/Token 持久化；没有旧分页回退；没有不必要的新依赖。提交：`test: 完成 Web 用量统计前端验证`。

### Task 11: 在独立开发环境做 Web 回归，不触碰生产

**Files / runtime targets:**

- Review only: `/Users/zhangyipeng/MyCodingSpace/ServiceTools/new-api-development/docker-compose.dev.yml`
- Runtime: `new-api-dev`、`new-api-dev-pg`、`new-api-dev-redis` 及其隔离卷/网络
- Production must remain untouched: `/Users/zhangyipeng/MyCodingSpace/ServiceTools/new-api`、正式 `new-api` 容器、正式 Redis/PostgreSQL

- [ ] **Step 1: 复核开发 Compose 边界**

  运行：

  ```bash
  cd /Users/zhangyipeng/MyCodingSpace/ServiceTools/new-api-development
  docker compose -f docker-compose.dev.yml config
  docker compose -f docker-compose.dev.yml ps
  ```

  确认开发 Compose 继续使用 `new-api-dev` 项目、独立容器/网络/数据卷和 `PASSWORD_LOGIN_ENCRYPTION_ENABLED=true`；不改文件、不执行生产 Compose 命令。

- [ ] **Step 2: 确认前端回归不需要重建后端镜像**

  本期只修改 Web 前端，开发 API 已经提供聚合接口，因此优先复用当前健康的 `new-api-dev`，不重建应用镜像、不改 `docker-compose.dev.yml`。如果实现同时引入了后端变更，才另行分配未复用的 `new-api:dev-20260831-<NN>-g<short-commit>` tag，使用隔离 Compose 重建并记录 `docker image inspect` 的摘要；不得覆盖现有 tag，不得把正式镜像当作开发镜像。

- [ ] **Step 3: 运行隔离 HTTP/UI 回归**

  验证开发 `/api/status` 为 `success=true`，两个聚合接口未认证为 401；启动 `make dev-web`（端口 5173）后用开发账号登录 `/usage-summary`，逐项切换今天/昨天/本周/上周/本月/上月/本季度、管理员全部/仅自己、刷新和空结果，确认浏览器 Network 每次范围只有一次聚合请求，现有 `/usage-logs/common` 仍可打开。不要把开发账号、Token 或真实数据写入 Git/日志。

- [ ] **Step 4: 复核生产完全未变**

  运行只读检查：

  ```bash
  docker inspect new-api --format '{{.Config.Image}} {{.State.Status}} {{.State.Health.Status}}'
  docker inspect new-api-redis --format '{{.Id}} {{.State.Status}}' 2>/dev/null || true
  docker inspect new-api-postgres --format '{{.Id}} {{.State.Status}}' 2>/dev/null || true
  git -C /Users/zhangyipeng/MyCodingSpace/ServiceTools/new-api status --short
  ```

  预期：正式镜像 digest、正式应用/Redis/PostgreSQL 容器和私人 Compose 无变化；本任务不执行正式发布、镜像替换、重启或回滚。

### Task 12: 记录阶段结果并准备后续发布门槛

**Files:**

- Modify after verification: `docs/development/release-log.md`
- Review: `docs/superpowers/specs/2026-08-31-web-usage-summary-design.md`

- [ ] **Step 1: 更新项目日志**

  在 `release-log.md` 追加本阶段的开发分支、功能提交、实际测试命令、开发镜像 tag/digest、开发容器验证结果和“生产未触碰”的事实；不写 Token、密码、Cookie 或其他凭据。若设计实现与设计文档有差异，先更新设计文档的“实际实现/验证”小节，再在日志中链接它。

- [ ] **Step 2: 做最终需求核对**

  明确确认：服务端聚合接口一次请求覆盖大范围；普通用户只能 self；管理员 all/self 路径正确；刷新不会回退分页；现有日志页未改；新增代码集中在 feature 目录；生产数据和 Compose 未改变。若任一项没有最新命令输出或测试证据，不得标记完成。

- [ ] **Step 3: 本地提交并停止在发布门槛前（实施阶段记录）**

  使用中文类型前缀提交最后的文档/验证变更，例如：`docs: 记录 Web 用量统计开发回归`。实施阶段原计划在此停止；本轮经用户确认后，Git 合并到 `personal/main` 和推送 `myfork` 作为独立收尾任务执行。构建正式候选镜像或重建生产容器仍需另开发布任务，重新执行备份、镜像摘要、回滚和用户确认门槛。

---

## 计划完成后的验证清单

- [x] `web/src/features/usage-summary/__tests__/` 新增测试全部通过。
- [ ] `bun run lint`、宿主机 `build:check` 仍受官方既有问题/当前依赖目录缺失阻塞；Node 等价类型、定向 lint、格式检查和 Dockerfile 构建已通过。
- [x] 开发容器 `/api/status`、两个接口的未认证 401、登录 Web 页面和七个范围的一次请求均有回归证据。
- [x] 生产镜像 digest、容器状态、Redis/PostgreSQL、私人 Compose 与本轮开发前保持未变。
- [x] `git diff --check` 通过，差异不含凭据和生产运行态文件（最终提交前需再次复核）。
- [x] `docs/development/release-log.md` 已记录本轮事实；commit、合并和推送按用户确认执行，生产切换仍保持独立审批。
