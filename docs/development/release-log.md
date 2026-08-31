# New API 构建与发布日志

本文件记录本项目每次源码同步、镜像构建、正式切换和回滚。凭据、Token、数据库密码和完整请求内容不得写入此处。

## 2026-08-29：同步官方主干并切换本地候选镜像

### 源码

- 官方远端：`origin/main`
- 个人稳定分支：`personal/main`
- 开发分支：`feature/log-usage-summary`
- 构建提交：`8454082f930f44593e92791c2581ffc63eb30a59`
- 构建目录：`new-api-production`
- 同步方式：fetch 官方主干后 fast-forward；两个 Worktree 均保持干净。

### 镜像

- 当前正式镜像：`new-api:rc-8454082f930f`（过渡期 tag）
- 镜像 ID：`sha256:30b1f6b7c3688cda93fb171ae6f84c0197e2904b1b69abbce21259609fd26b3e`
- 构建时间：2026-08-29 20:44:35（Asia/Shanghai）
- 架构：`linux/arm64`
- 镜像构建使用仓库根目录 `Dockerfile`；前端和 Go 后端均在镜像内完成构建。
- 后续候选镜像改用本文件上方规范的“上游版本-日期-当日序号-短哈希”格式。

### 数据、配置与回滚

- 正式 Compose：当前 `new-api/docker-compose.yml`，仅将 `new-api` 服务镜像从官方 tag 切换为本地候选 tag。
- 生产 `data`、`logs` 和 PostgreSQL 映射保持不变。
- PostgreSQL 备份：`ServiceTools/.backups/new-api/pre-sync-20260829-201506/postgres-new-api.dump`
- PostgreSQL 备份 SHA-256：`04b94b062b141a2f1fa01a348c3800ad94c7394a0a909fa603ae33a451cd926e`
- Compose 快照 SHA-256：`281cc0e615fb5cca1fed52a09472a796777c4c0508d26a6889ac2a2a5cd86288`
- 正式镜像回滚标签：`new-api:backup-official-20260829-rc27`
- 未执行 `docker compose down -v`、数据卷删除、`docker system prune` 或重新拉取 `latest`。

### 验证结果

- `docker compose config --quiet`：通过。
- `new-api`：候选镜像、`running`、`healthy`，`/api/status` 返回 `success: true`。
- 外部 Web UI：HTTP 200。
- 无 Token 访问 `/v1/models`：HTTP 401，符合鉴权预期。
- Redis、PostgreSQL：运行中，未被重建。
- 独立切换 Agent：实际 Responses/Codex 流式请求、补扣费、消费日志和批量更新均正常。
- `/api/status` 的 `version` 为空：因仓库 `VERSION` 文件为空，属于非 release 本地构建的已知差异。

### 结果

源码同步、本地镜像构建和正式应用容器切换完成，未发生回滚。后续每次构建必须先分配新的日期序号并追加本日志，再进行测试和发布。

### 后续镜像标签规则（自 2026-08-29 起）

为便于从镜像名直接定位构建时间、当日构建次序和源码来源，正式候选镜像统一使用：

```text
new-api:<upstream-version>-<YYYYMMDD>-<NN>-g<short-commit>
```

- `YYYYMMDD` 使用 `Asia/Shanghai` 日期；
- `NN` 是当天候选构建序号，从 `01` 开始，分配后不复用；失败构建也保留已分配的序号；
- `g<short-commit>` 用于回溯构建源码；没有可用上游版本时使用 `unreleased`；
- 开发镜像使用独立的 `new-api:dev-<YYYYMMDD>-<NN>-g<short-commit>` 序列，不与正式候选序号混用；
- 当前运行的 `new-api:rc-8454082f930f` 是本次切换保留的过渡 tag，后续不覆盖、不改名；下一次构建必须使用新日期序号 tag，并在本文件追加记录。

## 2026-08-30：日志用量聚合接口开发与隔离测试

### 开发范围

- 开发 Worktree：`new-api-development`。
- 分支：`feature/log-usage-summary`，基线提交 `8454082f930f44593e92791c2581ffc63eb30a59`。
- 新增只读接口：`GET /api/log/usage-summary`（管理员）和 `GET /api/log/self/usage-summary`（登录用户）。
- 数据源：`LOG_DB.logs`，固定统计 `type=2` 消费日志；按用户、Token、渠道和模型聚合请求数、输入/输出/总 Token 与 quota。
- 保留现有日志分页接口及全局 `page_size <= 100` 约束；不新增表、不修改日志数据、不切换生产镜像。

