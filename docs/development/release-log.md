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
- 当前测试镜像标签：`new-api:dev-20260830-02-g8454082-dirty`（Compose 使用别名 `new-api-dev:local`）。
- 当前镜像 ID：`sha256:b975be1195d842984c4db46ff3c40583bf594448cc7ae683ae4a7afd29ed8b48`，构建时间：2026-08-30 04:02:34（Asia/Shanghai），架构：`linux/arm64`。
- 首次标准 `docker compose up --build` 因 Docker 构建内存不足，在 Go 编译 `ch-go/proto` 时被 OOM killer 终止；未替换旧测试容器。随后使用一次性串行编译（`GOMAXPROCS=1`、`GOFLAGS=-p=1`）构建当前测试镜像，未修改仓库 Dockerfile。

### 验证结果

- `GOWORK=off go test ./model ./controller ./router`：通过。
- `go vet ./model ./controller ./router`：通过（在临时 Go 容器内执行）。
- 根模块全量测试与独立 `relaykit` 模块测试：全部通过。
- 管理员端到端汇总（合成数据）：4 请求、37 输入 Token、16 输出 Token、53 总 Token、420 quota。
- 管理员筛选、普通用户强制用户范围（忽略恶意 `username`）、时间范围校验和未认证 401：均符合预期。
- 测试应用重建后渠道名称从缓存正确返回；测试 PostgreSQL healthy、Redis 正常、`/api/status` HTTP 200。
- 当前测试容器 `new-api-dev` 正运行 `new-api-dev:local`，其镜像 ID 与上述 `-02` 候选一致；未触碰生产容器。
- 生产 `new-api` 仍运行 `new-api:rc-8454082f930f`，镜像 ID `sha256:30b1f6b7c3688cda93fb171ae6f84c0197e2904b1b69abbce21259609fd26b3e`，状态 `running + healthy`；生产 Redis、PostgreSQL、Nginx 未触碰。

### 当前状态

- 功能代码、测试、设计文档和 Postman 集合已形成提交 `064a107821e35ce0778ffa42230f477aab4fe27a`；本节记录的是隔离测试阶段的结果。
- 隔离测试完成后，生产发布按独立备份、评审和审批流程执行，结果见下方“日志用量聚合接口生产切换”章节。
- Chrome 插件接入仍是后续工作；在接入前应保留旧分页作为兼容回退，并用同一时间窗口对比服务端聚合与插件旧算法。

## 2026-08-30：日志用量聚合接口生产切换

### 发布对象

- 生产源码 Worktree：`new-api-production`，分支 `personal/main`。
- 功能分支 `feature/log-usage-summary` 已使用 fast-forward 合并，生产 HEAD 为 `064a107821e35ce0778ffa42230f477aab4fe27a`。
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
