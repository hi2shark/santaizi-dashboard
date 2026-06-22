# 配置参考

Dashboard 的配置文件默认位于 `data/config.yaml`。配置加载顺序：

1. 环境变量（前缀 `NZ_`，下划线替换为点）
2. `data/config.yaml`

例如：

```bash
NZ_DEBUG=true
NZ_HTTPPORT=8080
NZ_SITE_BRAND="My Monitor"
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
| `language` | string | `zh-CN` | 系统语言 |
| `httpport` | uint | `80` | Dashboard Web 端口 |
| `grpcport` | uint | `5555` | Agent gRPC 上报端口 |
| `grpchost` | string | `""` | 面板对外域名/IP，用于生成 Agent 一键安装命令 |
| `proxygrpcport` | uint | `0` | 如果设置，生成安装命令时使用该端口代替 `grpcport` |
| `tls` | bool | `false` | gRPC 是否启用 TLS |
| `enableplainipinnotification` | bool | `false` | 通知中 IP 是否不打码 |
| `disableswitchtemplateinfrontend` | bool | `false` | 前台是否禁用切换主题 |
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
| `offlinecheckintervalseconds` | uint64 | `10` | 离线检测间隔（秒，最小 `5`，且 ≤ 阈值） |
| `offlinemergegapseconds` | uint64 | `10` | 离线合并间隔（保留配置） |
| `offlinehistoryretentiondays` | uint64 | `365` | 离线历史保留天数 |
| `enableofflinenotification` | bool | `false` | 离线时发送通知 |
| `enablerecoverynotification` | bool | `false` | 恢复时发送通知 |
| `showavailabilitytoguest` | bool | `false` | 是否向前台访客展示服务器可用性摘要（30 天可用率、离线次数等） |

> 离线历史相关配置在后台设置页面保存后立即生效，无需重启 Dashboard。

---

## `site` 站点配置

| 配置项 | 默认值 | 说明 |
|--------|--------|------|
| `site.brand` | `Nezha Monitoring` | 站点标题 |
| `site.cookiename` | `nezha-dashboard` | 登录 Cookie 名 |
| `site.theme` | `default` | 前台主题 |
| `site.dashboardtheme` | `default` | 后台主题 |
| `site.customcode` | `""` | 前台自定义代码（HTML/JS/CSS） |
| `site.customcodedashboard` | `""` | 后台自定义代码 |
| `site.viewpassword` | `""` | 前台查看密码，留空表示不需要 |

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
  clientid: nezha-dashboard
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
| `installscript.linux` | `https://raw.githubusercontent.com/hi2shark/nezha-next/master/script/install_agent.sh` |
| `installscript.linuxen` | `https://raw.githubusercontent.com/hi2shark/nezha-next/master/script/install_agent_en.sh` |
| `installscript.windows` | `https://raw.githubusercontent.com/hi2shark/nezha-next/master/script/install.ps1` |
| `installscript.macos` | `https://raw.githubusercontent.com/hi2shark/nezha-next/master/script/install.command` |

如果你 fork 了仓库或在内网部署，建议替换为自有地址。

---

## 配置示例

```yaml
debug: false
language: zh-CN
httpport: 80
grpcport: 5555
grpchost: nezha.example.com
location: Asia/Shanghai

site:
  brand: "哪吒监控"
  theme: "default"
  dashboardtheme: "default"
  viewpassword: ""

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
```

---

## 在线修改配置

大部分配置可以在 Dashboard 的 **设置** 页面（`/setting`）中在线修改并保存到 `data/config.yaml`。修改后无需重启，即时生效。

需要重启才能生效的项（如端口、TLS）建议直接修改 `data/config.yaml` 后重启容器或进程。
