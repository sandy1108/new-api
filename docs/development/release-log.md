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

## 2026-08-31：交接包管理机制

### 规范与目录

- 管理规范：`docs/development/handoff-management.md`。
- 版本库入口与模板：`docs/development/handoffs/README.md`、`docs/development/handoffs/templates/`。
- 私有实际交接包统一放在工程同级 `.backups/new-api/handoffs/<handoff-id>/`；历史 `pre-switch-*`、`pre-sync-*` 和 `release-*` 目录由扫描器兼容。
- 每个包包含 `handoff.json`、任务说明（通常为 `HANDOFF.md`）和同目录 `EXECUTION_FEEDBACK.md`。私有备份、凭据和运行态证据不进入 Git。

### 扫描器与状态

- 扫描脚本：`scripts/scan-handoffs.py`；测试：`scripts/test_scan_handoffs.py`。
- `python3 scripts/test_scan_handoffs.py`：7 项测试通过。
- `python3 scripts/scan-handoffs.py`：当前发现 2 个包，`completed=1`、`pending=1`，`invalid=0`。
- 已为历史首次本地镜像切换包补齐元数据并登记为 `completed`；生产候选 `rc.29` 切换包补齐元数据和待执行反馈，保持 `pending`，没有伪造执行结果。
- Codex 负责创建和填写 `pending` 发包材料；豆包负责执行、更新状态并填写同目录反馈。最终回复必须同时提供可点击和纯文本的绝对路径。
- 本轮只读复核发现生产 `new-api` 已运行候选 `rc.29` 镜像且为 `healthy`，但未观察到执行 Agent 回填该包；因此 `new-api-pre-switch-20260831-rc29` 仍按元数据保持 `pending`，不能仅凭容器状态推断完整验收已完成。

## 2026-08-31：Web 控制台用量统计页开发与隔离回归

### 开发范围

- 开发 Worktree：`new-api-development`。
- 分支：`upgrade/upstream-main-20260831`，基线为已同步官方主线的个人分支；开发阶段验证 HEAD（提交前快照）：`812702ba`。
- 新增独立前端功能模块 `web/src/features/usage-summary/`，通过现有认证 API 客户端调用服务端聚合接口，不复制 Chrome 插件的 Cookie、分页或外部访问逻辑。
- 新增认证路由 `/usage-summary` 和侧边栏入口；普通用户固定使用 `GET /api/log/self/usage-summary`，管理员可在“全部/仅自己”之间切换并使用有界时间范围。
- 保留原有 `/usage-logs/common` 分页页面和服务端分页上限；本阶段未修改生产 Compose、生产容器、生产数据或 Redis/PostgreSQL。

### 验证结果

- `npx vitest run`：65 个测试文件、434 个测试全部通过（Node.js 24.18.0 运行时）。
- `npx tsgo -b`：通过。
- `npx oxlint -c .oxlintrc.json <本次涉及文件>`：通过；全仓 Lint 仍有官方既有文件的错误，未将无关文件纳入本次修复。
- 本功能涉及文件的 `oxfmt --check`：通过；全量 `npm run format:check` 仍列出 25 个官方既有文件的格式问题，未修改无关文件。
- `git diff --check`：通过。
- 开发 Compose `docker compose -f docker-compose.dev.yml config`：通过；项目名 `new-api-dev`，应用端口 `3000`、开发数据卷和生产隔离。
- 开发 API：`/api/status` HTTP 200 且 `success=true`；两个聚合接口无 Token 均返回 HTTP 401；开发容器、PostgreSQL、Redis 均保持运行。

### 当前构建边界（历史阻塞记录）

- 完整 `rsbuild build` 和 `npm run dev` 在入口解析阶段被当前开发环境缺失的 `@lobehub/ui`、`antd` peer 依赖阻塞；这些依赖虽出现在 `bun.lock`，但不在当前 `web/package.json`/`node_modules` 可用集合中。本阶段未擅自安装依赖或修改锁文件。
- 该宿主机阻塞不影响仓库原生 Dockerfile 的完整镜像构建；页面级回归使用已构建的隔离镜像和临时端口完成，具体证据见下方“浏览器页面回归”。

### 结果

- Web 用量统计页的源码、路由、导航、i18n、测试、设计文档和实施计划已在本开发分支完成；生产切换仍需单独审批，Git 发布动作按用户确认执行。

### 浏览器页面回归与临时资源清理（2026-08-31）

