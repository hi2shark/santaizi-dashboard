# 配置参考

Dashboard 的配置文件默认位于 `/etc/santaizi/dashboard.yaml`，SQLite 与探测数据默认位于 `/var/lib/santaizi-dashboard/`。两者均可通过 CLI 或配置覆盖。配置加载顺序：

1. 环境变量（前缀 `SANTAIZI_`，下划线替换为点）
2. `/etc/santaizi/dashboard.yaml`

例如：

```bash
SANTAIZI_DEBUG=true
SANTAIZI_HTTPPORT=8080
SANTAIZI_SITE_BRAND="My Monitor"
SANTAIZI_TELEMETRY_STATE_INTERVAL_SECONDS=5
```

等价于：

```yaml
debug: true
httpport: 8080
site:
  brand: "My Monitor"
```

---

## 顶层配置项

| 配置项 | 类型 | 默认值 | 说明 |
|--------|------|--------|------|
| `debug` | bool | `false` | Debug 模式；开启后启用 `/debug/pprof` 和 `mock` OAuth2 |
| `mode` | string | `primary` | `primary` 启动控制面与 Web；`collector` 只启动探测接收、复制与 gRPC Health |
| `language` | string | `zh-CN` | 系统语言 |
| `httpport` | uint | `80` | Dashboard Web 端口 |
| `grpcport` | uint | `5555` | Agent gRPC 上报端口 |
| `grpchost` | string | `""` | 面板对外域名/IP，用于生成 Agent 一键安装命令 |
| `proxygrpcport` | uint | `0` | 如果设置，生成安装命令时使用该端口代替 `grpcport` |
| `tls` | bool | `false` | 下发 Agent 以 TLS 连接 Primary；证书通常由公网 gRPC 反向代理终止 |
| `enableplainipinnotification` | bool | `false` | 通知中 IP 是否不打码 |
| `enableipchangenotification` | bool | `false` | 是否启用服务器 IP 变动通知 |
| `ipchangenotificationtag` | string | `default` | IP 变动通知使用的通知组 |
| `cover` | uint8 | `0` | IP 变动覆盖范围：`0`=全部服务器（排除特定），`1`=仅特定服务器 |
| `ignoredipnotification` | string | `""` | 特定服务器 ID，逗号分隔 |
| `location` | string | `Asia/Shanghai` | 时区 |
| `maxtcppingvalue` | int32 | `1000` | TCP Ping 最大值（ms） |
| `avgpingcount` | int | `2` | 平均 Ping 次数 |
| `dnsservers` | string | `""` | 自定义 DNS 服务器，逗号分隔 |
| `enableofflinehistory` | bool | `true` | 是否启用服务器离线历史 |
| `offlinethresholdseconds` | uint64 | `30` | 离线判定阈值（秒，最小 `10`） |
| `offlinecheckintervalseconds` | uint64 | `10` | 离线检测间隔（秒，最小 `5`，且 ≤ 阈值）；修改后热生效，无需重启 |
| `offlinemergegapseconds` | uint64 | `10` | 离线合并间隔（秒，1~3600）：相邻两次离线之间的在线时间 ≤ 该值时合并为一次，默认 10 |
| `offlinehistoryretentiondays` | uint64 | `365` | 离线历史保留天数 |
| `enableofflinenotification` | bool | `false` | 离线时发送通知 |
| `enablerecoverynotification` | bool | `false` | 恢复时发送通知 |
| `showavailabilitytoguest` | bool | `false` | 是否向前台访客展示服务器可用性摘要（30 天可用率、离线次数等） |

> 离线历史相关配置在后台设置页面保存后立即生效，无需重启 Dashboard。

---

## `site` 站点配置

| 配置项 | 默认值 | 说明 |
|--------|--------|------|
| `site.brand` | `Santaizi Monitoring` | 站点标题 |
| `site.cookiename` | `santaizi-dashboard` | 登录 Cookie 名 |
| `site.viewpassword` | `""` | 前台查看密码，留空表示不需要 |
| `site.primarycolor` | `#2563eb` | ServerStatus 品牌色 |
| `site.footertext` | `""` | 公开站页脚文字 |
| `site.logourl` | `/static/logo.svg` | 本地或 data image Logo |
| `site.backgroundurl` | ServerStatus 背景 | 本地或 data image 背景 |
| `site.safecustomcss` | `""` | 受限 CSS；禁止远程和可执行规则 |
| `web.delivery` | `embedded` | `embedded` 或同域反向代理下的 `external` |

