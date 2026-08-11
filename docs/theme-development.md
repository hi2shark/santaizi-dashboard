# 前端主题开发对接指南

本文档面向希望为 Santaizi 开发或对接前台/后台主题的开发者，说明主题系统的结构、约定、可用数据与接口。

> 本文档对应项目源码：`https://github.com/hi2shark/santaizi-dashboard`
> 
> 如果你只是想使用或切换主题，请阅读 [主题与自定义](themes.md)。

---

## 目录

- [核心架构](#核心架构)
- [主题类型](#主题类型)
- [目录与文件结构](#目录与文件结构)
- [创建前台主题](#创建前台主题)
- [模板规范](#模板规范)
- [theme.json 元数据](#themejson-元数据)
- [静态资源](#静态资源)
- [模板变量与函数](#模板变量与函数)
- [Servers 数据结构](#servers-数据结构)
- [后台主题变量](#后台主题变量)
- [服务监控数据结构](#服务监控数据结构)
- [网络监控数据结构](#网络监控数据结构)
- [可用性数据结构](#可用性数据结构)
- [实时数据与接口](#实时数据与接口)
- [前端框架与库](#前端框架与库)
- [国际化](#国际化)
- [自定义主题与本地覆盖](#自定义主题与本地覆盖)
- [主题切换机制](#主题切换机制)
- [开发调试与发布](#开发调试与发布)
- [最小可运行示例](#最小可运行示例)
- [常见问题](#常见问题)

---

## 核心架构

- **后端**：Go 1.25 + Gin 框架。
- **模板引擎**：标准库 `html/template`，所有模板在启动时解析并注册到 Gin。
- **资源嵌入**：`resource/` 下的模板与静态资源通过 `//go:embed` 嵌入二进制；运行时可通过本地文件系统覆盖。
- **前端模式**：服务端渲染 HTML + 静态资源。Santaizi **没有前端构建流程**（无 npm / Webpack / Vite），主题由纯 HTML / CSS / JS 组成。
- **实时更新**：页面通过 WebSocket `/ws` 接收服务器状态；图表与可用性数据通过 REST API 获取。

---

## 主题类型

| 类型 | 目录前缀 | 用途 | 示例 |
|------|----------|------|------|
| 前台主题 | `theme-<key>/` | 游客可见的首页、服务监控、网络监控等页面 | `theme-default`、`theme-server-status` |
| 后台主题 | `dashboard-<key>/` | 管理后台页面 | `dashboard-default` |

前台主题需要游客可访问，因此数据经过脱敏；后台主题通常面向登录用户，可直接使用完整配置。

---

## 目录与文件结构

```
resource/
├── resource.go              # 嵌入 static/、template/、l10n/
├── template/
│   ├── common/              # 共享片段：csrf、header、footer 等
│   ├── component/           # 可复用组件（弹窗、表单）
│   ├── dashboard-default/   # 内置后台主题
│   └── theme-<key>/         # 前台主题目录
│       ├── theme.json       # 第三方主题必填
│       ├── home.html        # 首页（必填）
│       ├── header.html      # 页面头部
│       ├── footer.html      # 页面尾部
│       ├── menu.html        # 导航菜单
│       ├── service.html     # 服务监控页
│       ├── network.html     # 网络监控页
│       └── viewpassword.html# 查看密码页
└── static/
    ├── custom/              # 本地静态资源覆盖目录
    ├── theme-<key>/         # 主题专属 CSS/JS/字体
    ├── unpkg/               # 第三方库（Vue、ECharts 等）
    ├── main.css/js          # 公共前台资源
    └── dashboard.css/js     # 公共后台资源
```

---

## 创建前台主题

以创建名为 `mytheme` 的前台主题为例：

### 1. 创建目录

```bash
mkdir -p resource/template/theme-mytheme
mkdir -p resource/static/theme-mytheme/css
mkdir -p resource/static/theme-mytheme/js
```

### 2. 编写 `theme.json`

```json
{
  "name": "My Theme"
}
```

> 内置主题（如 `theme-default`）不需要 `theme.json`；第三方本地主题或希望被系统识别为主题包时必须提供。

### 3. 编写 `home.html`

至少提供 `home` 模板定义：

```html
{{define "theme-mytheme/home"}}
{{template "theme-mytheme/header" .}}
{{template "theme-mytheme/menu" .}}

<div class="container">
    <h1>{{.Conf.Site.Brand}}</h1>
    <p>服务器数量：{{len .Servers}}</p>
</div>

{{template "theme-mytheme/footer" .}}
{{end}}
```

### 4. 放置静态资源

```
resource/static/theme-mytheme/css/main.css
resource/static/theme-mytheme/js/app.js
```

在 `header.html` 中引用：

```html
<link rel="stylesheet" href="/static/theme-mytheme/css/main.css">
<script src="/static/theme-mytheme/js/app.js"></script>
```

### 5. 启用主题

在 **设置** 页面选择 `My Theme`，或修改 `config.yaml`：

```yaml
site:
  theme: "mytheme"
```

---

## 模板规范

### 模板定义名（define name）

所有主题模板必须使用 `{{define "..."}}` / `{{end}}` 包裹。系统按以下名称查找并渲染：

| 模板定义名 | 说明 | 是否必填 |
|------------|------|----------|
| `theme-<key>/home` | 首页 | 是 |
| `theme-<key>/service` | 服务监控页 | 推荐 |
| `theme-<key>/network` | 网络监控页 | 推荐 |
| `theme-<key>/viewpassword` | 查看密码页 | 推荐 |
| `theme-<key>/header` | 页面头部 | 推荐 |
| `theme-<key>/footer` | 页面尾部 | 推荐 |
| `theme-<key>/menu` | 导航菜单 | 推荐 |

> 保存设置时，系统会校验 `resource/template/theme-<key>/home.html` 是否存在。只有存在该文件才会被接受。

### 模板继承与引用

在主题内部，使用 `{{template "theme-<key>/header" .}}` 引用自己的片段；使用 `{{template "common/csrf" .}}` 引用公共片段。

示例 `header.html`：

```html
{{define "theme-mytheme/header"}}
<!DOCTYPE html>
<html lang="{{.Conf.Language}}">
<head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>{{.Title}}</title>
    <link rel="stylesheet" href="/static/theme-mytheme/css/main.css">
    {{template "common/csrf" .}}
</head>
<body>
{{end}}
```

示例 `footer.html`：

```html
{{define "theme-mytheme/footer"}}
<footer>
    &copy; {{.Conf.Site.Brand}} | Powered by Santaizi {{.Version}}
</footer>
</body>
</html>
{{end}}
```

### 注入自定义代码

主题开发者无需关心用户自定义代码，但应在合适位置留出注入点：

```html
{{if .CustomCode}}
{{.CustomCode | safe}}
{{end}}
```

---

## theme.json 元数据

第三方主题必须在目录下提供 `theme.json`，最小内容：

```json
{
  "name": "主题显示名称"
}
```

启动时，系统会扫描 `resource/template/` 下所有 `theme-<key>/` 目录，读取 `theme.json` 中的 `name` 字段，并将 `theme-<key>` 注册到 `model.Themes` 映射中。这个映射会出现在：

- 后台设置页的主题下拉框。
- 前台主题切换菜单（`{{.Themes}}`）。

> 内置主题不需要 `theme.json`；`theme-custom` 为兼容旧版本，也不需要 `theme.json`。

---

## 静态资源

### 主题专属资源

建议放在 `resource/static/theme-<key>/` 下，按 `css/`、`js/`、`fonts/` 等子目录组织。

### 公共第三方库

项目已内置常用库，可直接引用：

```html
<!-- 示例：default 主题引用的库 -->
<link rel="stylesheet" href="/static/unpkg/semantic-ui@2.4.0/dist/semantic.min.css">
<script src="/static/unpkg/jquery@3.7.1/dist/jquery.min.js"></script>
<script src="/static/unpkg/vue@2.6.14/dist/vue.min.js"></script>
<script src="/static/unpkg/echarts@5.5.0/dist/echarts.min.js"></script>
```

可用库位于 `resource/static/unpkg/` 与 `resource/static/` 中，主题开发者可直接使用，无需自行引入 CDN。

### 本地覆盖机制

`resource/static/custom/` 目录下的文件会覆盖嵌入的同名静态资源。适合用户在不重新编译二进制的情况下覆盖主题 CSS/JS/图片。

---

## 模板变量与函数

### 公共环境变量

所有页面都会经过 `mygin.CommonEnvironment`，可在模板中直接使用：

| 变量 | 类型 | 说明 |
|------|------|------|
| `.MatchedPath` | string | 当前匹配的路由路径 |
| `.Version` | string | Dashboard 版本号 |
| `.CSRFToken` | string | CSRF Token，需通过 `{{template "common/csrf" .}}` 注入 |
| `.Conf` | `model.PublicConfig` / `model.Config` | 游客可见公开配置；登录后可见完整配置 |
| `.Themes` | `map[string]string` | 可用前台主题：`key` → `显示名` |
| `.CustomCode` | string | 前台自定义代码（原始 HTML/JS/CSS） |
| `.CustomCodeDashboard` | string | 后台自定义代码 |
| `.IsAdminPage` | bool | 当前是否为管理页面 |
| `.IsDashboardPage` | bool | 当前是否为管理页面或登录页 |
| `.Title` | string | 页面标题 |
| `.Admin` | `*model.User` | 当前登录用户（未登录为 nil） |
| `.LANG` | map | 后台 JS 使用的翻译字典 |

### 公开配置结构

```go
type PublicSiteConfig struct {
    Brand               string
    Theme               string
    DashboardTheme      string
    CustomCode          string
    CustomCodeDashboard string
}

type PublicConfig struct {
    Site                            PublicSiteConfig
    Language                        string
    MaxTCPPingValue                 int32
    DisableSwitchTemplateInFrontend bool
}
```

### `.LANG` 翻译字典

`.LANG` 是后台主题 JS 使用的翻译映射，key 为消息 ID，value 为当前语言下的翻译文本。前台主题通常直接在 Go 模板中使用 `{{tr "..."}}`，不需要用到 `.LANG`。

示例：

```javascript
const LANG = {{.LANG}};
console.log(LANG["ScheduledTasks"]); // "计划任务"
```

### `.Admin` 当前登录用户

| 字段 | 类型 | 说明 |
|------|------|------|
| `ID` | uint64 | 用户 ID |
| `Name` | string | 用户名/昵称 |
| `Login` | string | 登录账号 |
| `Role` | uint8 | 角色：0 普通用户，1 管理员 |
| `CreatedAt` | time.Time | 创建时间 |
| `UpdatedAt` | time.Time | 更新时间 |

> 未登录时 `.Admin` 为 `null`。

### 页面专属变量

#### 首页（`home.html`）

```html
<script>
    var data = JSON.parse('{{.Servers}}');
    // data.now: 服务端当前时间戳（毫秒）
    // data.servers: 服务器对象数组
</script>
```

`.Servers` 是 JSON 字符串，结构为：

```json
{
  "now": 1234567890000,
  "servers": [ { /* model.Server */ }, ... ]
}
```

---

## Servers 数据结构

首页与网络监控页通过 `.Servers` 接收服务器列表。下面给出前端实际能拿到的字段说明。

### `Server`

| 字段 | 类型 | 说明 |
|------|------|------|
| `ID` | uint64 | 服务器唯一 ID |
| `Name` | string | 服务器名称 |
| `Tag` | string | 分组标签 |
| `PublicNote` | string | 公开备注 |
| `DisplayIndex` | int | 展示排序，越大越靠前 |
| `HideForGuest` | bool | 是否对游客隐藏 |
| `EnableDDNS` | bool | 是否启用 DDNS |
| `Host` | `Host` / null | 主机静态信息（Agent 首次上报） |
| `State` | `HostState` / null | 实时状态 |
| `LastActive` | string (RFC3339) | 最近一次活跃时间 |

> `Secret`、`Note` 等敏感字段不会返回给前台主题。

### `Host`（主机静态信息）

| 字段 | 类型 | 说明 |
|------|------|------|
| `Platform` | string | 操作系统平台，如 `linux`、`windows`、`darwin` |
| `PlatformVersion` | string | 系统版本 |
| `CPU` | []string | CPU 信息列表，每条通常形如 `Model @ Frequency (N Physical / M Virtual)` |
| `MemTotal` | uint64 | 内存总量（字节） |
| `DiskTotal` | uint64 | 磁盘总量（字节） |
| `SwapTotal` | uint64 | Swap 总量（字节） |
| `Arch` | string | 架构，如 `amd64`、`arm64` |
| `Virtualization` | string | 虚拟化方式，如 `kvm`、`vmware` |
| `BootTime` | uint64 | 启动时间戳（秒） |
| `CountryCode` | string | 国家代码，小写两字母，如 `cn`、`us` |
| `Version` | string | Agent 版本号 |
| `GPU` | []string | GPU 信息列表 |

### `HostState`（实时状态）

| 字段 | 类型 | 说明 |
|------|------|------|
| `CPU` | float64 | CPU 使用率，0–100 |
| `MemUsed` | uint64 | 已用内存（字节） |
| `SwapUsed` | uint64 | 已用 Swap（字节） |
| `DiskUsed` | uint64 | 已用磁盘（字节） |
| `NetInTransfer` | uint64 | 累计入站流量（字节） |
| `NetOutTransfer` | uint64 | 累计出站流量（字节） |
| `NetInSpeed` | uint64 | 实时入站速率（字节/秒） |
| `NetOutSpeed` | uint64 | 实时出站速率（字节/秒） |
| `Uptime` | uint64 | 运行时长（秒） |
| `Load1` | float64 | 1 分钟平均负载 |
| `Load5` | float64 | 5 分钟平均负载 |
| `Load15` | float64 | 15 分钟平均负载 |
| `TcpConnCount` | uint64 | TCP 连接数 |
| `UdpConnCount` | uint64 | UDP 连接数 |
| `ProcessCount` | uint64 | 进程数 |
| `Temperatures` | `SensorTemperature[]` | 温度传感器数据 |
| `GPU` | float64 | GPU 使用率 |

### `SensorTemperature`

| 字段 | 类型 | 说明 |
|------|------|------|
| `Name` | string | 传感器名称 |
| `Temperature` | float64 | 温度（摄氏度） |

### 完整示例

```json
{
  "now": 1718888888000,
  "servers": [
    {
      "ID": 1,
      "Name": "HK-01",
      "Tag": "Asia",
      "PublicNote": "香港节点",
      "DisplayIndex": 10,
      "HideForGuest": false,
      "EnableDDNS": false,
      "Host": {
        "Platform": "linux",
        "PlatformVersion": "Ubuntu 22.04.4 LTS",
        "CPU": ["Intel(R) Xeon(R) CPU @ 2.20GHz (1 Physical / 2 Virtual)"],
        "MemTotal": 2097152000,
        "DiskTotal": 53687091200,
        "SwapTotal": 0,
        "Arch": "amd64",
        "Virtualization": "kvm",
        "BootTime": 1718000000,
        "CountryCode": "hk",
        "Version": "v1.0.0",
        "GPU": []
      },
      "State": {
        "CPU": 12.5,
        "MemUsed": 1048576000,
        "SwapUsed": 0,
        "DiskUsed": 21474836480,
        "NetInTransfer": 1099511627776,
        "NetOutTransfer": 549755813888,
        "NetInSpeed": 102400,
        "NetOutSpeed": 51200,
        "Uptime": 86400,
        "Load1": 0.12,
        "Load5": 0.15,
        "Load15": 0.10,
        "TcpConnCount": 42,
        "UdpConnCount": 8,
        "ProcessCount": 156,
        "Temperatures": [
          { "Name": "coretemp_package_id_0", "Temperature": 45.0 }
        ],
        "GPU": 0
      },
      "LastActive": "2024-06-20T12:34:56+08:00"
    }
  ]
}
```

### 判断在线状态

`.Servers` 中的 `State` 或 `Host` 可能为 `null`。推荐通过 `data.now - new Date(server.LastActive).getTime() <= 10000` 判断是否在线，与内置主题保持一致：

```javascript
function markLive(server, now) {
    if (!server.Host) return false;
    return now - new Date(server.LastActive).getTime() <= 10 * 1000;
}
```

---

## 后台主题变量

后台主题（`dashboard-<key>/`）除了公共环境变量外，各页面还会传入以下数据：

| 页面 | 变量 | 类型 | 说明 |
|------|------|------|------|
| 服务器管理 `/server` | `.Servers` | `[]*Server` | 全部服务器列表 |
| 服务监控 `/monitor` | `.Monitors` | `[]*Monitor` | 全部监控任务 |
| 计划任务 `/cron` | `.Crons` | `[]model.Cron` | 全部计划任务 |
| 通知 `/notification` | `.Notifications` | `[]model.Notification` | 通知渠道 |
| 通知 `/notification` | `.AlertRules` | `[]model.AlertRule` | 告警规则 |
| DDNS `/ddns` | `.DDNS` | `[]model.DDNSProfile` | DDNS 配置 |
| DDNS `/ddns` | `.ProviderMap` | map | 提供商映射 |
| DDNS `/ddns` | `.ProviderList` | `[]DDNSProvider` | 提供商列表 |
| NAT `/nat` | `.NAT` | `[]model.NAT` | NAT 配置 |
| API 管理 `/api` | `.Tokens` | `[]*model.ApiToken` | API Token 列表 |
| 设置 `/setting` | `.Languages` | `map[string]string` | 可选语言列表 |
| 设置 `/setting` | `.DashboardThemes` | `map[string]string` | 可用后台主题 |
| 离线历史 `/server/offline-history` | `.ServerID` | uint64 | 查询的服务器 ID |
| 离线历史 `/server/offline-history` | `.Server` | `model.Server` | 服务器对象 |
| 终端 `/terminal/:id` | `.SessionID` | string | 会话 ID |
| 终端 `/terminal/:id` | `.ServerName` | string | 服务器名称 |
| 终端 `/terminal/:id` | `.ServerID` | uint64 | 服务器 ID |
| 文件管理 `/file/:id` | `.SessionID` | string | 会话 ID |

后台主题默认使用 **Vue 3 + Element Plus**，模板定界符与前台不同，但仍需注意与 Go 模板的 `{{ }}` 冲突。

### 后台数据结构字段说明

#### `Server`、`Host`、`HostState`

字段说明见前文 [Servers 数据结构](#servers-数据结构)。后台 `.Servers` 中的对象字段与前台一致，但包含管理员可见的额外运行时数据。

#### `Monitor`

字段说明见前文 [服务监控数据结构](#服务监控数据结构) 中的 `Monitor`。

#### `Cron`

| 字段 | 类型 | 说明 |
|------|------|------|
| `ID` | uint64 | 任务 ID |
| `Name` | string | 任务名称 |
| `TaskType` | uint8 | `0` 计划任务，`1` 触发任务 |
| `Scheduler` | string | Cron 表达式（仅计划任务） |
| `Command` | string | 执行的命令 |
| `Servers` | []uint64 | 目标服务器 ID 列表 |
| `PushSuccessful` | bool | 成功时是否推送通知 |
| `NotificationTag` | string | 通知组标签 |
| `LastExecutedAt` | time.Time | 最后执行时间 |
| `LastResult` | bool | 最后执行结果 |
| `Cover` | uint8 | `0` 仅覆盖特定服务器，`1` 仅忽略特定服务器，`2` 由触发服务器执行 |

#### `Notification`

| 字段 | 类型 | 说明 |
|------|------|------|
| `ID` | uint64 | 通知渠道 ID |
| `Name` | string | 名称 |
| `Tag` | string | 分组标签 |
| `URL` | string | Webhook URL |
| `RequestMethod` | int | `1` GET，`2` POST |
| `RequestType` | int | `1` JSON，`2` Form |
| `RequestHeader` | string | 请求头 JSON |
| `RequestBody` | string | 请求体模板 |
| `VerifySSL` | *bool | 是否校验 SSL |

#### `AlertRule`

| 字段 | 类型 | 说明 |
|------|------|------|
| `ID` | uint64 | 规则 ID |
| `Name` | string | 规则名称 |
| `RulesRaw` | string | 规则原始 JSON |
| `Enable` | *bool | 是否启用 |
| `TriggerMode` | int | `0` 始终触发，`1` 单次触发 |
| `NotificationTag` | string | 通知组标签 |
| `FailTriggerTasksRaw` | string | 失败时触发任务 ID JSON |
| `RecoverTriggerTasksRaw` | string | 恢复时触发任务 ID JSON |
| `Rules` | []Rule | 反序列化后的规则列表 |
| `FailTriggerTasks` | []uint64 | 失败时触发任务 ID |
| `RecoverTriggerTasks` | []uint64 | 恢复时触发任务 ID |

其中 `Rule` 结构：

| 字段 | 类型 | 说明 |
|------|------|------|
| `Type` | string | 指标类型：`cpu`、`memory`、`swap`、`disk`、`net_in_speed`、`net_out_speed`、`net_all_speed`、`transfer_in`、`transfer_out`、`transfer_all`、`offline`、`transfer_in_cycle`、`transfer_out_cycle`、`transfer_all_cycle` |
| `Min` | float64 | 最小阈值 |
| `Max` | float64 | 最大阈值 |
| `CycleStart` | *time.Time | 流量统计开始时间 |
| `CycleInterval` | uint64 | 流量统计周期数 |
| `CycleUnit` | string | 周期单位：`hour`、`day`、`week`、`month`、`year` |
| `Duration` | uint64 | 持续时间（秒） |
| `Cover` | uint64 | 覆盖范围 |
| `Ignore` | map[uint64]bool | 排除的服务器 ID |

#### `DDNSProfile`

| 字段 | 类型 | 说明 |
|------|------|------|
| `ID` | uint64 | 配置 ID |
| `Name` | string | 配置名称 |
| `EnableIPv4` | *bool | 是否启用 IPv4 |
| `EnableIPv6` | *bool | 是否启用 IPv6 |
| `MaxRetries` | uint64 | 最大重试次数 |
| `Provider` | uint8 | 提供商：`0` dummy、`1` webhook、`2` cloudflare、`3` tencentcloud |
| `AccessID` | string | 访问 ID |
| `AccessSecret` | string | 访问密钥 |
| `WebhookURL` | string | Webhook URL |
| `WebhookMethod` | uint8 | Webhook 请求方法 |
| `WebhookRequestType` | uint8 | Webhook 请求类型 |
| `WebhookRequestBody` | string | Webhook 请求体 |
| `WebhookHeaders` | string | Webhook 请求头 |
| `Domains` | []string | 目标域名列表 |

#### `NAT`

| 字段 | 类型 | 说明 |
|------|------|------|
| `ID` | uint64 | 配置 ID |
| `Name` | string | 配置名称 |
| `ServerID` | uint64 | 服务器 ID |
| `Host` | string | 内网地址 |
| `Domain` | string | 外网域名 |

#### `ApiToken`

| 字段 | 类型 | 说明 |
|------|------|------|
| `ID` | uint64 | Token ID |
| `UserID` | uint64 | 所属用户 ID |
| `Token` | string | Token 字符串 |
| `Note` | string | 备注 |

#### `User`（`.Admin`）

| 字段 | 类型 | 说明 |
|------|------|------|
| `ID` | uint64 | 用户 ID |
| `Login` | string | 登录名 |
| `AvatarURL` | string | 头像地址 |
| `Name` | string | 昵称 |
| `Blog` | string | 网站链接 |
| `Email` | string | 邮箱 |
| `Bio` | string | 个人简介 |
| `SuperAdmin` | bool | 是否为超级管理员 |

#### 后台完整配置 `.Conf`

登录后 `.Conf` 为完整 `model.Config`，主要字段：

| 字段 | 类型 | 说明 |
|------|------|------|
| `Debug` | bool | 调试模式 |
| `Language` | string | 系统语言 |
| `Site` | SiteConfig | 站点配置 |
| `HTTPPort` | uint | Web 端口 |
| `GRPCPort` | uint | gRPC 端口 |
| `GRPCHost` | string | gRPC 主机 |
| `ProxyGRPCPort` | uint | gRPC 代理端口 |
| `TLS` | bool | 是否启用 TLS |
| `EnablePlainIPInNotification` | bool | 通知中 IP 是否不打码 |
| `DisableSwitchTemplateInFrontend` | bool | 前台是否禁用切换主题 |
| `EnableIPChangeNotification` | bool | IP 变更通知 |
| `IPChangeNotificationTag` | string | IP 变更通知组 |
| `Cover` | uint8 | 覆盖范围 |
| `IgnoredIPNotification` | string | 忽略 IP 通知的服务器 |
| `Location` | string | 时区 |
| `MaxTCPPingValue` | int32 | TCP Ping 最大值 |
| `AvgPingCount` | int | 平均 Ping 次数 |
| `DNSServers` | string | DNS 服务器 |
| `EnableOfflineHistory` | bool | 是否启用离线历史 |
| `OfflineThresholdSeconds` | uint64 | 离线阈值秒数 |
| `OfflineCheckIntervalSeconds` | uint64 | 离线检查间隔 |
| `OfflineMergeGapSeconds` | uint64 | 离线合并间隔（1~3600，默认 10）：相邻两次离线之间的在线时间 ≤ 该值时合并为一次 |
| `OfflineHistoryRetentionDays` | uint64 | 离线历史保留天数 |
| `EnableOfflineNotification` | bool | 离线通知 |
| `EnableRecoveryNotification` | bool | 恢复通知 |
| `ShowAvailabilityToGuest` | bool | 是否向前台展示可用性 |

---

## 服务监控数据结构

服务监控页通过 `.Services` 接收所有需要在页面展示的服务监控项。

### `ServiceItemResponse`

| 字段 | 类型 | 说明 |
|------|------|------|
| `Monitor` | `*Monitor` | 监控任务定义 |
| `CurrentUp` | uint64 | 当天（当前周期）正常次数 |
| `CurrentDown` | uint64 | 当天（当前周期）异常次数 |
| `TotalUp` | uint64 | 最近 30 天累计正常次数 |
| `TotalDown` | uint64 | 最近 30 天累计异常次数 |
| `Delay` | `[30]float32` | 最近 30 天每天平均延迟（毫秒） |
| `Up` | `[30]int` | 最近 30 天每天正常次数 |
| `Down` | `[30]int` | 最近 30 天每天异常次数 |

数组下标 `0` 表示最近一天，`29` 表示 30 天前。`TotalUptime()` 方法（模板中可用 `float32f $service.TotalUptime`）返回 `TotalUp / (TotalUp + TotalDown) * 100`。

### `Monitor`

| 字段 | 类型 | 说明 |
|------|------|------|
| `ID` | uint64 | 监控任务 ID |
| `Name` | string | 监控名称 |
| `Type` | uint8 | 任务类型：`1` HTTP、`2` ICMP Ping、`3` TCP Ping |
| `Target` | string | 监控目标（URL / IP / 域名） |
| `Duration` | uint64 | 检测间隔（秒），默认 30 |
| `Notify` | bool | 是否开启通知 |
| `NotificationTag` | string | 通知组标签 |
| `Cover` | uint8 | 覆盖范围：`0` 覆盖全部服务器，`1` 忽略全部服务器 |
| `EnableTriggerTask` | bool | 失败/恢复时是否触发任务 |
| `EnableShowInService` | bool | 是否在前台服务监控页展示 |
| `MinLatency` | float32 | 延迟告警下限（毫秒） |
| `MaxLatency` | float32 | 延迟告警上限（毫秒） |
| `LatencyNotify` | bool | 是否开启延迟告警 |

### `CycleTransferStats`

周期流量统计， keyed by alert rule ID。

| 字段 | 类型 | 说明 |
|------|------|------|
| `Name` | string | 规则名称 |
| `From` | time.Time | 统计开始时间 |
| `To` | time.Time | 统计结束时间 |
| `Max` | uint64 | 流量上限（字节） |
| `Min` | uint64 | 流量下限（字节） |
| `ServerName` | `map[uint64]string` | 参与统计的服务器 ID → 名称 |
| `Transfer` | `map[uint64]uint64` | 服务器 ID → 当前已用流量（字节） |
| `NextUpdate` | `map[uint64]time.Time` | 服务器 ID → 下次更新时间 |

### 服务监控页完整示例

```json
{
  "Services": {
    "1": {
      "Monitor": {
        "ID": 1,
        "Name": "Google DNS",
        "Type": 2,
        "Target": "8.8.8.8",
        "Duration": 30,
        "EnableShowInService": true
      },
      "CurrentUp": 2880,
      "CurrentDown": 0,
      "TotalUp": 86400,
      "TotalDown": 120,
      "Delay": [12.5, 13.0, 11.8, null, 12.1],
      "Up": [2880, 2879, 2880, 0, 2880],
      "Down": [0, 1, 0, 0, 0]
    }
  },
  "CycleTransferStats": {
    "1": {
      "Name": "月流量 1TB",
      "From": "2024-06-01T00:00:00Z",
      "To": "2024-07-01T00:00:00Z",
      "Max": 1099511627776,
      "Min": 0,
      "ServerName": { "1": "HK-01" },
      "Transfer": { "1": 536870912000 },
      "NextUpdate": { "1": "2024-06-20T13:00:00Z" }
    }
  }
}
```

---

## 网络监控数据结构

网络监控页通过 `.MonitorInfos` 接收指定服务器的监控历史数据，用于绘制延迟趋势图。

### `MonitorInfo`

| 字段 | 类型 | 说明 |
|------|------|------|
| `monitor_id` | uint64 | 监控任务 ID |
| `server_id` | uint64 | 服务器 ID |
| `monitor_name` | string | 监控任务名称 |
| `server_name` | string | 服务器名称 |
| `created_at` | []int64 | 数据点 UTC 时间戳（毫秒） |
| `avg_delay` | []float32 | 对应时间点的平均延迟（毫秒） |

### `MonitorInfoResponse`

| 字段 | 类型 | 说明 |
|------|------|------|
| `code` | int | 响应码，`0` 表示成功 |
| `message` | string | 响应消息 |
| `result` | `[]*MonitorInfo` | 监控历史数据数组 |

### 网络监控页示例

```javascript
const monitorInfos = JSON.parse('{{.MonitorInfos}}');
// monitorInfos.result: [{ monitor_id, server_id, monitor_name, server_name, created_at, avg_delay }, ...]
```

### 完整 JSON 示例

```json
{
  "code": 0,
  "message": "success",
  "result": [
    {
      "monitor_id": 1,
      "server_id": 1,
      "monitor_name": "Google DNS",
      "server_name": "HK-01",
      "created_at": [1718888400000, 1718888460000, 1718888520000],
      "avg_delay": [12.5, 13.0, 11.8]
    }
  ]
}
```

---

## 可用性数据结构

`/api/v1/server/availability` 返回服务器在指定天数内的可用性摘要。

### `ServerAvailability`

| 字段 | 类型 | 说明 |
|------|------|------|
| `server_id` | uint64 | 服务器 ID |
| `days` | int | 统计天数 |
| `offline_count` | int | 离线次数 |
| `total_offline_seconds` | uint64 | 累计离线秒数 |
| `longest_offline_seconds` | uint64 | 最长单次离线秒数 |
| `availability_percent` | float\|null | 可用率百分比，如 `99.95`；服务器从未上报过数据时为 `null`（前端应显示为空，而非 100%） |

### 接口说明

```
GET /api/v1/server/availability?id=1,2,3&days=30
```

| 参数 | 说明 |
|------|------|
| `id` | 服务器 ID，多个用逗号分隔；为空则返回所有可见服务器 |
| `days` | 统计天数，默认 30，最大 3660 |

> 需要后台开启 **ShowAvailabilityToGuest** 才会对游客返回数据，否则返回 `403`。

### 完整 JSON 示例

```json
{
  "code": 200,
  "result": [
    {
      "server_id": 1,
      "days": 30,
      "offline_count": 2,
      "total_offline_seconds": 300,
      "longest_offline_seconds": 180,
      "availability_percent": 99.95
    }
  ]
}
```

### 常用模板函数

| 函数 | 示例 | 说明 |
|------|------|------|
| `tr` | `{{tr "ServerIsOffline"}}` | 国际化翻译 |
| `json` | `{{.Servers}}` | 序列化为 `template.JS` |
| `safe` | `{{.CustomCode \| safe}}` | 标记为安全 HTML |
| `bf` | `{{$transfer \| bf}}` | 字节格式化 |
| `tf` / `sft` | `{{$time \| tf}}` | 时间格式化 |
| `div` / `add` | `{{div $a $b}}` | 整数运算 |
| `float32f` | `{{$value \| float32f}}` | 浮点格式化 |
| `className` | `{{className $ratio}}` | 根据可用率返回 CSS 类名 |
| `statusName` | `{{statusName $ratio}}` | 根据可用率返回状态文本 |
| `dayBefore` | `{{dayBefore $i}}` | 返回最近 30 天对应日期标签 |

---

## 实时数据与接口

### WebSocket `/ws`

前台主题最重要的实时数据源。连接建立后，服务端每 2 秒推送一次所有可见服务器的状态。

#### WebSocket 返回结构

| 字段 | 类型 | 说明 |
|------|------|------|
| `now` | int64 | 服务端当前时间戳（毫秒） |
| `servers` | `[]*Server` | 服务器数组，字段说明见 [Servers 数据结构](#servers-数据结构) |

```javascript
const protocol = window.location.protocol === 'https:' ? 'wss' : 'ws';
const ws = new WebSocket(`${protocol}://${window.location.host}/ws`);

ws.onmessage = (event) => {
    const data = JSON.parse(event.data);
    // data.now: 服务端当前时间戳（毫秒）
    // data.servers: 服务器数组
    updateServers(data.servers);
};

ws.onclose = () => {
    setTimeout(() => connect(), 3000); // 自动重连
};
```

> WebSocket 连接受查看密码保护；如果站点启用了查看密码，需先通过 `/view-password` 验证。

### 监控历史图表：`GET /api/v1/monitor/:id`

获取指定服务器最近 24 小时的监控历史，返回结构与 [网络监控数据结构](#网络监控数据结构) 中的 `MonitorInfoResponse` 一致。

```javascript
fetch(`/api/v1/monitor/${serverId}`)
  .then(r => r.json())
  .then(data => {
      // data.result: MonitorInfo 数组
      renderChart(data.result);
  });
```

### 可用性数据：`GET /api/v1/server/availability?id=...`

返回服务器可用性摘要，返回结构与 [可用性数据结构](#可用性数据结构) 中的 `ServerAvailability` 一致。需要后台开启 **ShowAvailabilityToGuest** 才会对游客返回数据。

### 服务器列表 API

详见 [API 文档](api.md)。

---

## 前端框架与库

### 推荐技术栈

- **Vue 2.6.14**：大多数内置前台主题使用 Vue 2 进行数据绑定。
- **ECharts 5.5.0**：监控图表。
- **jQuery 3.7.1**：DOM 操作与 AJAX。
- **Semantic UI 2.4.0**：默认主题 UI 框架。
- **Bootstrap Icons / Font Logos / Flag Icons**：图标与系统 Logo。

### Vue 定界符

由于 Go 模板也使用 `{{ }}`，前端 Vue 必须使用自定义定界符。项目约定：

```javascript
new Vue({
    el: '#app',
    delimiters: ['@#', '#@'],
    // ...
});
```

模板中：

```html
<!-- Go 模板变量 -->
<title>{{.Title}}</title>

<!-- Vue 变量 -->
<span>@#server.Name#@</span>
```

### 公共 Mixin

`theme-default` 提供了 `mixinsVue`，包含主题切换、Cookie 读写、移动端检测、注销等方法。主题开发者可参考或复用：

```html
<script src="/static/theme-default/js/mixin.js"></script>
<script>
new Vue({
    el: '#app',
    delimiters: ['@#', '#@'],
    mixins: [mixinsVue],
    // ...
});
</script>
```

---

## 国际化

### 模板中翻译

```html
<h1>{{tr "Home"}}</h1>
```

翻译文件位于 `resource/l10n/{zh-CN,zh-TW,en-US,es-ES}.toml`。

### JS 中翻译

后台页面通过 `.LANG` 获取翻译映射。前台主题通常直接在后端模板中使用 `{{tr "..."}}` 生成静态文本；动态生成的文本可预先在模板中生成好，或自行维护前端字典。

### 新增翻译

如果主题需要新的翻译条目，需向 `resource/l10n/*.toml` 添加对应键值，并在模板中使用 `{{tr "YourKey"}}`。

---

## 自定义主题与本地覆盖

### theme-custom

用户可创建 `resource/template/theme-custom/home.html` 作为本地自定义主题，无需 `theme.json`。

启用方式：

```yaml
site:
  theme: "custom"
```

### static/custom

用户可将静态资源放在 `resource/static/custom/`，运行时会优先读取该目录，覆盖嵌入资源。

例如：

```
resource/static/custom/theme-default/css/main.css
```

会覆盖内置 `theme-default` 的同名 CSS。

---

## 主题切换机制

### 前台切换

如果未禁用，游客可通过菜单切换主题。切换逻辑：

1. 写入 Cookie `preferred_theme=<key>`。
2. 刷新页面。
3. 中间件 `PreferredTheme` 读取 Cookie，若主题存在则使用；否则回退到 `config.yaml` 中配置的 `site.theme`。

### 禁用前台切换

后台设置中开启 **前台禁用切换主题**，或配置：

```yaml
disableswitchtemplateinfrontend: true
```

此时游客只能看到默认主题，无法切换。

### 模板名称解析

控制器调用：

```go
c.HTML(http.StatusOK, mygin.GetPreferredTheme(c, "/home"), data)
```

若 Cookie 指定了 `server-status`，则解析为 `theme-server-status/home`；否则使用配置主题。

---

## 开发调试与发布

### 开发环境

1. 克隆项目源码。
2. 在 `resource/template/theme-<key>/` 创建主题文件。
3. 在 `resource/static/theme-<key>/` 创建静态资源。
4. 修改 `data/config.yaml`：

```yaml
site:
  theme: "<key>"
```

5. 编译并运行 Dashboard：

```bash
go build -o santaizi-dashboard ./cmd/dashboard
./santaizi-dashboard
```

### 调试技巧

- 模板修改后需要**重启 Dashboard** 才能生效（嵌入资源在编译时固定）。
- 若使用 `theme-custom` 或 `static/custom`，文件放在本地文件系统，修改后刷新页面即可生效。
- 查看日志可发现模板解析错误：`SANTAIZI>> Error parsing templates ...`

### 发布主题

建议以独立仓库或压缩包形式发布，目录结构如下：

```
santaizi-theme-mytheme/
├── resource/
│   ├── template/theme-mytheme/
│   │   ├── theme.json
│   │   ├── home.html
│   │   ├── header.html
│   │   ├── footer.html
│   │   ├── menu.html
│   │   ├── service.html
│   │   ├── network.html
│   │   └── viewpassword.html
│   └── static/theme-mytheme/
│       ├── css/
│       └── js/
└── README.md
```

用户安装时，将 `resource/` 下的内容合并到 Dashboard 的 `resource/` 目录，重启即可。

---

## 最小可运行示例

以下是一个最简前台主题 `theme-minimal` 的完整文件。

### `resource/template/theme-minimal/theme.json`

```json
{
  "name": "Minimal"
}
```

### `resource/template/theme-minimal/header.html`

```html
{{define "theme-minimal/header"}}
<!DOCTYPE html>
<html lang="{{.Conf.Language}}">
<head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>{{.Title}}</title>
    <style>
        body { font-family: system-ui, sans-serif; margin: 2rem; }
        .offline { color: #999; }
        .online { color: #2e7d32; }
    </style>
    {{template "common/csrf" .}}
</head>
<body>
{{end}}
```

### `resource/template/theme-minimal/footer.html`

```html
{{define "theme-minimal/footer"}}
<footer>
    <p>&copy; {{.Conf.Site.Brand}} | Santaizi {{.Version}}</p>
</footer>
</body>
</html>
{{end}}
```

### `resource/template/theme-minimal/menu.html`

```html
{{define "theme-minimal/menu"}}
<nav>
    <a href="/">{{tr "Home"}}</a>
    <a href="/service">{{tr "Services"}}</a>
    <a href="/network">{{tr "NetworkSpiter"}}</a>
</nav>
{{end}}
```

### `resource/template/theme-minimal/home.html`

```html
{{define "theme-minimal/home"}}
{{template "theme-minimal/header" .}}
{{if .CustomCode}}{{.CustomCode | safe}}{{end}}
{{template "theme-minimal/menu" .}}

<main id="app">
    <h1>{{.Conf.Site.Brand}}</h1>
    <ul>
        <li v-for="server in servers" :key="server.ID" :class="server.live ? 'online' : 'offline'">
            @#server.Name#@ — @#server.live ? '在线' : '离线'#@
        </li>
    </ul>
</main>

{{template "theme-minimal/footer" .}}
<script src="/static/unpkg/vue@2.6.14/dist/vue.min.js"></script>
<script>
    const initial = JSON.parse('{{.Servers}}');
    new Vue({
        el: '#app',
        delimiters: ['@#', '#@'],
        data: {
            servers: initial.servers.map(s => ({ ...s, live: false }))
        },
        created() {
            this.connectWS();
        },
        methods: {
            connectWS() {
                const protocol = window.location.protocol === 'https:' ? 'wss' : 'ws';
                const ws = new WebSocket(`${protocol}://${window.location.host}/ws`);
                ws.onmessage = (event) => {
                    const data = JSON.parse(event.data);
                    this.servers = data.servers.map(s => {
                        const live = s.Host && (data.now - new Date(s.LastActive).getTime() <= 10000);
                        return { ...s, live };
                    });
                };
                ws.onclose = () => setTimeout(() => this.connectWS(), 3000);
            }
        }
    });
</script>
{{end}}
```

### 启用

```yaml
site:
  theme: "minimal"
```

---

## 常见问题

### Q1：主题修改后没有生效？

嵌入到二进制中的主题需要重新编译并重启 Dashboard。只有 `theme-custom` 和 `static/custom` 下的本地文件会即时生效。

### Q2：如何调试模板解析错误？

启动时查看控制台日志，错误信息类似：

```
SANTAIZI>> Error parsing templates theme-mytheme: ...
```

### Q3：Vue 与 Go 模板冲突？

前台主题必须使用自定义定界符，例如 `delimiters: ['@#', '#@']`，避免与 Go 的 `{{ }}` 冲突。

### Q4：如何让主题出现在前台切换菜单？

提供有效的 `theme.json` 并确保 `home.html` 存在，主题会自动注册到 `.Themes`。

### Q5：能否使用 Vue 3 或 React？

可以。Santaizi 不限制前端框架，只要最终输出 HTML/CSS/JS 即可。但内置主题和公共库以 Vue 2 / jQuery 为主，使用其他框架需要自行打包或编写原生代码。

### Q6：后台主题和前台主题开发有什么区别？

- 后台主题目录为 `dashboard-<key>/`，模板定义名为 `dashboard-<key>/<page>`。
- 后台主题不需要 `theme.json`。
- 后台页面变量更丰富，通常直接使用完整 `.Conf` 和各种管理数据列表。
- 后台主题默认使用 Vue 3 + Element Plus。

---

## 相关链接

- [主题与自定义（用户指南）](themes.md)
- [API 文档](api.md)
- [配置参考](configuration.md)
- 项目源码：`https://github.com/hi2shark/santaizi-dashboard`