### 隔离测试环境

- Compose：`new-api-development/docker-compose.dev.yml`，项目名 `new-api-dev`。
- 应用、PostgreSQL、Redis 使用独立容器、网络、数据卷和宿主机端口 `3000`；测试数据与生产数据分离。
- 早期测试镜像标签：`new-api:dev-20260830-01-g8454082-dirty`，镜像 ID：`sha256:305bdcf874ad8274115e5baa09dd5d67e366175dd056d5b43cba564b4d027f66`；该镜像保留作历史追溯，Compose 别名不变。
- 开发阶段测试镜像标签：`new-api:dev-20260830-02-g8454082-dirty`（镜像 ID：`sha256:b975be1195d842984c4db46ff3c40583bf594448cc7ae683ae4a7afd29ed8b48`，构建时间：2026-08-30 04:02:34（Asia/Shanghai），架构：`linux/arm64`）。
- 生产发布后，`new-api-dev:local` 别名与正式候选 `new-api:v1.0.0-rc.27-20260830-01-g064a1078` 共用镜像 ID `sha256:c032fe63dc188342a390743f3752986fc1a93c68b89f24297b1c7705198aa932`；早期 `-02` 镜像仍保留作历史追溯。
- 首次标准 `docker compose up --build` 因 Docker 构建内存不足，在 Go 编译 `ch-go/proto` 时被 OOM killer 终止；未替换旧测试容器。随后使用一次性串行编译（`GOMAXPROCS=1`、`GOFLAGS=-p=1`）构建当前测试镜像，未修改仓库 Dockerfile。

### 验证结果

- `GOWORK=off go test ./model ./controller ./router`：通过。
- `go vet ./model ./controller ./router`：通过（在临时 Go 容器内执行）。
- 根模块全量测试与独立 `relaykit` 模块测试：全部通过。
- 管理员端到端汇总（合成数据）：4 请求、37 输入 Token、16 输出 Token、53 总 Token、420 quota。
- 管理员筛选、普通用户强制用户范围（忽略恶意 `username`）、时间范围校验和未认证 401：均符合预期。
- 测试应用重建后渠道名称从缓存正确返回；测试 PostgreSQL healthy、Redis 正常、`/api/status` HTTP 200。
- 隔离测试阶段的 `new-api-dev` 曾运行上述 `-02` 镜像；生产发布后的当前别名复用正式候选摘要，测试与生产镜像内容一致。
- 生产 `new-api` 仍运行 `new-api:rc-8454082f930f`，镜像 ID `sha256:30b1f6b7c3688cda93fb171ae6f84c0197e2904b1b69abbce21259609fd26b3e`，状态 `running + healthy`；生产 Redis、PostgreSQL、Nginx 未触碰。

### 当前状态

- 功能代码、测试、设计文档和 Postman 集合在实现提交 `064a107821e35ce0778ffa42230f477aab4fe27a` 中完成；本节记录的是隔离测试阶段的结果。
- 隔离测试完成后，生产发布按独立备份、评审和审批流程执行，结果见下方“日志用量聚合接口生产切换”章节。
- Chrome 插件接入仍是后续工作（2026-08-30 隔离测试阶段快照）；在接入前应保留旧分页作为兼容回退，并用同一时间窗口对比服务端聚合与插件旧算法。当前接入状态以本日志最新章节为准。

## 2026-08-30：日志用量聚合接口生产切换

### 发布对象

- 生产源码 Worktree：`new-api-production`，分支 `personal/main`。
- 功能分支 `feature/log-usage-summary` 已使用 fast-forward 合并；发布镜像构建源码为 `064a107821e35ce0778ffa42230f477aab4fe27a`，生产 Worktree 随后在文档收尾提交 `40bf77890c1b53d7f7901a3b1bdf002425b6a09e` 更新。
- 正式候选镜像：`new-api:v1.0.0-rc.27-20260830-01-g064a1078`。
- 镜像 ID：`sha256:c032fe63dc188342a390743f3752986fc1a93c68b89f24297b1c7705198aa932`，架构 `linux/arm64`。
- 候选镜像与测试环境使用同一摘要；未在生产发布时重新构建。

### 备份与配置

