# 日志用量聚合接口设计

## 背景

Chrome 插件当前通过 `GET /api/log/` 按每页 100 条读取消费日志，再在浏览器端按 Token、渠道、模型聚合。日志量达到数万条时，需要发出数百次请求，并且每次分页查询都会先执行总数统计，容易与其他请求叠加触发限流。

## 目标

- 提供一次请求即可返回日志用量聚合结果的只读接口。
- 保留现有 `/api/log/` 分页接口和全局 `page_size <= 100` 约束不变。
- 复用现有日志筛选语义，区分管理员和普通用户的数据范围。
- 不新增表、不修改现有数据、不改变生产 Compose。
- 让 Chrome 插件和未来的 New API 自定义统计页共享同一份服务端聚合数据。

## 非目标

- 本阶段不把 Chrome 插件完整迁移到 Web 控制台。
- 本阶段不调整全局分页上限，也不新增原始日志批量导出接口。
- 本阶段不改造 `quota_data` 表或补充数据库迁移。
- 本阶段不切换生产镜像或重启生产容器。

## 接口设计

新增两个接口：

```text
GET /api/log/usage-summary       管理员权限
GET /api/log/self/usage-summary  普通用户权限
```

接口固定统计 `LogTypeConsume (type=2)`，不把 `type` 参数作为通用日志类型开关。支持以下查询参数：

- `start_timestamp`、`end_timestamp`：可选，闭区间 Unix 秒时间戳。
- `username`：仅管理员接口使用，沿用现有显式文本筛选语义。
- `token_name`、`model_name`、`channel`、`group`：沿用现有日志筛选语义。

普通用户接口将认证上下文中的用户 ID作为强制条件，忽略客户端提供的 `username`，避免通过参数越权读取其他用户日志。

响应使用现有 `success/message/data` 包装，`data` 结构如下：

```json
{
  "total_requests": 1234,
  "total_input_tokens": 100000,
  "total_output_tokens": 80000,
  "total_tokens": 180000,
  "total_quota": 500000,
  "items": [
    {
      "user_id": 1,
      "username": "alice",
      "token_id": 11,
      "token_name": "primary",
      "channel_id": 7,
      "channel_name": "official",
      "model_name": "gpt-5.6-luna",
      "requests": 100,
      "input_tokens": 10000,
      "output_tokens": 8000,
      "total_tokens": 18000,
      "quota": 50000
    }
  ]
}
```

`items` 是扁平聚合行，分组键为 `user_id、username、token_id、token_name、channel_id、model_name`。返回顺序按 `total_tokens` 降序，并使用稳定的维度键作为并列排序依据。渠道名称由主数据库按渠道 ID 补充；日志数据库为 ClickHouse 时不做跨库 JOIN，沿用现有的二次查询模式。

总计由聚合行在服务端累加得到，输入/输出 Token 使用原始 `logs.prompt_tokens` 和 `logs.completion_tokens`，因此比现有 `quota_data.token_used` 更适合复现 Chrome 插件的统计。

## 数据流

```text
HTTP 请求
  -> Gin 路由 + AdminAuth/UserAuth
  -> 控制器解析筛选条件
  -> LOG_DB 对 logs 执行 type=2 + 条件 + GROUP BY
  -> 主 DB 查询渠道名称
  -> 服务端计算总计和稳定排序
  -> success/message/data 响应
```

查询不使用分页和 `OFFSET`，也不调用现有 `GetPageQuery`。时间条件由调用方提供时优先使用；未提供时保持与现有统计接口一致的全量语义，但客户端应默认传递有界时间范围。

## 错误和边界

- 时间戳存在但无法解析，或结束时间早于开始时间时，返回业务失败响应。
- 空结果返回空数组及全 0 总计，不返回 `null`。
- 空 Token、空渠道、空模型保留原始空值，由客户端负责本地化显示。
- 已删除渠道无法解析名称时只保留 `channel_id`，不伪造历史名称。
- 查询失败沿用 `common.ApiError`，不向客户端泄露数据库连接凭据或 SQL 详情。

## 测试策略

- 模型测试：消费日志分组、输入/输出/总 Token、quota、时间范围、用户和维度筛选、非消费日志排除、空结果。
- 控制器测试：管理员筛选参数、普通用户强制用户范围、业务错误响应和 JSON 契约。
- 使用现有 SQLite 测试数据库验证逻辑；在独立 Docker 测试栈中做 PostgreSQL 冒烟验证。
- 对比同一 fixture 下服务端聚合结果与 Chrome 插件现有客户端聚合结果，确认总请求数、Token 数和 quota 一致。

## 后续扩展

接口稳定后，再实现 Web 控制台自定义统计页。若未来需要超大维度基数或趋势缓存，再单独评估 `quota_data` 扩展、缓存或异步查询，不在本阶段引入。

## 实施验证补充（2026-08-30）

- 模型、控制器和路由实现已落在开发 Worktree；使用 PostgreSQL 隔离测试栈完成真实 HTTP 验收，未修改生产 Compose 或数据。
- 管理员和普通用户的合成数据结果与本设计中的聚合字段、权限边界和稳定排序一致；Chrome 插件实际接入仍留待后续阶段。
- Docker 标准并行构建曾因构建内存不足在 `ch-go/proto` 编译时被 OOM killer 终止；通过一次性 `GOMAXPROCS=1`、`GOFLAGS=-p=1` 串行编译生成测试镜像，未改变仓库 Dockerfile。
- 最终隔离测试容器运行 `new-api:dev-20260830-02-g8454082-dirty`（镜像摘要 `sha256:b975be1195d842984c4db46ff3c40583bf594448cc7ae683ae4a7afd29ed8b48`）；早期 `-01` 测试镜像仍保留作历史追溯，生产镜像和容器未触碰。
