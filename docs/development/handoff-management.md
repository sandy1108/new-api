# New API Agent 交接包管理规范

本规范适用于 New API 改造项目中由 Codex 发起、由豆包（或其他明确指定的执行 Agent）执行的交接任务。它解决三个问题：交接包放在哪里、当前执行到哪一步、执行结果在哪里回填。

## 1. 存放位置

新交接包优先使用下面的目录结构：

```text
/Users/zhangyipeng/MyCodingSpace/ServiceTools/.backups/new-api/handoffs/<handoff-id>/
├── handoff.json               # 机器可读元数据，必需
├── HANDOFF.md                 # 发包 Agent 的任务说明，必需
├── EXECUTION_FEEDBACK.md      # 执行 Agent 的回填，必需
└── evidence/                  # 可选：命令输出、截图、校验和等证据
```

这个目录位于 Git 仓库外，适合包含生产路径、镜像摘要、备份位置等私有信息，不得提交到公共仓库。模板和不含私有运行信息的通用示例放在版本库内：

```text
new-api-development/docs/development/handoffs/
├── README.md
└── templates/
    ├── handoff.json
    └── EXECUTION_FEEDBACK.md
```

如果交接包完全脱敏且确实需要随源码版本化，可以放在 `docs/development/handoffs/<handoff-id>/`；仍然必须包含同样的三个文件。新的生产交接包不要再直接放在 `.backups/new-api/` 根目录。历史上的 `pre-switch-*`、`pre-sync-*` 和 `release-*` 目录继续被扫描器兼容，但应在有机会时补齐元数据。

## 2. 元数据和命名

`handoff-id` 使用小写英文、数字和短横线，包含项目、任务和日期，例如：

```text
new-api-pre-switch-20260831-rc29
```

`handoff.json` 至少包含：

| 字段 | 说明 |
| --- | --- |
| `handoff_id` | 全局唯一 ID，新包建议与目录名保持一致 |
| `title` | 人类可读的任务标题 |
| `status` | 当前状态，见下节 |
| `sender_agent` | 发包 Agent，通常为 `codex` |
| `executor_agent` | 执行 Agent，通常为 `doubao` |
| `created_at` | 创建日期或 ISO-8601 时间 |
| `started_at` | 开始执行时间，未开始为 `null` |
| `completed_at` | 完成/失败/回滚时间，未结束为 `null` |
| `result` | 简短结果，未结束为 `null` |
| `handoff_file` | 包内任务说明的相对路径 |
| `feedback_file` | 包内执行反馈的相对路径 |
| `sensitivity` | `private` 或 `sanitized` |

元数据中的文件路径必须是包内相对路径，不能使用 `..` 指向包外。扫描器会把它们解析为绝对路径供本机定位。

## 3. 状态流转

允许的状态及含义：

| 状态 | 含义 |
| --- | --- |
| `pending` | 已生成，等待执行 |
| `in_progress` | 执行 Agent 已开始操作 |
| `completed` | 任务和验收均完成 |
| `blocked` | 因缺少权限、凭据、窗口或外部条件暂停 |
| `failed` | 执行失败，尚未确认回滚 |
| `rolled_back` | 已按方案回滚到安全状态 |
| `cancelled` | 经负责人决定取消 |

常见路径为：

```text
pending -> in_progress -> completed
                    ├──> failed
                    ├──> blocked
                    └──> rolled_back
pending ───────────────> cancelled
```

`completed` 只能在执行反馈写完并有新鲜验证证据后设置；发包 Agent 不得预先把包标记为完成。

## 4. 角色职责

### Codex：发包 Agent

1. 先确认任务边界、风险、备份和回滚方式。
2. 创建唯一目录、复制模板、填写 `handoff.json`，初始状态设为 `pending`。
3. 在 `HANDOFF.md` 写清目标、固定对象、禁止操作、执行命令、验收条件和回滚命令。
4. 创建空白或“待执行”版 `EXECUTION_FEEDBACK.md`，不要代替执行 Agent 填写结果。
5. 最终消息同时提供交接包主文件的可点击绝对路径和纯文本绝对路径。