- 回归镜像：`new-api:dev-20260831-02-g812702ba`，摘要 `sha256:1d92a9df44e1a752908cf2fd7790b970d55727d6a2e0dd4056c4ef924ef5c1c0`。
- 在临时隔离栈（应用 `new-api-web-e2e-app`、端口 `3311`、独立 PostgreSQL/Redis/网络/数据卷）中，管理员登录后打开 `/usage-summary` 成功，页面标题、筛选控件、空状态和刷新控件均正常。
- 默认“全部”请求 `/api/log/usage-summary`；切换“仅自己”请求 `/api/log/self/usage-summary`；今天、昨天、本周、上周、本月、上月、本季度七个范围均各触发一次对应聚合请求；刷新仅再次请求当前范围。九次聚合响应均为 HTTP 200，页面交互期间未调用旧 `/api/log/` 分页接口。
- `/usage-logs/common` 仍可打开，观察到原有 `/api/log/stat` 和 `/api/log/` 请求，证明旧日志页未被新页面替换。
- 回归结束后已删除临时应用、PostgreSQL、Redis 容器及其独立网络和数据卷；候选镜像保留用于后续复核。生产 `new-api`、Redis、PostgreSQL、Compose 和数据未触碰。

### 当前阶段结论

- Web 控制台 Token 用量页面已完成源码、自动化测试和隔离浏览器回归；设计文档和本发布日志已与实际进度对齐。
- 发布前差异已完成审阅；用户已确认 Git 发布动作，commit、合并和推送按本项目分支规范执行。Git 发布不等同于生产切换，生产仍需单独审批。

### 交接包状态复核（2026-08-31）

- 重新执行 `python3 scripts/test_scan_handoffs.py` 和 `python3 scripts/scan-handoffs.py`：共发现 3 个交接包，`completed=3`、`pending=0`、`invalid=0`；生产候选 `rc.29` 交接包已由执行 Agent 回填完成。
- 该扫描结果只代表交接包元数据和反馈文件状态；生产运行态仍以容器和 Compose 的独立只读检查为准。

## 2026-08-31：Web 用量统计页三级明细 UI 修正

- 用户确认采用数据看板风格后，进一步明确一级明细必须是“API 令牌”，不能把“Token”作为含义不清的维度名称。
- 参考 Chrome 插件 `newapi-analysis-chrome-ext` 的 `renderRows()`、`renderDetail()` 和 `detail-state.js`，将 Web 页面交互调整为：API 令牌汇总 → 选中 API 令牌 → 渠道汇总 → 选中渠道 → 渠道内模型汇总。
- 移除跨渠道混合的模型合计；切换 API 令牌或渠道只改变前端选择状态，不增加聚合接口请求。
- 汇总卡片、令牌表、渠道指标和模型表使用中文 `千/万/亿` 紧凑单位，并通过 HTML `title` 保留完整数值。
- 视觉预览文件保存在开发 Worktree 的 `.superpowers/brainstorm/7046-1788173052/content/usage-summary-v1.html` 和 `usage-summary-v2.html`，仅用于本地预览，不进入生产镜像。

### 本次源码验证

- `npx vitest run src/features/usage-summary/__tests__`：6 个测试文件、32 个测试通过。
- `npx tsgo -b`：通过。
- `npx oxlint -c .oxlintrc.json src/features/usage-summary`：通过。
- `npx oxfmt -c .oxfmtrc.json --check`（本次涉及 9 个前端文件）：通过。
- `git diff --check`：通过。
- 七个 locale 的翻译键顺序和数量已核对一致；本次新增英文、简体中文、繁体中文文案，其他语言暂以英文回退值补齐键。
- 仅修改开发 Worktree 的 usage-summary 组件/测试、i18n 和文档；未修改生产 Compose、生产容器、数据库或 Redis。

## 2026-08-31：独立测试环境部署与演示数据

- 测试资源目录：`../.backups/new-api/ui-test-20260831-03/`（相对 `ServiceTools/new-api-development`）。
- 测试 Compose、环境变量和演示数据脚本均与 Git 源码、生产 Compose、`new-api-dev` 分离；`test.env` 与 `seed-demo.sql` 保持本机权限 600，不纳入版本库。
- 测试项目/容器/网络：`new-api-ui-test`、`new-api-ui-test-app`、`new-api-ui-test-pg`、`new-api-ui-test-redis`、`new-api-ui-test-network`；应用端口 `3312:3000`，PostgreSQL/Redis 未暴露宿主机端口，数据卷独立。
- 使用镜像：`new-api:dev-20260831-03-g4c647d35-dirty`；摘要 `sha256:b34597c98470700618fbf5f81b9f7b9a0f2714a072d59c92ca66372c048e974c`，架构 `linux/arm64`。
- 测试管理员由 `/api/setup` 创建，账号和密码仅记录在测试目录 README；登录加密开关确认有效（`PASSWORD_LOGIN_ENCRYPTION_ENABLED=true`）。
- 演示数据：2 个 API 令牌、4 个渠道、19 条 `type=2` 消费日志和 1 条 `type=3` 非消费日志；覆盖今天、昨天、近一周和更早日期。
- 聚合接口验证：全量与个人接口均返回 `success=true`，全量结果为 19 requests、67,900 tokens、752,000 quota、8 个令牌/渠道/模型明细；时间范围和非法时间范围验证通过。
- 加密网页登录、`/api/user/self`、管理员/个人聚合接口验证通过；自动化验证使用的临时 PAT 已清除。
- 首次在应用运行后注入渠道数据时，后台批量能力尚未生成，触发一次已有 `InitChannelCache` nil-map 竞态；应用自动恢复，随后仅重启测试应用并确认已有数据下启动及后续同步无 panic/fatal/error。
- 生产 `new-api`（`new-api:v1.0.0-rc.29-20260831-02-g4c647d35`）和既有 `new-api-dev` 均保持运行，本次未停止、未重建、未修改其 Compose、数据库、Redis 或数据卷。