---

## `oauth2` 登录配置

| 配置项 | 必填 | 说明 |
|--------|------|------|
| `oauth2.type` | 是 | 可选：`github`、`gitee`、`gitlab`、`jihulab`、`gitea`、`cloudflare`、`oidc`、`mock` |
| `oauth2.admin` | 是 | 管理员用户名/ID，逗号分隔 |
| `oauth2.admingroups` | 否 | OIDC 管理员用户组 |
| `oauth2.clientid` | 除 `mock` 外 | OAuth2 Client ID |
| `oauth2.clientsecret` | 除 `mock` 外 | OAuth2 Client Secret |
| `oauth2.endpoint` | 自建时 | 自建 Gitea / Cloudflare Access 的 endpoint |
| `oauth2.oidcdisplayname` | 否 | 默认 `OIDC` |
| `oauth2.oidcissuer` | OIDC | OIDC issuer URL |
| `oauth2.oidclogouturl` | 否 | OIDC 登出地址 |
| `oauth2.oidcregisterurl` | 否 | OIDC 注册地址 |
| `oauth2.oidcloginclaim` | 否 | 默认 `sub` |
| `oauth2.oidcgroupclaim` | 否 | 默认 `groups` |
| `oauth2.oidcscopes` | 否 | 默认 `openid,profile,email` |
| `oauth2.oidcautocreate` | 否 | 是否自动创建用户，默认 `false` |
| `oauth2.oidcautocreate` | 否 | 是否自动登录，默认 `false` |

> `mock` 类型仅在 `debug: true` 时可用，仅用于本地开发。
> OAuth2 只在 `mode: primary` 时要求配置；Collector 模式不会加载 OAuth、HTTP、业务 UI、告警或内部调度。

---

## `telemetry` 可靠探测

| 配置项 | 默认值 | 说明 |
|--------|--------|------|
| `telemetry.data_dir` | `/var/lib/santaizi-dashboard` | 签名密钥和探测运行数据目录 |
| `telemetry.signing_key_path` | `<data_dir>/telemetry-signing.key` | Primary Ed25519 私钥；必须持久保存并限制权限 |
| `telemetry.primary_endpoint` | `grpchost` 或本机 gRPC | Agent 控制流下发的 Primary 地址 |
| `telemetry.state_interval_seconds` | `5` | State 采样间隔 |
| `telemetry.heartbeat_interval_seconds` | `10` | Heartbeat 间隔 |
| `telemetry.offline_threshold_seconds` | `30` | 新鲜度与离线判定阈值 |
| `telemetry.ingest_batch_size` | `256` | V2 接收批大小 |
| `telemetry.ingest_queue_size` | `4096` | 接收侧有界容量 |
| `telemetry.credential_validity_days` | `30` | Agent 探测凭据有效期 |
| `telemetry.credential_refresh_days` | `7` | 到期前刷新窗口 |
| `telemetry.credential_grace_days` | `7` | Collector 离线且已有授权时的过期宽限 |
| `telemetry.availability_bucket_seconds` | `30` | Availability Bucket 大小 |
| `telemetry.min_observers` | `1` | 判定 `OFFLINE` 所需的最少健康 Observer 数 |

`enable_connectivity_notification`、`enable_correction_notification`、`enable_collector_offline_notification` 与 `enable_data_loss_notification` 位于 `telemetry` 下，默认均为 `false`。Host 离线通知继续由顶层 `enableofflinenotification` 控制。

## `collector` Collector 模式

| 配置项 | 默认值 | 说明 |
|--------|--------|------|
| `collector.primary_endpoint` | 无 | Primary gRPC 地址，Collector 模式必填 |
| `collector.primary_tls` | `false` | 连接 Primary 时使用 TLS |
| `collector.primary_insecure_tls` | `false` | 跳过证书校验，仅供受控测试 |
| `collector.registration_token` | 无 | Primary 管理后台可查看和轮换的注册 Token |
| `collector.database_path` | `<telemetry.data_dir>/collector.db` | Collector 本地 SQLite |
| `collector.spool_max_bytes` | `5368709120` | Spool Hard Limit（5 GiB） |
| `collector.spool_max_age_days` | `30` | Spool 保留上限 |
| `collector.status_authorization` | 无 | 调用鉴权 `GetStatus` 的共享值 |

示例：

