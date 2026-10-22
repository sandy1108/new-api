# New API 个人分支与发布管理规范

交接任务的目录、状态、执行反馈和 Agent 职责另见 [`handoff-management.md`](handoff-management.md)；发包前后应运行其中的交接包扫描器。

每次同步官方主线前，先按 [`upstream-sync-checklist.md`](upstream-sync-checklist.md) 完成基线、远端范围、私有改造保留、迁移演练和开发回归检查；该检查单不替代生产发布审批。

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

### HTTPS反代与会话验收

本地HTTP测试通过不代表生产HTTPS反代写请求可用。若TLS由反代终止，按`docs/authentication.md`同时配置`SESSION_COOKIE_SECURE=true`及精确的`SESSION_COOKIE_TRUSTED_URL`，不要直接信任客户端转发头或放宽为任意Origin。真实域名仅保存在私人部署配置中，不提交到仓库。

发布验收须覆盖生产HTTPS登录、读取、保存设置/新增业务对象、刷新后数据持久化，以及会话到期刷新；未覆盖的项目单列待观察。未认证401、空对象400和容器healthy均不能替代实际写操作成功；凭据、Cookie和完整HAR不得写入反馈。配置修改先备份、只重建应用，数据库/Redis/永久测试保持不动。

## 镜像标签与项目发布日志

### PONG运行版本号

个人分支直接维护根目录`VERSION`，不再增加第二份版本号文件。每次正式候选构建前填写：

```text
<upstream-version>+pong.<YYYYMMDD>.<NN>
```

例如`v1.0.0-rc.45+pong.20261022.02`：官方基线rc.45、北京时间2026-10-22、当天候选序号02。`NN`沿用正式候选序号，不与临时开发镜像序号混用；失败已分配的序号也不复用。文件记录拟构建版本，不代表对应镜像已上线；构建时复制它的不可变快照，并在发布日志记录源码、实际镜像ID及部署状态。不能仅改源文件或重新打标签就声称运行程序版本已更新。

现有Dockerfile读取`VERSION`注入前端构建变量及Go的`common.Version`，页面和`/api/status`复用该值，无需新增数据库字段、API或替换官方版本比较代码。现有比较器忽略`+`后的构建元数据，只按官方基线判断上游更新；它不用于比较两个PONG构建的先后。正式/测试应提升同一个已验证镜像，不在生产发布时改日期后重编。

Docker镜像tag不允许`+`，因此与运行版本对应的正式候选tag使用`<upstream-version>-pong.<YYYYMMDD>.<NN>-g<short-commit>`；开发tag仍按下文`dev-...`格式，可加`-dirty`并保存源码manifest说明未提交改动。旧镜像tag不追溯重命名。

官方跟踪了空白`VERSION`，其发布流水线会在构建目录写版本。合并官方且对方未修改此文件时，通常保留个人内容；不保证未来零冲突。每次同步检查`VERSION`历史与官方流水线，保留PONG格式、更新已实际吸收的官方基线，不能直接选择空白版本。若直接运行官方CI，它可能覆盖`VERSION`，个人发布必须检查最终注入值与源文件一致。

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