### UI 修正候选镜像更新（2026-08-31）

- 针对三级明细交互修正重新构建候选镜像：`new-api:dev-20260831-04-g4c647d35-dirty`。
- 镜像摘要：`sha256:b7084d227a3ca6748036c9267579bf0dd772557c46f8e2ae34a64d1eba225fd9`，架构 `linux/arm64`。
- 仅使用测试 Compose 重建 `new-api-ui-test-app`；测试 PostgreSQL/Redis、生产 `new-api`、`new-api-dev` 及生产 Compose 均未触碰。
- 测试应用端口仍为 `3312:3000`，当前容器状态为 `running` + `healthy`；`/api/status` 返回 `success=true`。
- 自动化测试、类型检查、Lint、格式检查和 Docker 构建已完成；令牌与渠道点击切换已在后续测试环境回归中确认，边缘裁剪验收见下方 `dev-05` 小节。

### 面板边缘裁剪样式修正候选镜像（2026-08-31）

- 根因定位为统计页内部 `overflow-auto` 滚动层紧贴内容边界，卡片的边框、圆角和阴影绘制区域在左右两侧被裁剪。
- 仅修改 `web/src/features/usage-summary/index.tsx` 的统计页滚动层，增加 `p-0.5 sm:p-1` 内缩；未修改全局 `SectionPageLayout`，避免影响其他页面。
- 新候选镜像：`new-api:dev-20260831-05-g4c647d353-dirty`。
- 镜像摘要：`sha256:764e8afe8104781d1056bbc95fd78451e6ac26f4c3e314a624b8c8c2ac41a3e6`，架构 `linux/arm64`。
- 仅重建测试应用 `new-api-ui-test-app`，测试 PostgreSQL/Redis、生产 `new-api`、`new-api-dev` 及生产 Compose 均未触碰。
- 用量统计模块 35 个测试、前端全量 441 个测试、类型检查、Lint、格式检查和 Docker 构建均通过；用户刷新测试页后确认面板左右边缘完整，本次 UI 修正通过人工验收。

## 2026-09-06：官方主线同步 20260906 开发分支验证

### 同步范围

- 官方基线：`2b6f1dfe`（v1.0.0-rc.29）→ `origin/main` 最新 `49ec46966`，共 23 个官方提交，跨 rc.30/rc.31/rc.32/rc.33。
- 开发 Worktree：`new-api-development`，新分支 `upgrade/upstream-main-20260906`，merge 提交 `815e5869b`（`merge: 同步官方主线 20260906`）。
- 同步前备份分支：`backup/pre-sync-20260906-185020`（`0b487787`）。
- merge 无冲突；个人提交完整保留（`origin/main..HEAD` = 23，即 22 个个人提交 + 1 个 merge 提交）。

### 重点官方变更

- 数据库迁移：`27ff6a876` migrate legacy token key constraints（含官方迁移测试）。
- 计费：gpt-6-astra 内置表达式计价、hosted-tool 转换与计费完整性系列。
- 日志：LogOther 投影重构、特权元数据隔离、usage statistics quota 修复（与个人用量统计功能重叠区）。

### 验证结果

- Go 根模块全量测试（`golang:1.26.1-alpine` 容器，`GOWORK=off`）：通过。
- `relaykit` 独立构建（`GOWORK=off go build ./...`）：通过。
- 前端 `vitest`：66 个文件 443 个测试全部通过；`tsgo -b`：通过。
- 三库迁移验证：SQLite 内存库；临时容器真实 MySQL 8.0 与 PostgreSQL 15，`TestMigrateTokenKey` 全场景通过，`./model` 带真实 DSN 全量通过；临时容器与网络已清理。
- 开发镜像：`new-api:dev-20260906-01-g815e5869b`（`sha256:65ec7011ee212ee2e5310a81477c24fc430e7be1920e143d8601f43cb8607739`，linux/arm64，241MB）。因 Docker VM 仅约 2GB 内存，使用临时 `Dockerfile.syncbuild` 在 Go 构建阶段追加 `GOMAXPROCS=1 GOFLAGS=-p=1` 串行构建；官方 `Dockerfile` 未改动，临时文件已删除。
- 隔离栈验证（`new-api-sync-verify`，`127.0.0.1:3313`，独立 PG15/Redis/数据卷）：
  - `/api/status` 返回 `success=true`；无 Token 请求 `/v1/models` 与两个聚合接口均返回 401。
  - `PASSWORD_LOGIN_ENCRYPTION_ENABLED=true` 下 RSA-OAEP 加密登录成功；JWT Bearer 调用 `/api/log/self/usage-summary` 与 `/api/log/usage-summary` 均 HTTP 200（个人功能正常）。
  - 应用重启两次 `/api/status` 均为 `success=true`，容器日志无 panic/fatal/迁移错误（新库迁移幂等）。
  - 验证后临时栈、网络与数据卷已全部删除；测试账号仅存在于已删除的临时卷中。