```yaml
mode: collector
grpcport: 5556
telemetry:
  data_dir: /var/lib/santaizi-dashboard
collector:
  primary_endpoint: primary.example.com:5555
  primary_tls: true
  registration_token: "从 Primary 管理后台复制的注册 Token"
  database_path: /var/lib/santaizi-dashboard/collector.db
  spool_max_bytes: 5368709120
  spool_max_age_days: 30
  status_authorization: "本地运维鉴权值"
```

## `rollup` 与 `retention`

| 配置项 | 默认值 |
|--------|--------|
| `rollup.enabled` | `true` |
| `rollup.batch_size` | `1000` |
| `retention.state_raw_hours` | `6` |
| `retention.state_one_minute_days` | `30` |
| `retention.state_one_hour_days` | `365` |
| `retention.observation_days` | `30` |
| `retention.lifecycle_days` | `3650` |
| `retention.batch_size` | `1000` |

Retention 使用小批后台清理；State Payload 只有在对应 Rollup 完成后才会被清空。

### GitHub OAuth2 示例

```yaml
oauth2:
  type: github
  admin: your-github-username
  clientid: xxxxxxxxxxxxxxxxxxxx
  clientsecret: xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
```

### OIDC 示例

```yaml
oauth2:
  type: oidc
  admin: admin-user-id
  admingroups: admin-group
  clientid: santaizi-dashboard
  clientsecret: xxxxxxxxxxxx
  oidcdisplayname: SSO
  oidcissuer: https://auth.example.com
  oidclogouturl: https://auth.example.com/logout
  oidcscopes: openid,profile,email
  oidcgroupclaim: groups
  oidcautocreate: true
```

---

## `installscript` 安装脚本源

用于 Dashboard 中生成 Agent 一键安装命令的脚本地址。

| 配置项 | 默认值 |
|--------|--------|
| `installscript.linux` | `https://raw.githubusercontent.com/hi2shark/santaizi-dashboard/master/script/install_agent.sh` |
| `installscript.linuxen` | `https://raw.githubusercontent.com/hi2shark/santaizi-dashboard/master/script/install_agent_en.sh` |
| `installscript.windows` | `https://raw.githubusercontent.com/hi2shark/santaizi-dashboard/master/script/install.ps1` |
| `installscript.macos` | `https://raw.githubusercontent.com/hi2shark/santaizi-dashboard/master/script/install.command` |

如果你 fork 了仓库或在内网部署，建议替换为自有地址。

---

## 配置示例

```yaml
debug: false
mode: primary
language: zh-CN
httpport: 80
grpcport: 5555
grpchost: santaizi.example.com
location: Asia/Shanghai

site:
  brand: "三太子监控"
  viewpassword: ""
  primarycolor: "#2563eb"

web:
  delivery: embedded

oauth2:
  type: github
  admin: "your-github-username"
  clientid: "xxx"
  clientsecret: "xxx"

enableofflinehistory: true
offlinethresholdseconds: 30
offlinecheckintervalseconds: 10
offlinehistoryretentiondays: 365
enableofflinenotification: false
enablerecoverynotification: false

telemetry:
  data_dir: /var/lib/santaizi-dashboard
  secret_key_path: /var/lib/santaizi-dashboard/business-secrets.key
  primary_endpoint: santaizi.example.com:5555
  state_interval_seconds: 5
  heartbeat_interval_seconds: 10
  offline_threshold_seconds: 30
  availability_bucket_seconds: 30
  min_observers: 1

rollup:
  enabled: true
  batch_size: 1000

retention:
  state_raw_hours: 6
  state_one_minute_days: 30
  state_one_hour_days: 365
  observation_days: 30
  lifecycle_days: 3650
  batch_size: 1000
```

---

## 在线修改配置

大部分业务配置可以在 Dashboard 的 **设置** 页面（`/admin/settings`）中在线修改并保存到 `/etc/santaizi/dashboard.yaml`。修改后无需重启，即时生效。

`mode`、端口、TLS、数据目录、签名密钥、Collector 连接以及 Rollup/Retention Worker 配置需要重启才能完整生效。

`telemetry.secret_key_path` 保存业务凭证的 AES-256-GCM 主密钥。Primary 首次启动会以 `0600` 权限生成该文件；备份数据库时必须一并备份，且不得通过后台或 API 读取。

> 数据库策略：仅接受空数据库，或由当前版本创建且包含 `schema_migrations` 的数据库；其他非空数据库会拒绝启动。
