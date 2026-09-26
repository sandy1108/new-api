# 客户端访问凭据与 Codex 额度接口发布准备计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. 当前用户已确认 FULL 工作流；生产容器切换由独立执行 Agent 执行，不在本计划中直接重启。

**Goal:** 将已在隔离测试环境验证的客户端访问功能整理为可复现的正式候选，完成生产备份与可执行交接包。

**Architecture:** 开发 worktree 的 `feature/client-access-codex-quota` 只提交本功能源码、测试和文档；生产源码 worktree 保留 `personal/main` 与既有未提交日志。正式候选从已提交的精确 Git SHA 构建并在永久测试栈验证，生产切换只替换应用镜像行，由独立 Agent 按交接包执行。

**Tech Stack:** Git worktree、Go、React/Vitest、Docker、PostgreSQL 15、Redis、New API 本地 Compose。

**Spec:** `docs/superpowers/specs/2026-09-23-client-access-credentials-and-codex-quota-design.md`；发布边界以 `docs/development/git-branch-and-release-workflow.md`、`docs/development/handoff-management.md` 为准。

## Global Constraints

- 用户明确接受跳过本次生产数据副本升级演练；仍必须审查迁移、完成生产 PostgreSQL 可恢复备份、记录镜像/配置快照与回滚路径。不能把应用镜像回滚等同于数据库恢复。
- 不修改 `new-api/docker-compose.yml`、`.volumes/new-api/` 或生产容器；不执行 `down -v`、prune 或生产数据恢复。
- 生产源码 worktree 的 `docs/development/release-log.md` 已有既存未提交内容，必须保留并单独归属，不能覆盖、丢弃或混入功能提交。
- 用户已把 Docker 虚拟机内存上限调为约 4 GiB；各一次性构建/测试容器仍需独立硬内存和 CPU 限额。构建/测试监控 Docker 内存，不能再使守护进程 OOM。
- `personal/main` 是共享维护分支，只可 merge，不 rebase 或 force push；提交、合并、推送、生产切换分阶段核验。
- 凭据、Cookie、OAuth Token、数据库密码、完整响应不得进入 Git、测试输出、交接正文或知识库。

## Task 1：冻结功能范围并审查差异

**Files:** 审查 `model/client_access_*`、`service/clientaccess/`、`middleware/client_access_*`、`controller/client_*`、`router/client-access-router.go`、Web Feature、Postman、API 文档及现有文件的最小改动；本任务不修改源码。

**Interfaces:** 核对 `GET /api/client-access/v1/codex/usage`、Root 管理 API、`codex.quota.read`、两张私有表和旧渠道刷新入口。

- [ ] 运行 `git status --short --branch`、`git diff --check` 和精确未跟踪文件清单；将 `node_modules/`、`scripts/__pycache__/`、`.superpowers/` 及历史计划与本功能文件分开。
- [ ] 对照设计文档逐项审查鉴权隔离、Scope/Grant、OAuth 刷新竞争、Redis 失败关闭、脱敏、迁移与旧路由兼容；记录发现的阻断项。
- [ ] 对非平凡差异进行独立代码评审；修复 Critical/Important 问题后复测，不能把旧镜像的测试结果当作新源码证据。

## Task 2：完整且受限的源码验证

**Files:** 不新增生产代码；若发现缺陷，只按测试先行原则修改对应功能文件和测试。

**Interfaces:** 六个相关 Go 包、根模块/`relaykit`、前端功能测试、类型检查与正式 Web 构建。

- [ ] 使用带硬内存上限的一次性 Go 容器执行相关六包 `go test -count=1`；容器失败不影响 Docker daemon。
- [ ] 在资源允许时执行根模块与 `relaykit` 全量测试、前端测试/类型检查/构建；如已知上游基线失败，保存精确错误和缩小后的通过范围，不虚报全绿。
- [ ] 运行 `gofmt` 检查、`git diff --check`、敏感串扫描；复核永久测试栈和生产应用健康状态。

## Task 3：可复现提交与正式候选

**Files:** 仅暂存本功能源码、测试、设计/API 文档、Postman、发布记录及本计划；不暂存依赖目录、缓存或无关历史计划。

**Interfaces:** Git SHA → `new-api:v1.0.0-rc.40-<YYYYMMDD>-<NN>-g<short-sha>` → 镜像 ID/digest。

- [ ] 用显式路径 `git add`，执行 `git diff --cached --name-status` 和 `git diff --cached --check`，逐项核对无私人 Compose、凭据和无关文件。
- [ ] 使用中文、带类型前缀的 commit message 提交；保存完整 SHA，确认工作区剩余文件均可解释。
- [ ] 分配未用的当日正式候选序号；从已提交 SHA 构建完整 Dockerfile 镜像，记录构建时间、架构、镜像 ID/digest。构建时监控资源；失败序号不复用。
- [ ] 在永久测试栈以同一镜像进行最小回归；切换测试应用前复核独立数据备份与 Compose，测试后镜像摘要必须一致。不得在生产发布时重新构建。