### 边界与待办

- 未合入 `personal/main`、未推送 `myfork`、未构建正式候选镜像、未触碰生产栈。
- 待办：用户审批后合入并推送；生产发布前建议补充升级路径（旧库→新库）应用级验证与 `/usage-summary` 浏览器回归。

## 2026-09-06：同步分支合入 personal/main 与正式候选镜像验证

### Git 合入与推送

- `release-log.md` 记录提交 `6bebe63db`（docs: 记录 20260906 官方主线同步开发验证结果）。
- `personal/main` 在生产 Worktree 以 `--ff-only` 从 `0b487787` 前进到 `6bebe63db`，工作区保持干净。
- 已推送 `myfork`：`personal/main` 与 `upgrade/upstream-main-20260906` 均为 `6bebe63db`；`myfork/main` 官方镜像同步至 `49ec46966`。

### 正式候选镜像

- 构建目录：`new-api-production`（`personal/main` @ `6bebe63db`）。
- 候选镜像：`new-api:v1.0.0-rc.33-20260906-01-g6bebe63db`，镜像 ID `sha256:65ec7011ee212ee2e5310a81477c24fc430e7be1920e143d8601f43cb8607739`，linux/arm64，241MB。
- 该镜像与开发镜像 `new-api:dev-20260906-01-g815e5869b` 为同一镜像 ID（同一源码产物），前一轮隔离栈验证结果直接适用于本候选镜像。
- 构建同样使用临时 `Dockerfile.syncbuild` 串行参数，官方 `Dockerfile` 未改动，临时文件已删除。

### 升级路径验证（旧库 → 新候选镜像）

- 临时隔离栈：`new-api-upgrade-verify`（`127.0.0.1:3314`，独立 PG15/Redis/数据卷）。
- 旧库建立：旧生产镜像 `new-api:v1.0.0-rc.29-20260831-03-g0b487787` 启动，创建管理员、1 个渠道、1 个令牌，直插 2 条 `type=2` 消费日志（quota 合计 4,560）。
- 切换新候选镜像启动：`/api/status` `success=true`；既有数据完整保留（日志 2 条/渠道 1/令牌 1）；旧密码加密登录成功；管理员聚合接口返回 HTTP 200 且请求数 2、quota 4,560，与旧库一致。
- 重启第二次启动 `success=true`，日志无 panic/fatal；验证后临时栈、网络与数据卷已全部删除，测试账号随卷销毁。

### 浏览器回归（/usage-summary）

- 新候选镜像 + 升级后的旧库数据，真实浏览器管理员登录后打开 `/usage-summary`：统计卡片（请求数 2、输入 930、输出 264、总 1,194、额度 4,560）与旧库一致；趋势图、API 令牌分布、三级明细（API 令牌 → 渠道 → 渠道内模型）渲染和交互正常，中文紧凑单位与占比正常。
- 截图确认卡片边框与面板边缘完整，无边缘裁剪回归。回归后浏览器页签关闭。

### 边界

- 生产容器、生产 Compose、生产数据（`.volumes/new-api/`）本轮全程未触碰；生产仍运行 `new-api:v1.0.0-rc.29-20260831-03-g0b487787`。
- 生产切换需用户另行确认后按交接包流程执行。

## 2026-09-06：生产切换至 rc.33-20260906-01

### 切换前备份

- 备份目录：`.backups/new-api/release-20260906-sync-rc33/`。
- PostgreSQL dump（`pg_dump -Fc`，3.05MB）SHA-256：`13bc5159d79516cd5e257dce10ec9e1c1419ee9cb62e98747616029cfc33a01a`。
- 切换前 Compose 快照 SHA-256：`31c0cc6c9e1f2e9cc48b8f46de2f706f2f7c242950b15d9faaaea76168508672`。
- 切换前运行镜像：`new-api:v1.0.0-rc.29-20260831-03-g0b487787`（Up 2 days, healthy）。

