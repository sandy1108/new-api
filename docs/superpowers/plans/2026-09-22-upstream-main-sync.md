# 2026-09-22 Official Mainline Synchronization Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to execute this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在隔离开发 worktree 将最新 `origin/main` 合并到个人维护基线，保留 New API 私有日志用量聚合 API、Web 统计页和项目文档，并完成源码、迁移和开发环境回归。

**Architecture:** 先固定 `backup/pre-sync-20260922` 回退点，在新建的 `upgrade/upstream-main-20260922` 上以普通 merge 吸收官方主线。官方认证、迁移、计费、Responses/Relay 和前端能力优先保留；私有统计改造按路由、模型和 UI 语义逐项核对。生产控制 worktree、生产源码 worktree、Compose、容器和数据卷只读不动。

**Tech Stack:** Go/Gin/GORM、PostgreSQL、Redis、React/TanStack Router、Rsbuild、Vitest、Docker Compose。

**Spec:** `docs/development/git-branch-and-release-workflow.md`、`docs/development/upstream-sync-checklist.md`、`docs/superpowers/specs/2026-08-29-log-usage-summary-design.md`、`docs/superpowers/specs/2026-08-31-web-usage-summary-design.md`。

## Global Constraints

- 只在 `/Users/zhangyipeng/MyCodingSpace/ServiceTools/new-api-development` 操作源码、分支和开发测试环境。
- 不修改 `/Users/zhangyipeng/MyCodingSpace/ServiceTools/new-api/docker-compose.yml`，不重启正式容器，不连接或迁移生产 PostgreSQL/Redis 数据卷。
- 不删除当前 worktree 中已有未跟踪的 `.superpowers/`、既有计划、`node_modules`、缓存、`__pycache__` 或 `web/pnpm-lock.yaml`。
- 官方安全、认证、迁移、计费和协议兼容逻辑不得因私有改造被覆盖；语义不清的冲突停止并报告。
- 本轮不自动合并 `personal/main`，不推送 `myfork`，不构建正式候选镜像；完成开发验证后再向用户提供集成选项。

---

### Task 1: 基线、远端和变更范围

**Files:**
- Create: `.backups/new-api/upgrade-20260922/` evidence files outside Git
- Inspect: `git worktree`, `git status`, `git log`, `git diff`, remote refs

**Interfaces:**
- Consumes: `upgrade/upstream-main-20260922` at `41808c765849c6d71311185e3d3d81c30ff37e91` and `backup/pre-sync-20260922`.
- Produces: fetched official target SHA, commit range report, and an unchanged production boundary.

- [x] **Step 1: Fetch official and personal remotes without rewriting branches.**

```bash
git fetch origin --tags
git fetch myfork
```

- [x] **Step 2: Record target refs and commit range.**

```bash
git rev-parse HEAD
git rev-parse origin/main
git describe --tags --always origin/main
git rev-list --count HEAD..origin/main
git log --oneline --decorate HEAD..origin/main
git diff --stat HEAD...origin/main
git diff --name-status HEAD...origin/main
```

- [x] **Step 3: Classify upstream changes.** Review authentication/session, migration/index/constraint, billing/pre-consume, Responses/Relay, channel/model, web route/menu, and dependency changes. Save the classification under `.backups/new-api/upgrade-20260922/`.

### Task 2: Merge official mainline and preserve private changes

**Files:**
- Modify only files reported by Git as conflicted, expected to include backend router/model and Web route/i18n files if upstream overlaps.
- Preserve: private usage-summary controllers/models/routes, Web usage-summary route/menu/three-level hierarchy, and project docs.

**Interfaces:**
- Consumes: fetched `origin/main` and current private baseline.
- Produces: a conflict-free merge tree with both official and private contracts present.

- [x] **Step 1: Start a no-commit merge and save the conflict list.**

```bash
git merge --no-ff --no-commit origin/main
git status --short
git diff --name-only --diff-filter=U
```

- [x] **Step 2: Resolve backend conflicts by semantic review.** Keep official middleware, authorization, audit, migration, billing, and protocol handling; retain private usage-summary endpoints only when their auth and response contract remain valid. Run `git diff --check` after each resolution.

- [x] **Step 3: Resolve Web conflicts by preserving both navigation surfaces.** Keep official routes, labels and settings; retain the usage-summary entry and token → channel → model hierarchy. Regenerate route artifacts with the project command if the merge changes route generation inputs.

- [x] **Step 4: Verify private surface and stage only intended files.**

```bash
rg -n "usage-summary|UsageSummary|按 API 令牌|按渠道|按模型" controller model router web/src docs/development
git diff --check
git status --short
git diff --name-only --diff-filter=U
```

### Task 3: Source and frontend regression

**Files:**
- Test: Go packages and private usage-summary tests
- Test: `web` Vitest, typecheck, build and lint scripts

**Interfaces:**
- Consumes: conflict-free merge tree.
- Produces: fresh exit-code evidence for backend and frontend source compatibility.

- [x] **Step 1: Run Go tests and build with bounded parallelism.**

```bash
GOMAXPROCS=1 GOFLAGS=-p=1 go test ./...
GOMAXPROCS=1 GOFLAGS=-p=1 go build ./...
```

- [x] **Step 2: Run the existing Web scripts.**

```bash
cd web
bun test
bun run typecheck
bun run build
bun run lint
cd ..
```

- [x] **Step 3: Review key contracts statically.** Confirm authentication/encryption defaults, Responses routes, GPT-6 model capability handling, usage-summary API paths, and Web hierarchy still compile and remain discoverable.

### Task 4: Database migration and isolated development validation

**Files:**
- Inspect: official migration files and model tests
- Create: `.backups/new-api/upgrade-20260922/` migration and test reports
- Modify only if required: development-only Compose or test helper files

**Interfaces:**
- Consumes: a separately backed-up test database copy and the merged source tree.
- Produces: first-start migration, second-start idempotence, health, login, model, Responses and usage-summary evidence. Production remains untouched.

- [x] **Step 1: Enumerate migration/constraint changes and mark irreversible operations.**
- [x] **Step 2: Run the existing isolated development Compose using independent project name, ports, PostgreSQL, Redis, `data` and `logs`; never pass production paths.**
- [x] **Step 3: Verify `/api/status`, login/encryption login, `/v1/models`, `/v1/chat/completions`, `/v1/responses`, private usage-summary endpoints and the Web statistics page.**
- [x] **Step 4: Restart the application and confirm migration idempotence, key row counts and no fatal/panic/migration errors.**

### Task 5: Review, document and stop at the integration gate

**Files:**
- Modify: `docs/development/release-log.md`
- Preserve: the new checklist and plan

**Interfaces:**
- Consumes: merge diff, test output, migration report and development runtime evidence.
- Produces: reviewable local sync commit and an explicit decision point before `personal/main`, push or production.

- [x] **Step 1: Run final diff and boundary review.**

```bash
git diff --check
git status --short --untracked-files=all
git diff --stat backup/pre-sync-20260922...HEAD
git -C ../new-api-production status --short --untracked-files=all
git -C ../new-api status --short --untracked-files=all
```

- [x] **Step 2: Append the baseline, target, conflicts, verification and open items to `release-log.md`.**
- [x] **Step 3: Stage only synchronization and documentation files and inspect the cached diff.**
- [x] **Step 4: Create the Chinese type-prefixed local merge commit only after fresh verification.**
- [x] **Step 5: Stop and present integration options; do not merge to `personal/main` or push without a separate user decision.**