## Task 4：生产发布前备份与交接

**Files:** 在 Git 外 `.backups/new-api/handoffs/<handoff-id>/` 创建 `handoff.json`、`HANDOFF.md`、`EXECUTION_FEEDBACK.md` 和 `evidence/`；在项目发布日志及 Obsidian 项目记录追加脱敏摘要。

**Interfaces:** 精确生产镜像 tag/ID、旧镜像 tag/ID、生产 Compose 快照、PostgreSQL 自定义格式 dump、应用 data/logs 归档、SHA-256、只重建应用容器的命令。

- [ ] 只读确认生产 Compose 实际引用镜像、容器 ID/健康、挂载路径、旧镜像保留及 `PASSWORD_LOGIN_ENCRYPTION_ENABLED=true`；检查备份目录空间和权限。
- [ ] 在不停止生产的前提下执行 `pg_dump -Fc`，并备份 Compose、环境配置的安全快照及应用 data/logs；计算 SHA-256、使用 `pg_restore --list` 验证 dump 可读，不尝试恢复到生产。
- [ ] 编写明确的切换前检查、`--no-build --pull never --no-deps` 仅重建应用步骤、健康/登录/额度/旧 API 验收、失败时旧镜像回滚、数据库恢复只在单独批准后执行的边界。
- [ ] 按 `docs/development/handoff-management.md` 模板创建 pending 包和空白执行反馈；运行 `python3 scripts/scan-handoffs.py`，确认路径、状态和文件完整。
- [ ] 将候选、备份和交接事实写入项目日志与知识库并读回；最终回复提供同一 `HANDOFF.md` 的可点击绝对路径和纯文本绝对路径。此时仍不得自行切换生产容器。

## 执行闸门

1. Task 1/2 的 Critical 或 Important 问题未解决，不得提交或构建正式候选。
2. 工作区源码尚未提交、镜像仍带 `dirty`、候选摘要与测试镜像不一致，不得发生产交接包。
3. 生产备份不可读或校验不匹配、旧镜像缺失、Compose 差异超出镜像行，不得切换。
4. `personal/main` 合并、推送和生产部署分别依据当时的明确授权办理，不因创建交接包自动执行。

## 2026-09-26 验证检查点：本机依赖树阻断

- FULL 回归中，受限 Go 容器执行 `go test -count=1 ./...` 时，除根包外各包均通过；根包因 `web/dist/index.html` 缺失而在 `go:embed` 阶段失败，尚未运行根包测试。
- 宿主机 `pnpm build` 无法生成该文件：现有 `web/node_modules` 中 `@rspack/core@2.1.4` 与 `@rspack/binding@2.2.1` 不匹配；仓库正式 `web/bun.lock` 将二者都固定为 2.1.4，而 Dockerfile 使用 `bun install --frozen-lockfile`。本机 `web/pnpm-lock.yaml` 是未跟踪文件，不能将该依赖状态视为正式构建输入。
- 全量宿主机 Vitest 出现大量 React Invalid hook call / 双副本迹象，已中止，不能声称全量通过。新增功能的两个定向前端文件此前 4/4 通过；这一事实不能替代全量回归。
- 在决定是使用正式 Dockerfile 的隔离 Bun 构建，还是修复宿主机依赖树之前，暂停提交和正式候选构建；不得修改生产或把本检查点误报为发布就绪。

## 2026-09-27 验证检查点：隔离工具链

- 用户提高 Docker 上限后，使用 Bun 1.4.0、`bun.lock` 和独立 2.5 GiB 容器完成正式前端构建；类型检查单独在 3 GiB 容器中通过。宿主机 `node_modules` 和未跟踪的 `pnpm-lock.yaml` 未参与验证。
- 新增页面直接导入 `SectionPageLayout` 的组件文件，避免布局总入口带入无关系统设置模块；新增页面定向 Vitest 2/2 通过。前端全量 Vitest 仍不通过：从未修改的 HEAD 导出基线后，原有渠道测试也在 `z.object` 处以 0 tests 失败。不能称全量前端测试通过，也不能把所有失败归因于本功能。
- 用真实 Bun 构建产物补齐 `go:embed`，只读挂载源码、限制 Go 容器 1.4 GiB 和 1 CPU，根模块 `go test -count=1 ./...` 通过；独立 `relaykit` 模块 `go test -count=1 ./...` 通过。临时前端产物卷已删除，生产应用维持 healthy。
- 以上为当前未提交源码的验证；提交 SHA、正式候选镜像和同镜像回归仍待执行。生产数据副本演练按用户决定跳过，不能因此略过生产备份和回滚检查。