### 历史状态勘误

- 切换前检查发现 `new-api:v1.0.0-rc.29-20260831-03-g0b487787` 标签当前指向镜像 ID `sha256:764e8afe8104…`，与 `new-api:dev-20260831-05-g4c647d353-dirty`（用量统计面板边缘裁剪修正候选）相同，即上一轮生产容器实际运行的是 -05 dirty 构建而非标签原始构建。该标签仍指向生产切换前实际运行的镜像，按标签回滚路径有效。历史操作与记录不一致的原因已无法追溯，仅在此登记勘误。

### 切换操作

- 生产 Compose（`new-api/docker-compose.yml`）仅镜像行变更为 `new-api:v1.0.0-rc.33-20260906-01-g6bebe63db`（diff 全文仅此一行）。
- `docker compose up -d --no-build --pull never --no-deps new-api` 只重建应用容器；PostgreSQL、Redis、Nginx、数据卷均未触碰（切换后仍为 Up 2 days）。

### 切换后验证

- 容器 `new-api` 运行新镜像（ID `sha256:65ec7011ee21…` 与候选一致），状态 `running + healthy`。
- 容器内 `/api/status` 返回 `success=true`；无 Token 请求 `/v1/models` 与 `/api/log/usage-summary` 均返回 401。
- Nginx HTTPS 公网入口 `/api/status` 返回 `success=true`。
- 应用日志无 panic/fatal/迁移错误。
- 管理员登录与实际 Codex 流量验证待用户自行确认；数据库迁移（token key 约束）已在升级路径验证中确认幂等且向后兼容。

### 回滚方案

- 镜像行改回 `new-api:v1.0.0-rc.29-20260831-03-g0b487787` 后再次 `up -d --no-build --pull never --no-deps new-api` 即恢复切换前状态；数据库备份位于上述备份目录。

### 用户最终验证（2026-09-06）

- 用户已确认生产环境运行正常：管理员登录与实际 Codex 流量均通过验证。
- 本次同步（rc.29 → rc.33+2，生产镜像 `new-api:v1.0.0-rc.33-20260906-01-g6bebe63db`）正式收尾，无遗留待办。

## 2026-09-07：官方主线同步至 6298b0f 开发回归

### 同步范围

- 官方远端：`origin/main`，目标提交 `6298b0f3238461b9629dfc1c00866f8325123aa1`。
- 开发 Worktree：`new-api-development`，升级分支 `upgrade/upstream-main-20260907`。
- 同步方式：以当前个人维护基线执行 `git merge --no-ff --no-commit origin/main`，解决前端菜单/路由冲突后提交合并结果。
- 合并提交：`c593db418`（`merge: 同步官方主线并保留用量统计改造`）。
- 生产 Worktree、生产 Compose、生产容器、生产 PostgreSQL/Redis 与正式数据本轮均未触碰。

### 冲突与个人改造保留

- 保留个人 `Token Usage` 页面和两个日志用量聚合接口：`/api/log/usage-summary`、`/api/log/self/usage-summary`。
- 合入官方 `Audit Logs`、`Security & Access`、统一登录验证/安全审计、模型供应商与价格管理、Responses/Relay 修复及数据库迁移改动。
- 侧边栏顺序调整为 `Usage Logs → Audit Logs → Token Usage`；个人统计页不再覆盖官方审计和安全入口。
- 为两个官方前端测试补充 `ApiRequestConfig` 类型标注，解决 `tsgo` 对 `config.params` 的推断问题；未修改业务逻辑。

### 源码验证

- Go：在 `golang:1.26.1-alpine` 临时容器中执行 `GOWORK=off go test ./...` 与 `GOWORK=off go build ./...`，两项退出码均为 `0`。
- 前端：Vitest `95/95` 测试文件、`710/710` 测试通过；`bun run typecheck` 与 `bun run build` 通过。
- 本次涉及文件的定向 Oxlint/Oxfmt 检查通过；全仓 `format:check` 仍被官方既有文件（包括根 `AGENTS.md` 和多个既有组件）列出的格式差异拦截，未对无关文件做格式化。
- `git diff --check` 通过；合并后工作区仅保留既有未跟踪的 Superpowers 预览/计划、依赖目录和本地锁文件，均未纳入提交。

### 当前门槛与后续

