# Santaizi 使用文档

欢迎使用 **Santaizi** —— 一个开源、自托管的服务器监控与运维面板。本文档将帮助你完成安装、配置和日常使用。

> 本文档对应项目源码：`https://github.com/hi2shark/santaizi-dashboard`

## 快速导航

- [快速开始](quickstart.md) — 5 分钟部署 Dashboard 和第一台 Agent
- [安装指南](installation.md) — Dashboard / Agent 的完整安装方式
- [配置参考](configuration.md) — `config.yaml` 全量配置项说明
- [服务器管理](servers.md) — 添加服务器、分组、离线历史
- [服务监控](monitoring.md) — HTTP / ICMP / TCP 监控与告警规则
- [计划任务](cron-tasks.md) — 定时任务与触发任务
- [通知方式](notifications.md) — Webhook 通知与变量占位符
- [动态 DNS (DDNS)](ddns.md)
- [内网穿透 (NAT)](nat.md)
- [API 文档](api.md)
- [主题与自定义](themes.md)
- [主题开发对接指南](theme-development.md)
- [常见问题](faq.md)

## 核心概念

| 术语 | 说明 |
|------|------|
| **Dashboard** | 面板服务端，负责接收 Agent 数据、展示页面、发送通知 |
| **Agent** | 部署在被监控机器上的客户端，负责采集系统状态并上报 |
| **服务器** | 被监控的节点，每台机器对应一个 Agent |
| **监控** | 对目标地址的 HTTP / ICMP / TCP 检测 |
| **任务** | 在指定服务器上执行的计划任务或触发任务 |
| **通知** | 通过 Webhook 发送的告警/事件消息 |

## 系统要求

### Dashboard

- Docker + Docker Compose（推荐）
- 或 Linux 服务器 + Go 1.25+（手动运行二进制）
- 至少 1 核 CPU / 512MB 内存 / 1GB 磁盘
- 需要暴露 Web 端口（默认 `80`）和 gRPC 端口（默认 `5555`）

### Agent

- Linux（amd64 / arm64 / arm）
- Windows（amd64）
- macOS（amd64 / arm64）
- 仅需能访问 Dashboard 的 gRPC 端口

## 默认端口

| 服务 | 默认端口 | 说明 |
|------|----------|------|
| Web 面板 | `80` | HTTP 访问端口，可在 `config.yaml` 中修改 |
| gRPC 上报 | `5555` | Agent 与 Dashboard 通信端口 |

## 安全提示

1. 生产环境务必配置 OAuth2 / OIDC 登录，不要使用 `mock` 类型。
2. gRPC 端口建议只向 Agent 开放，不要直接暴露到公网。
3. 使用 HTTPS 反向代理（如 Nginx / Caddy）保护 Web 面板。
4. 定期备份 `data/sqlite.db` 和 `data/config.yaml`。
