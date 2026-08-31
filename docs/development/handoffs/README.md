# New API 交接包目录

这里保存交接包管理模板和脱敏示例。生产切换、数据库备份、镜像摘要等私有材料不进入 Git，实际包放在工程同级的：

```text
/Users/zhangyipeng/MyCodingSpace/ServiceTools/.backups/new-api/handoffs/<handoff-id>/
```

完整规范见 [`../handoff-management.md`](../handoff-management.md)。模板位于 [`templates/`](templates/)：

- [`templates/handoff.json`](templates/handoff.json)：机器可读元数据模板；
- [`templates/EXECUTION_FEEDBACK.md`](templates/EXECUTION_FEEDBACK.md)：执行 Agent 回填模板。

## 快速使用

```bash
cd /Users/zhangyipeng/MyCodingSpace/ServiceTools/new-api-development
python3 scripts/scan-handoffs.py
python3 scripts/scan-handoffs.py --format json
```

扫描器会同时扫描本目录、工程同级私有备份中的标准 `handoffs/` 目录，以及历史 `pre-switch-*`、`pre-sync-*`、`release-*` 目录。它只输出元数据和文件状态，不读取交接正文或敏感内容。

新包必须与 `handoff.json`、`HANDOFF.md`、`EXECUTION_FEEDBACK.md` 同目录保存。发包 Agent 将状态设为 `pending`；执行 Agent 开始前改为 `in_progress`，完成后根据真实结果改为 `completed`、`failed`、`blocked` 或 `rolled_back`。