- 当前只完成开发分支源码合并和主机/容器化编译验证，尚未构建新的开发镜像或切换任何运行容器。
- 已使用合并后文档提交固定源码构建不可变开发镜像：`new-api:dev-20260907-01-g8e5246350`；镜像 ID `sha256:ba933112636ea3062420e491b8ace80235fda206bc63df7bbe292135b2bb1d97`，架构 `linux/arm64`，大小约 242.6 MB，构建时间 2026-09-07 23:03（Asia/Shanghai）。构建阶段使用临时 `Dockerfile.syncbuild` 串行 Go 编译参数，构建完成后已删除临时文件，官方 `Dockerfile` 未修改。
- 下一步在独立 Compose 项目、端口、PostgreSQL、Redis、数据目录中执行“旧镜像建库 → 新镜像升级”的回归。
- 测试重点：新迁移幂等、旧数据保留、`PASSWORD_LOGIN_ENCRYPTION_ENABLED=true`、后台登录、`/api/status`、`/v1/models`、`/v1/responses`、两个用量聚合接口以及 Web `/usage-summary`。测试完成前不合入 `personal/main`、不推送远端、不触碰生产。

### 隔离升级路径验证

- 测试 Compose：`.backups/new-api/upgrade-verify-20260907/docker-compose.yml`；项目、网络、应用端口 `3315`、PostgreSQL、Redis、应用数据卷和数据库卷均独立于生产及既有开发栈。
- 旧基线镜像：`new-api:v1.0.0-rc.33-20260906-01-g6bebe63db`；先完成初始化并写入合成数据：1 个管理员、2 个 API 令牌、2 个渠道、3 条 `type=2` 消费日志、1 条 `type=3` 管理日志。
- 切换到新镜像 `new-api:dev-20260907-01-g8e5246350` 后，用户/令牌/渠道/日志行数保持 `1/2/2/3/1`；新迁移创建 `audit_logs`，并补充 `users.access_token_created_at`，登录审计记录正常生成。
- `PASSWORD_LOGIN_ENCRYPTION_ENABLED=true` 下，RSA-OAEP 登录成功；`/api/status` `success=true`、`setup=true`；管理员和个人聚合均返回 3 请求、1,500 输入 Token、450 输出 Token、1,950 总 Token、6,900 quota；管理员明细 3 条。
- API 冒烟：带合成 API 令牌的 `/v1/models` HTTP 200；未认证 `/api/log/usage-summary`、`/api/log/self/usage-summary` 和 `POST /v1/responses` 均按预期 HTTP 401；Web 根页面 HTTP 200。
- 新镜像连续重启 2 次均恢复 `/api/status`；PostgreSQL/Redis 容器 ID 未变化；迁移/启动错误扫描无 fatal、迁移失败或 panic。首次切换日志曾出现一次 `InitChannelCache panic: assignment to entry in nil map, retrying once`，应用自动重试并完成渠道同步；这是现有可恢复竞态的观察项，不等同于迁移失败，正式切换前仍应继续观察。
- Playwright CLI 尝试进行真实浏览器登录/`/usage-summary` 页面回归，但本机 Chrome 在启动阶段收到 `SIGTRAP` 退出，未形成新的浏览器 DOM 证据；不将该项表述为已完成的浏览器人工验收。前端自动化测试、类型检查和 Rsbuild 构建仍已通过。

### 当前状态

- 隔离升级验证栈目前保留运行，便于用户从 `http://127.0.0.1:3315` 做人工页面检查；测试账号和合成数据仅存在于该独立卷，不与生产共享。
- 本轮未合入 `personal/main`、未推送 `myfork`、未构建正式候选镜像，生产容器和生产数据全程未触碰。正式发布前仍需用户确认是否采用该开发镜像，并补做可用浏览器环境下的 `/usage-summary` 页面验收及实际 Agent 流量回归。

## 2026-09-08：官方主线同步至 rc.35 与隔离测试回归

### 同步与源码

- 官方远端：`origin/main=0e0ba152bdcc6891f6053047ccf14d41b3cad60a`（`v1.0.0-rc.35`）。
- 开发/升级分支：`upgrade/upstream-main-20260908`，合并提交 `40d5fd64c682f6799934fd10c199448ed8b20008`；`personal/main` 已快进到同一提交并推送到 `myfork`。
- 本轮主要吸收官方插件系统与插件图标、模型计价编辑器和 Web 管理页面改动；未发现新的数据库迁移文件，现有迁移在隔离 PostgreSQL 中启动两次均幂等。
- 官方新增测试中有两项标签断言与当前 UI 命名不一致，已修正为 `Additional charge` / `Unit price`；未改动业务逻辑。

### 候选镜像

- 候选镜像：`new-api:v1.0.0-rc.35-20260908-01-g40d5fd64`。
- 镜像摘要：`sha256:8deb71a40a9245ae29a457d6f591c409deae69e37c9ff148540d452c00ddc994`，架构 `linux/arm64`。
- 构建日志：`.backups/new-api/release-20260908-upstream-rc35/build.log`。
- 采用本项目既定的“上游版本-日期-当日序号-短哈希”不可变标签；本轮没有覆盖旧标签或使用 `latest`。

### 3316 隔离测试环境