### 豆包：执行 Agent

1. 执行前先读 `handoff.json` 和 `HANDOFF.md`，确认路径、版本、容器和数据边界。
2. 真正开始变更前，将状态改为 `in_progress`，并记录 `started_at`。
3. 只执行包内授权范围；遇到需要扩大范围的动作先停止并反馈，不自行推断授权。
4. 将命令、时间、镜像/容器摘要、健康检查、业务冒烟和回滚情况写入同目录的 `EXECUTION_FEEDBACK.md`。
5. 结束时更新 `status`、`completed_at` 和 `result`。失败、阻塞或回滚必须如实标记，不能用 `completed` 代替。

## 5. 扫描和回顾

从开发 Worktree 执行：

```bash
cd /Users/zhangyipeng/MyCodingSpace/ServiceTools/new-api-development
python3 scripts/scan-handoffs.py
python3 scripts/scan-handoffs.py --format json
```

扫描器默认同时检查：

1. 当前 Worktree 的 `docs/development/handoffs/`；
2. 工程同级的 `.backups/new-api/handoffs/`；
3. 兼容历史 `.backups/new-api/pre-switch-*`、`pre-sync-*` 和 `release-*` 目录。

如果要扫描正式控制目录而不是开发 Worktree：

```bash
python3 /Users/zhangyipeng/MyCodingSpace/ServiceTools/new-api-development/scripts/scan-handoffs.py \
  --project-root /Users/zhangyipeng/MyCodingSpace/ServiceTools/new-api
```

表格输出适合人工巡检，JSON 输出适合后续脚本或 Dashboard 消费。扫描器只读取元数据和文件是否存在，不打印交接正文、Token、密码、私钥或数据库内容。状态只以 `handoff.json` 为准，不根据容器、进程或其他外部状态自动推断完成，避免把“服务已经切换”误报成“验收已完成”。`invalid` 表示元数据损坏或字段不合法；旧目录缺少元数据时会登记为 `pending` 并给出警告，不能误认为已完成。

## 6. 反馈模板最低要求

执行反馈至少记录：

- 交接包 ID、执行 Agent、开始/结束时间；
- 实际执行的命令和关键输出摘要；
- 变更对象及未触碰的对象；
- 健康检查、接口/业务冒烟和数据边界验证；
- 是否失败、回滚或仍被阻塞；
- 可复核的证据文件路径和 SHA-256（如适用）。

反馈中不得写入 Token、密码、私钥、Cookie 或完整请求正文。真实凭据只通过本机安全环境提供。

## 7. 交接回复路径规范

涉及交接包的最终回复必须给出两种路径，且指向同一个实际文件：

```markdown
[HANDOFF.md](/绝对路径/到/交接包/HANDOFF.md)
```

同时紧跟一行可复制的纯文本绝对路径：

```text
/绝对路径/到/交接包/HANDOFF.md
```

不能只给相对路径，也不能只给 Markdown 链接。若路径含空格，Markdown 目标使用尖括号包裹。

## 8. 发包和收包检查清单

发包前确认：

- [ ] `handoff.json` 字段完整，状态为 `pending`；
- [ ] `HANDOFF.md` 写清范围、命令、验收和回滚；
- [ ] `EXECUTION_FEEDBACK.md` 已创建但未伪造执行结果；
- [ ] 私有信息只在 Git 外的备份目录；
- [ ] 已运行扫描器并能看到该包；
- [ ] 回复同时包含可点击和纯文本绝对路径。

收包后确认：

- [ ] 执行前已核对包 ID、版本、目录、依赖和禁止操作；
- [ ] 状态和时间字段与实际阶段一致；
- [ ] 反馈与证据文件同目录；
- [ ] 验证结果可复现，失败/回滚没有被写成成功；
- [ ] 再次运行扫描器，状态统计与反馈一致。