- 备份目录：`ServiceTools/.backups/new-api/release-20260830-log-usage-summary/`。
- PostgreSQL 自定义格式 dump：`postgres-new-api.dump`。
- dump SHA-256：`7933152fd59201de4f24760da0547142246c103e2c1d67140e6b5fcd31d59b07`。
- 已使用 `pg_restore --list` 读取 dump TOC（389 行），未对生产数据库执行恢复。
- 生产 Compose 相对切换前快照仅修改 `new-api` 服务镜像行；绝对数据路径、Redis/PostgreSQL 配置、容器名和网络保持不变。
- 未执行 `down -v`、数据卷删除、`system prune` 或重新拉取 `latest`。

### 运行与验证

- `new-api` 已切换到候选镜像，状态 `running + healthy`。
- 正式 Redis 容器 ID `50c2f0b181e5`、PostgreSQL 容器 ID `9693a25fd342` 未变化。
- 容器内 `/api/status` 返回 `success=true`、`setup=true`；`VERSION` 为空的本地构建差异仍存在。
- `/api/log/usage-summary` 与 `/api/log/self/usage-summary` 未认证均返回 401；`/v1/models` 已有真实 Codex 客户端请求返回 200，`/v1/responses` 未认证返回 401，路由和鉴权链可达。
- 启动和迁移日志未发现 FATAL、panic 或迁移失败；观察到的少量 `client_gone/context canceled` 属于流式客户端主动断开，需继续按正常业务日志观察，不能当作成功响应证据。
- 用户已在 Postman/实际客户端完成认证态管理员、普通用户聚合及相关业务回归，确认接口可用；凭据未写入交接包或本日志。

### 结果与后续

- 生产切换完成，未发生回滚；旧镜像 `new-api:rc-8454082f930f` 仍保留作为回滚对象。
- Postman 集合：`new-api-development/postman/New-API-Custom-APIs.postman_collection.json`，应放入 `PONG API` 分组并使用本机安全环境变量填写后台 Token。
- 下一阶段优先让 Chrome 插件接入聚合接口：默认携带有界时间范围，保留旧分页作为兼容回退，并在接入前用同一时间窗口对比服务端聚合与插件旧算法的请求数、输入/输出 Token 和 quota。

## 2026-08-30：Git 收尾与个人远端同步

- 文档收尾提交：`40bf77890c1b53d7f7901a3b1bdf002425b6a09e`（`docs: 完善日志聚合接口发布记录`）。
- `feature/log-usage-summary` 已推送到 `myfork/feature/log-usage-summary`；`personal/main` 已推送到 `myfork/personal/main`；两个远端分支 SHA 均与本地一致。
- 本次提交未包含生产 `docker-compose.yml`；根目录 Worktree 的私人 Compose 修改继续保留在本地部署控制目录。
- fetch 后发现官方 `origin/main` 为 `918427d8`，相对已验证基线 `8454082f` 另有一个上游提交；本次未将其混入已验证发布，后续同步需单独评审和回归。

## 2026-08-31：同步官方主线与回归验证

### 同步范围

- 官方目标：`origin/main` → `2b6f1dfefbe217fed31fc0726717cc7de6958e8e`，提交主题为 `fix(model): drop leftover prefill_groups unique constraints before AutoMigrate`。
- 开发 Worktree：`new-api-development`，分支 `upgrade/upstream-main-20260831`。
- 同步方式：以 `personal/main`（`5eceb0b7575476225fb24d4df785cd65bc4a9eb3`）为父提交合并官方主线，合并提交为 `5d3ec41d068a675ae60246637acfd295b61203af`。
- 本次同步实际引入官方主线连续 13 个提交（从 `918427d8` 到 `2b6f1dfe`），不是只有最后一个迁移提交；因此合入前按认证、任务插件、数据库、Relay 和前端模块完成整段回归审查。
- 代码快照：`backup/pre-upgrade-20260831-01` 仍保留在同步前的 `5eceb0b7`。
- 本轮只操作开发 Worktree；生产部署控制目录、生产 Compose、生产容器和生产数据均未修改或重启。

### 官方变更与风险边界

- 新增 `model/prefill_group_migration.go` 及测试：PostgreSQL 启动迁移前检查 `prefill_groups.name` 的遗留全局唯一约束，已知旧对象迁移为 `deleted_at IS NULL` 的部分唯一索引；遇到未知冲突对象时报告错误，不自动删除。
- `model/main.go` 不再使用旧的并行 `migrateDBFast` 路径，数据库迁移顺序和失败行为因此需要重点观察。
- 官方同时新增数据库变更验证要求。该提交涉及迁移/约束行为，在完成真实 SQLite、MySQL、PostgreSQL 矩阵以及新库/升级库的幂等性验证前，不把本分支标记为“数据库兼容已完成”或直接提升生产。