- Compose：`.backups/new-api/upgrade-verify-20260908/docker-compose.yml`；应用绑定 `127.0.0.1:3316`，PostgreSQL、Redis、应用 `/data` 卷、网络均为独立资源。
- `PASSWORD_LOGIN_ENCRYPTION_ENABLED=true` 已显式设置；测试账号只保存在隔离目录 README，不写入 Git、生产配置或本日志。
- 合成数据：1 个管理员、2 个 API 令牌、4 个渠道、8 个能力记录、19 条 `type=2` 消费日志和 1 条 `type=3` 非消费日志。
- 演示脚本已在 Git 外修正：显式写入 `abilities`，并按 TokenAuth 约定将库内测试 key 保持为不含 `sk-` 的形式；这避免测试脚本制造无效渠道缓存状态。原先直接插入渠道且缺少 abilities 时触发的 nil-map panic 属于测试 fixture 问题，不是 rc.35 新增回归。

### 回归结果

- 加密登录公钥接口 HTTP 200；RSA-OAEP 登录 HTTP 200；`/api/user/self` 返回测试管理员。
- 管理员聚合（含趋势）：19 请求、42,400 输入 Token、25,500 输出 Token、67,900 总 Token、752,000 quota、8 个明细、7 个日趋势点。
- 当前用户聚合在附带恶意 `username=intruder` 时仍严格返回认证用户上述 19 条数据；临时跨用户探针已删除。
- 闭区间单秒筛选返回 1 请求、3,000 输入、1,800 输出、4,800 总 Token、60,000 quota；`include_trend=false` 不返回趋势；非法时间范围返回 `success=false` 与 `invalid time range`。
- 未认证 `/api/log/usage-summary` 与 `/api/log/self/usage-summary` 均 HTTP 401；带测试 API 令牌的 `/v1/models` HTTP 200，返回 5 个 OpenAI 模型。
- 应用连续重启两次；每次 `/api/status` 均为 `success=true`，第二次重启后跨过完整 60 秒渠道同步周期仍为 `running`，无新的 panic/fatal/迁移错误。PostgreSQL、Redis 容器未重建，数据行数保持 `1/2/4/8/19/1`。
- Go 全量测试/构建、前端 109 个测试文件 903 个测试、TypeScript 类型检查和前端构建均已通过；全仓 Oxlint 仍保留官方及历史既有错误，未扩大修改范围。

### 正式环境边界与下一步

- 生产容器当前仍运行 `new-api:v1.0.0-rc.33-20260906-01-g6bebe63db`，状态 `running + healthy`；生产 Compose、PostgreSQL、Redis、数据卷本轮未触碰。
- 本轮仅完成源码同步、候选镜像构建和 3316 隔离回归；尚未切换生产，也未对生产数据库执行任何迁移。
- 下一步可按交接包流程执行生产切换；交接包初始状态必须保持 `pending`，由执行 Agent 回填实际备份、切换、验收和回滚结果。

## 2026-09-08：rc.35 生产切换完成

### 发布对象与备份

- 生产源码 Worktree：`new-api-production`；切换时 `personal/main` HEAD=`5f291fb98528ca4b7912cb5d55310e00352f5b6e`，运行代码对应父提交 `40d5fd64c`。
- 候选镜像：`new-api:v1.0.0-rc.35-20260908-01-g40d5fd64`，摘要 `sha256:8deb71a40a9245ae29a457d6f591c409deae69e37c9ff148540d452c00ddc994`，架构 `linux/arm64`。
- 切换前备份目录：`.backups/new-api/release-20260908-rc35-switch/`；PostgreSQL 自定义格式 dump 的 SHA-256 为 `4ffa1fb2f2bb929c0506f3dde3cb3b5038e4d56351237113ddf9e211d364775c`。
- 交接包：`.backups/new-api/handoffs/new-api-pre-switch-20260908-rc35/`，状态已由执行 Agent 回填为 `completed`。

### 切换边界

- 生产 Compose 仅将 `new-api` 镜像从 rc.33 改为上述 rc.35 候选；`PASSWORD_LOGIN_ENCRYPTION_ENABLED=true`、绝对数据路径、凭据引用、网络、端口和容器名均保持不变。
- 执行 `docker compose up -d --no-build --pull never --no-deps new-api`，只重建应用容器；PostgreSQL、Redis、Nginx、FRP、数据卷和生产数据均未触碰，也未执行 `down`、`down -v`、`prune` 或重新拉取 `latest`。
- 本候选不新增数据库迁移；启动日志显示迁移正常完成，未发现迁移失败、FATAL 或 panic。

### 上线验收

