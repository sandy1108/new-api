# New API 个人分支与发布管理规范

## 目的

本规范适用于本项目的个人维护分支、上游同步、测试环境和生产发布。目标是同时满足：

- 持续吸收官方上游更新；
- 保留个人生产 Compose 和功能改造；
- 在不影响正式服务的前提下验证新版本；
- 为每次升级保留可恢复的代码、镜像和数据边界。

## 远端职责

```text
origin  -> https://github.com/QuantumNous/new-api.git
          官方上游，只读取和同步

myfork  -> 个人 Fork
          个人分支和已确认的改造结果
```

不得把个人生产凭据提交到任一远端。生产密码、数据库连接串和其他密钥放在被 Git 忽略的环境文件或受保护的运行时配置中。

## 分支结构

```text
origin/main
  官方上游基线

myfork/main
  官方代码镜像，不放个人部署差异

personal/main
  长期个人维护分支，承载脱敏后的部署结构和已完成改造

feature/<topic>
  单项功能或修复，例如 feature/log-usage-summary

backup/pre-sync-<YYYYMMDD>
  上游同步或大规模改造前的代码快照
```

生产发布另外使用不可变 tag，例如 `prod-2026-08-26-rc21`。运行环境应记录实际镜像 digest，不使用浮动的 `latest` 作为回滚依据。

## 首次整理顺序

1. 检查工作区状态、当前 commit、两个远端和生产 Compose 差异。
2. 在 Git 外部备份 PostgreSQL、生产 `data`/`logs` 路径、Compose 和当前镜像 digest。
3. 从当前 `main` 创建 `personal/main`，保留用户的生产 Compose 修改。
4. 将 Compose 中的真实凭据移出版本库，改用环境变量或被忽略的 env 文件。
5. 只提交脱敏后的 Compose 结构和必要说明。
6. 创建 `backup/pre-sync-<日期>` 分支及对应 tag。
7. fetch `origin` 和 `myfork`，确认上游提交范围后，再把 `personal/main` 同步到最新官方代码。
8. 基于同步后的 `personal/main` 创建功能分支。

首次整理阶段可以 rebase，因为个人分支尚未作为共享历史使用。完成后再推送到个人 Fork。

## 日常上游同步

- `origin/main` 是官方更新入口，先 fetch，再审查提交和迁移内容。
- 尚未发布或未被其他设备使用的个人分支可以 rebase 到 `origin/main`。
- 已推送且被多个 Agent、设备或部署流程依赖的 `personal/main` 使用 merge 同步，不改写共享历史。
- 功能分支合并前可以 rebase 到最新 `personal/main`，解决冲突后运行完整验证。
- 不使用普通 `git push --force`；确需修正个人未共享历史时，也只能使用 `--force-with-lease` 并先确认远端状态。

推荐提交分层：

```text
chore: 调整个人生产环境部署结构
chore: 增加隔离的测试环境
feat: 增加日志用量聚合接口
feat: 客户端接入日志用量聚合接口
```

部署配置、功能代码和客户端改造分开提交，便于审查、回滚和后续同步上游。

## 测试环境规则

测试环境必须与生产完全分离：

- 使用独立 Compose 项目、容器名、网络和端口；
- 测试 New API 使用独立 PostgreSQL 和 Redis；
- 测试使用独立 `data`、`logs` 和环境变量文件；
- 不挂载生产 PostgreSQL、Redis、`data` 或 `logs`；
- 使用完整生产 Dockerfile 构建，避免测试后端与正式镜像内容不一致；
- 测试端口默认只绑定 `127.0.0.1`，避免绕过现有 Nginx 暴露公网。

功能验证通过后，应保留测试镜像的不可变 tag 或 digest。正式环境提升同一个镜像，不在发布时重新构建。

## 正式发布与回滚

发布前必须完成：

1. PostgreSQL 可恢复备份，并记录备份时间和校验结果；
2. 生产 Compose、环境配置和当前镜像 digest 记录；
3. 新旧数据库迁移审查；
4. 测试环境健康检查、后台登录、Chat Completions、Responses/Codex 和日志统计验证；
5. 生产切换步骤、预计影响和回滚命令明确。

发布时只重建 New API 应用容器，不删除或重建生产 PostgreSQL、Redis 数据卷。禁止对生产栈执行 `docker compose down -v`，禁止删除生产 PostgreSQL 数据目录。

上线后验证健康检查、登录、现有模型路由、Token 统计和关键 Agent 请求。失败时切回上一个已验证镜像 tag，并保留失败日志；数据库迁移若不可逆，必须在升级前单独评估恢复方案。

## 镜像标签与项目发布日志

正式候选镜像使用可读且不可复用的标签格式：

```text
new-api:<upstream-version>-<YYYYMMDD>-<NN>-g<short-commit>
```

示例：

```text
new-api:v1.0.0-rc.27-20260829-01-g8454082f
```

规则如下：

- `<upstream-version>` 保留官方版本或最近 release candidate，例如 `v1.0.0-rc.27`；没有合适的上游版本时使用 `unreleased`。
- `<YYYYMMDD>` 使用 Asia/Shanghai 日期，表示构建日期。
- `<NN>` 是本机当天 New API 候选构建序号，从 `01` 开始；序号一旦分配即使构建失败也不复用，允许出现空号。
- `g<short-commit>` 是构建源码的 Git 提交短哈希，用于精确回溯代码。
- 开发镜像单独使用 `new-api:dev-<YYYYMMDD>-<NN>-g<short-commit>`，不与正式候选序号混用。
- 回滚镜像使用独立的 `backup-<来源>-<日期>` 标签，不能覆盖候选标签，也不能依赖 `latest`。

每次构建或发布都必须在 `docs/development/release-log.md` 追加记录，至少包含：源码分支和 commit、镜像 tag/ID/digest、构建时间和架构、数据库备份位置与校验、Compose 变更范围、健康检查、关键接口冒烟测试、回滚标签和最终结果。正式 Compose 只引用已经记录并验证过的候选 tag。

## 验收记录

每次同步或发布至少记录：

- 基线 commit、目标 commit 和同步方式；
- 个人改造 commit 列表；
- 数据库备份位置、时间和校验；
- 测试镜像 tag/digest；
- 正式镜像 tag/digest；
- 健康检查和关键接口结果；
- 是否发生回滚，以及回滚后的验证结果。