### 开发镜像与配置

- 测试镜像：`new-api:dev-20260831-01-g5d3ec41`。
- 镜像 ID：`sha256:f7855c8410afbdcdb51da6b0e141e2b3072dd2ad541f7c1d4c053e23d8d4111d`，架构 `linux/arm64`。
- 测试 Compose 显式设置 `PASSWORD_LOGIN_ENCRYPTION_ENABLED=true`；该设置不能依赖官方默认值。
- 测试容器：`new-api-dev`、`new-api-dev-pg`、`new-api-dev-redis`；数据、网络、卷和宿主机端口与生产隔离。

### 回归结果

- Go 根模块 `GOWORK=off go test ./...`：通过。
- Go 静态检查 `GOWORK=off go vet ./...`：通过；`relaykit` 独立模块测试：通过。
- 前端 Bun `typecheck` 与 `build`：通过。
- Node 24 等价 Vitest：59/59 测试文件、406/406 测试通过。
- Bun Vitest：51/59 文件、370/378 测试通过；剩余 8 项均在测试运行时导入 `zod` 时出现 `z.object`/`z.number` 未定义，未发现业务断言失败，未修改生产源码迁就该运行时差异。
- HTTP 回归：`/api/status`、加密登录公钥、加密登录/会话、JWT 个人接口、管理员聚合、普通用户越权防护、管理员用户/模型接口均通过；未认证聚合接口和 `/v1/responses` 正确返回 401。
- 测试数据库无活动残留会话；生产运行镜像仍为 `new-api:v1.0.0-rc.27-20260830-01-g064a1078`，摘要 `sha256:c032fe63dc188342a390743f3752986fc1a93c68b89f24297b1c7705198aa932`，状态 `running + healthy`。

### 数据库迁移矩阵（2026-08-31）

- 验证使用隔离 Docker 网络、临时数据库容器和临时数据卷；测试完成后已清理，未复用或修改 `new-api-dev`、生产数据库和生产 Redis。
- 实际版本：SQLite `3.51.0`、MySQL `8.0.46`、PostgreSQL `15.19`；Go 测试使用一次性 `golang:1.26.1-alpine` 工具容器（宿主机未安装 `go`，未改动用户 PATH 或工具链）。
- 单元/集成迁移命令：
  `TEST_MYSQL_DSN=... TEST_POSTGRES_DSN=... GOWORK=off go test -count=1 -v ./model -run '^TestMigratePrefillGroupUniqueness(SQLite|MySQL|PostgreSQL)$'`：三项均通过；随后 `GOWORK=off go test -count=1 -timeout=10m ./model`：通过。
- 新库启动：SQLite、MySQL、PostgreSQL 各启动两次，`/api/status` 均返回 `success=true`，第二次未重复创建索引或报迁移错误。
- 升级库启动：使用当前正式候选镜像先建立代表性旧库并写入既有数据，再使用同步后的开发镜像各启动两次；SQLite/MySQL 数据行保持，PostgreSQL 遗留全局唯一索引 `idx_prefill_groups_name` 被转换为 `deleted_at IS NULL` 的 `uk_prefill_name`。
- PostgreSQL 迁移后行为：活动名称重复仍被唯一索引拒绝；软删除后可重新使用同名；未知冲突对象的保护行为由迁移测试覆盖。
- 生产复核：`new-api` 仍为 `new-api:v1.0.0-rc.27-20260830-01-g064a1078`、摘要 `sha256:c032fe63dc188342a390743f3752986fc1a93c68b89f24297b1c7705198aa932`，状态 `running + healthy`；生产 PostgreSQL/Redis 容器 ID 未变化。

### 当前结论与后续门槛

- 官方主线已在隔离开发 Worktree 同步并完成应用层回归；已于本轮将 `personal/main` 快进到 `7d7f26ba1f4b79bd2921246f59b19ccb72205cbc`，并推送 `myfork/personal/main` 与 `myfork/upgrade/upstream-main-20260831`；生产容器仍未切换。
- 项目前文中“Chrome 插件尚未接入”的表述属于 2026-08-30 的阶段快照；截至本记录，用户已确认 Chrome 插件接入完成。后续不再把插件接入作为本分支的待办；Web 自定义统计页仍是可选后续工作。
- 三数据库迁移矩阵、新库/升级库双启动幂等、既有数据和唯一性约束验证均已完成；代码同步与分支整理已收尾。后续如要切换生产，仅需另行执行生产备份、镜像构建、停机窗口和回滚验证流程；本轮未切换生产容器。