- `new-api` 容器为 `running + healthy`，运行镜像摘要与候选一致；PostgreSQL/Redis 容器 ID 与切换前一致。
- `/api/status` 返回 `success=true`、`setup=true`、`password_login_encryption_enabled=true`、`self_use_mode_enabled=true`。
- 外部 `/login`、`/usage-summary`、旧 `/usage-logs/common` 返回 HTTP 200；未认证的两个聚合接口与 `/v1/models` 按预期返回 HTTP 401；真实认证态聚合接口已由用户测试通过。
- 交接反馈阶段未主动制造真实业务请求；切换后客户端实际 `/v1/responses` 请求已观察到 HTTP 200，流式路由正常。
- 是否回滚：否。候选镜像、rc.33 回滚镜像、Compose 快照和 PostgreSQL dump 均保留，可按交接包中的回滚步骤复核。

## 2026-09-11：官方主线同步至 rc.36 后的开发环境回归

### 同步范围

- 官方远端：`origin/main=bdef117505247769268b209665fb3ad7554c3da7`，最近发布为 `v1.0.0-rc.36`。
- 开发 Worktree：`new-api-development`，分支 `upgrade/upstream-main-20260910`。
- 同步方式：以个人维护基线 `cb904fe12` 合并官方主线，合并提交 `b72243cee8bddcf7b83060c4e9cf5ffabff6a6ab`；父提交为 `cb904fe12` 与 `bdef11750`。
- `personal/main`、生产 Worktree、生产 Compose、生产容器、生产 PostgreSQL/Redis 与正式数据本轮均未触碰。

### 源码验证

- Go：在 `golang:1.26.1-alpine` 临时容器中执行 `GOWORK=off go test ./...` 与 `GOWORK=off go build ./...`，两项退出码均为 `0`。
- 前端：Vitest 单进程 `124/124` 测试文件、`1174/1174` 测试通过；`pnpm typecheck` 与 `pnpm build` 通过。
- 全量前端并发运行曾出现 2 个异步 UI 测试波动失败；两个失败文件单独复跑及整套单进程复跑均通过，未修改测试或业务代码掩盖问题。
- 本次上游新增的 `options` 主键修复迁移未与个人用量统计改造冲突；个人接口和页面文件均保留。

### 候选镜像

- 镜像：`new-api:v1.0.0-rc.36-20260911-01-gb72243cee`。
- 本地镜像 ID：`sha256:fb00ad54fdf482891b6ba40e2a4335115eb4e90d6602e72033c35e5f16533566`。
- 架构：`linux/arm64`；构建使用同步分支完整 `Dockerfile`，未使用开发占位前端镜像。

### 3317 隔离升级回归

- Compose：`/Users/zhangyipeng/MyCodingSpace/ServiceTools/.backups/new-api/upgrade-verify-20260911/docker-compose.yml`。
- 应用地址：`http://127.0.0.1:3317`；基线镜像为已验证的 rc.35，先初始化旧数据库并写入合成数据，再只替换 New API 应用镜像为本候选。
- 测试数据卷、PostgreSQL、Redis、网络和容器名均使用 `new-api-upgrade-verify-20260911-*`，未复用 3315、3316、生产资源。
- 升级前 PostgreSQL dump：`.backups/new-api/upgrade-verify-20260911/postgres-before-upgrade.dump`；SHA-256：`94092c95754c9937a7b535f13f743e2275ca56b26474f71adcde09ac4903d077`。
- 升级前后关键行数保持：`users=1`、`tokens=2`、`channels=4`、`abilities=8`、`logs=20`、`options=3`；新版本新增的 `audit_logs` 表正常生成。
- `options` 迁移后存在 `options_pkey PRIMARY KEY (key)`，重复 key 为 `0`；应用连续重启两次，迁移与启动均无错误，PostgreSQL/Redis 容器 ID 未变化。

### 回归结果

- `PASSWORD_LOGIN_ENCRYPTION_ENABLED=true` 下，RSA-OAEP 登录成功；`/api/status` 返回 `success=true`、`setup=true`。
- 管理员及当前用户聚合均返回合成数据的 `19` 条消费请求、`67,900` 总 Token、`8` 个明细和 `7` 个趋势点；当前用户附带其他用户名参数仍只返回当前用户数据。
- 未认证的两个聚合接口、`/v1/models` 和 `POST /v1/responses` 均按预期拒绝；带合成 API 令牌的 `/v1/models` 返回 HTTP 200、7 个模型。
- `/`、`/login`、`/usage-summary`、旧 `/usage-logs/common` 均返回 HTTP 200。
- 带合成渠道调用 `POST /v1/responses` 会因渠道地址为 `example.invalid` 返回上游请求错误；这是隔离 fixture 的预期边界，不作为真实供应商流量验收结论。

### 当前状态与下一步

- 3317 隔离回归栈目前保留运行，供人工打开页面检查；测试账号和密码只记录在隔离目录 README，不写入 Git 或本日志。
- 本轮尚未合入 `personal/main`、尚未推送 `myfork`、尚未切换生产；正式发布前仍需用户确认采用该候选镜像，并按生产交接包重新备份和验收。
