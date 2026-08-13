# API v2 与管理页面覆盖矩阵

`openapi/v2.yaml` 是 HTTP API 的唯一契约。Go DTO/接口与 Axios SDK 均由该文件生成；管理页面只调用生成 SDK 的类型化封装。

| 业务能力 | OpenAPI operationId | 页面操作 | 自动化验收 |
|---|---|---|---|
| 登录与退出 | `getSession`, `logout` | 顶栏用户与退出 | 会话、CSRF 与 SPA 导航 |
| 服务器 CRUD | `createServer`, `updateServer`, `deleteServer` | `/admin/servers` 专用编辑弹窗与删除 | 公开备注结构化编辑、脏数据关闭确认 |
| 服务器批量管理 | `batchUpdateServerGroup`, `batchDeleteServers` | 表格多选、批量分组和删除 | 批量请求与危险确认 |
| 服务器排序与分组管理 | `updateServerDisplayIndex`, `listServerGroups`, `renameServerGroup` | 列表行内改序、分组管理弹窗、编辑器分组下拉 | 单字段改序与派生 tag 重命名/合并 |
| 凭据与安装 | `getServerCredential`, `resetServerSecret`, `getProbeCapabilities`, `getServerInstallPreview` | 密钥查看/复制、分平台能力化安装弹窗 | 标准·云/标准·物理/轻量/仅存活、IP 位置子选项与清洁安装确认 |
| 流量策略 | `listTrafficPolicies`, `createTrafficPolicy`, `updateTrafficPolicy`, `deleteTrafficPolicy`, `getTrafficPolicyUsage` | 服务器编辑器内多策略卡片 | 累计/周期策略与用量进度 |
| 可用性与离线历史 | `listServerAvailability`, `resetServerAvailability`, `deleteOfflineHistory`, `cleanupOfflineHistory` | 服务器历史抽屉与设置页清理 | 历史读取、重置和删除 |
| 服务监控 | `createMonitor`, `updateMonitor`, `deleteMonitor` | `/admin/services` HTTP/ICMP/TCP 编辑弹窗和服务器穿梭框 | 完整 CRUD、范围和历史 |
| 通知渠道 | `createNotification`, `updateNotification`, `deleteNotification`, `testNotification` | `/admin/notifications` 类型化请求编辑器 | 请求头键值表、请求体、TLS 与测试 |
| 告警规则 | `createAlertRule`, `updateAlertRule`, `deleteAlertRule` | `/admin/alert-rules` 可视化条件卡片 | 指标、阈值、持续时间、服务器范围和通知组 |
| DDNS | `createDDNSProfile`, `updateDDNSProfile`, `deleteDDNSProfile` | 附加功能中的 Provider 驱动编辑器 | 域名、协议、凭据和 Webhook 动态字段 |
| NAT | `createNATTunnel`, `updateNATTunnel`, `deleteNATTunnel` | 附加功能中的服务器选择器与目标表单 | 完整 CRUD 与目标格式校验 |
| 系统设置 | `updateSettings` | `/admin/settings` | 站点、网络、可用性、通知和安全外观 |
| API Token | `listApiTokens`, `createApiToken`, `getApiToken`, `patchApiToken`, `deleteApiToken` | `/admin/api-tokens` 签发（权限/有效期）、列表复制、启用/禁用与删除 | 只读/操作权、过期与禁用鉴权；明文仅详情返回 |
| Collector 生命周期 | `createCollector`, `updateCollector`, `getCollectorToken`, `rotateCollectorToken`, `revokeCollector`, `deleteCollector`, `getCollectorInstallPreview` | `/admin/telemetry` 专用编辑弹窗、安装命令与操作菜单 | Token 查看/轮换、安装预览、撤销和删除 |
| Collector Scope | `updateCollectorScope` | All/Server/Group/Tag 类型化范围 | Scope 选择与配置版本更新 |
| 连接观察 | `getConnectionSummary`, `listConnectionPaths`, `listConnectionLatency`, `listCollectors` | `/admin/connections` 主从表与节点路径及延迟列/抽屉历史；总览连接摘要；主机历史抽屉连接页 | 心跳派生从端状态、路径 sink、RTT 最近一次与 24h 分钟桶 |
| 公开可用性与资源历史 | `getPublicServerAvailability`, `getPublicMetrics` | Nazhua 详情资源曲线；可用性受 `show_availability_to_guest` 门控 | 匿名 403、无绑定空 list、rollup Average 序列 |
| 可靠探测数据 | `listObserverAssignments`, `listAgentReliability`, `listIncidents`, `listIncidentRevisions`, `listTelemetryDataLoss`, `listTelemetryAlerts` | `/admin/telemetry` 六个数据 tab 固定列、只读抽屉与 `page`/`page_size` 翻页 | 解码 sink/证据、RFC3339 时间、分类小写、截断 UUID、列表分页 |

公开端由 `getPublicBootstrap`、`createViewPasswordSession`、`listPublicServers`、`getPublicServer`、`listPublicServices`、`getPublicNetworkHistory`、`listPublicCycleTransfer`、`getPublicServerAvailability` 与 `getPublicMetrics` 驱动；bootstrap 含 `theme` / `allow_frontend_theme_switch`。`getPublicNetworkHistory` 的 `data` 为延迟历史数组；周期流量含 `warning_percent` / `remaining_bytes` / `next_reset_at`。`ServerHost` / `ServerState` / `SensorTemperature` 为封闭 schema（`additionalProperties: false`），字段与 `model.Host` / `model.HostState` 一一对应且只用 PascalCase，前端拼错字段在 typecheck 阶段即失败。

契约防回退包括 OpenAPI lint、可重复代码生成、Gin operation 注册、四语言键检查、Vitest、Playwright、Go 测试和双仓构建。