### 本轮补充验证（2026-08-31）

- 开发 Worktree 的 `web/dist` 原本为空；按仓库 CI 和 `Dockerfile.dev` 的既定做法临时创建最小 `index.html` 后，`GOWORK=off go test -count=1 -timeout=15m ./...`、`GOWORK=off go vet ./...`、`relaykit` 独立测试和 `GOWORK=off go build ./...` 均通过。占位文件及一次性 Go 测试缓存卷已清理，未进入 Git。
- 开发容器 `new-api-dev` 使用镜像 `new-api:dev-20260831-01-g5d3ec41`，`/api/status` 返回 `success=true`；两个聚合接口未认证均返回 401，最近 30 分钟日志无 fatal/panic/migration/error。
- 开发 Compose 已固定顶层项目名 `new-api-dev`，与现有容器标签和数据卷命名一致；因此后续可直接使用 `docker compose -f docker-compose.dev.yml ...` 管理当前开发栈，无需额外追加 `-p`。该配置变更只影响 Compose 项目标识，不会自动重启容器或触碰生产。

### 生产候选镜像构建（2026-08-31）

- 构建来源：`personal/main`，提交 `c9215cfcbcf3b059a989b76a75f53c088c8371c3`；使用仓库原生 `Dockerfile` 完整构建前端和 Go 后端。
- 镜像标签：`new-api:v1.0.0-rc.29-20260831-01-gc9215cfc`；日期、当日序号和提交短 SHA 均按项目约定写入。
- 本地镜像摘要：`sha256:89a537dbf93ad07b56939e8136c6ad208106e824e8a94fe52729d59569e75b49`，架构 `linux/arm64`，大小约 240 MB；当前仅存在于本机，未推送镜像仓库。
- 构建过程的前端 Bun 构建、Go 依赖下载、Go 编译和最终镜像导出均成功；本次只生成镜像，未重建或重启正式 `new-api` 容器。
- 下一步需在独立测试数据和端口上运行该候选镜像并回归；生产 Compose 的 `PASSWORD_LOGIN_ENCRYPTION_ENABLED=true`、备份和切换仍是后续独立门槛。

### 生产候选隔离运行回归（2026-08-31）

- 使用独立临时 PostgreSQL、Redis、Docker 网络、数据卷和宿主机端口 `3310` 启动 `new-api:v1.0.0-rc.29-20260831-01-gc9215cfc`；未复用 `new-api-dev` 或生产数据。
- `/api/status` 返回 HTTP 200 且 `success=true`；因使用全新数据库，`setup=false` 属于预期的初始化向导状态。前端首页返回 200。
- `/api/log/usage-summary`、`/api/log/self/usage-summary` 未认证均返回 401；`POST /v1/responses` 未认证返回 401，鉴权边界正常。
- 启动日志仅出现预期的数据库迁移开始记录，未出现 fatal/panic/error；临时容器、网络、PostgreSQL 数据卷和端口已清理，候选镜像继续保留在本机。

### 生产切换前备份与交接准备（2026-08-31）

- 私人生产 Compose 已加入 `PASSWORD_LOGIN_ENCRYPTION_ENABLED=true`；当前运行容器仍保持旧镜像 `new-api:v1.0.0-rc.27-20260830-01-g064a1078`，该环境变量尚未通过重建应用容器应用。
- 预切换备份目录：`.backups/new-api/pre-switch-20260831-rc29/`。PostgreSQL dump SHA-256 为 `c8163626f04eeb3eff1b7a76de7c3da42b7833b6af4dcd0009ac6bbaacbae979`；加入加密登录后的 Compose 快照 SHA-256 为 `390c37f866f10d0d13b49ec09b02b0df0aae0796e43eaf90f314f9b77d43ade5`。
- 生产切换交接包：`.backups/new-api/pre-switch-20260831-rc29/PRODUCTION_SWITCH_HANDOFF.md`。其中只允许替换 `new-api` 应用镜像并执行 `--no-build --pull never --no-deps`，禁止触碰 PostgreSQL/Redis、数据卷或执行 `down -v`。
- 本轮仅完成备份、配置预置和交接材料生成，未重启或重建正式容器；正式切换需另行安排中断窗口并完成健康、登录、聚合接口和实际 Codex 流量验证。
