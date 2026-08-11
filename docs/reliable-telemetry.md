# 可靠遥测运维指南

Santaizi 采用单 Primary 控制面与多 Collector 遥测面。Agent 先写本地 Segment WAL，再分别向 Primary 和已分配的 Collector 发送；Collector 在本地事务提交后 ACK Agent，并通过持久 Outbox 将事实复制到 Primary。所有可靠链路都是至少一次投递，接收端按 Event、Observation、Replication Batch 去重，并且只在事务提交后返回累计连续 ACK。

## 数据与进程角色

- Primary：加载 OAuth、Web、API、服务监控、告警、内部调度、Availability、Rollup 与 Retention。
- Collector：同一 Dashboard 二进制以 `mode: collector` 启动，只提供鉴权的遥测接收、复制、`GetStatus` 与标准 gRPC Health；不提供 HTTP Health 或 Web UI。
- Agent：默认配置 `/etc/santaizi/agent.yaml`，可靠数据目录 `/var/lib/santaizi-agent/`；WAL 默认 8 MiB Segment、256 MiB 总量、1 MiB 紧急预留。
- Dashboard：默认配置 `/etc/santaizi/dashboard.yaml`，Primary 数据库 `/var/lib/santaizi-dashboard/sqlite.db`，Collector 数据库默认 `/var/lib/santaizi-dashboard/collector.db`。

所有模式共用一个 gRPC 端口。`SantaiziService`、V2 分层服务和 `grpc.health.v1.Health` 同时注册。控制协议只能表达 HTTP、ICMP、TCP 探测以及独立鉴权的 NAT 流，不能表达命令、终端、文件或更新操作。

## 部署 Collector

1. 在 Primary 管理 API 创建 Collector：

   ```http
   POST /api/v2/collectors
   Content-Type: application/json

   {"name":"HK","address":"hk.example.com:5556","tls":true,"scopes":[{"type":"all","value":""}]}
   ```

2. 在管理后台查看并复制注册 Token，将它写入 Collector 的 `collector.registration_token`，并持久化 Collector 数据目录。
3. 启动 Collector。成功注册后会缓存 Primary 公钥、授权、Assignment、撤销状态和配置版本。
4. 用 `PUT /api/v2/collectors/:id/scope` 更新 `all`、`server`、`group` 或 `tag` Scope。Scope/标签变化会产生新配置版本和带有效期的 Assignment，不删除历史 Evidence。
5. Rotate、Revoke 或 Delete 后旧 Token 立即失效。Delete 仅结束未来 Assignment；历史事实仍保留。删除后重新添加会获得新的稳定 ID/Generation。

生产公网 gRPC 推荐启用 TLS 并使用受信任证书。`primary_insecure_tls` 和 endpoint 的 `insecure_tls` 只适用于受控测试。

## 可用性语义

每个 30 秒 Bucket 根据当时有效 Assignment 计算 expected，以 Observer Health 计算 healthy，以 Observation 计算 seen：

- 任一健康 Observer 看到节点：Host `ONLINE`。
- 无人看到且健康 Observer 数达到 `min_observers`：Host `OFFLINE`、Connectivity `UNAVAILABLE`。
- Observer 证据不足：Host 与 Connectivity 均为 `UNKNOWN`。
- 全部健康 Observer 看到为 `FULL`；部分看到为 `PARTIAL`。V2 `available` 对 FULL/PARTIAL 为 `true`，UNAVAILABLE 为 `false`，UNKNOWN 为 `null`。

晚到 Replay 会重算历史 Bucket，并以不可变 Incident Revision 修正分类；Correction 通知默认关闭。V1 的布尔 `online` 因类型限制会把 UNKNOWN 映射为 `false`。

## 容量、丢失与诊断

- Agent WAL 只删除所有当前可靠 Sink 都已 ACK 的完整 Segment。Collector 被正式删除后，相应义务解除。
- WAL 压力优先降采样并生成 1 分钟 Rollup；任何被替代、损坏或 Hard Limit 丢失都用显式 Gap/Data Loss 表示，不制造隐式 Sequence Hole。
- Collector Spool 默认 5 GiB/30 天。正常只清理 Primary 已 ACK 数据；Hard Limit 必须丢弃未同步数据时，会生成可复制的 Gap 与 Data Loss 事实。
- 后台 `/telemetry` 和 `/api/v2/telemetry/servers/:id/status` 展示 WAL、Sink Cursor、Spool、Replication Backlog、Gap 与 Data Loss。关键日志不包含 Secret。
- 运行基准：`go test -run '^$' -bench BenchmarkSyntheticTelemetry1000x10 -benchtime=1x ./service/telemetry`。

## 数据库与升级

本架构只支持版本化 Santaizi 数据库。若目标 SQLite 非空但没有 `schema_migrations`（Collector 为 `collector_schema_migrations`），进程会拒绝启动并提示配置空数据库；不会导入其他产品数据库、身份或协议状态。上线前请单独备份现有数据库、配置和凭证主密钥。

发布顺序固定为：Primary → Collector → 清洁安装 Agent。Dashboard 与 Agent 必须成对升级；不要将 Collector 与 Agent 角色混淆。
