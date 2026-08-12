# santaizi-dashboard  

三太子监控的 Dashboard 与 Collector。管理后台、公开 ServerStatus 和 API 文档均为独立 Vue 3 应用，默认嵌入 Go 二进制，也可在同域反向代理下外置部署。HTTP API v2 以 OpenAPI 3.0.3 为唯一契约。

- 管理后台：`/admin/`
- ServerStatus：`/`
- 交互式 API 文档：`/docs/api/`
- OpenAPI：`/openapi/v2.yaml`、`/openapi/v2.json`

【版本声明 / 版权】  
本项目基于 [哪吒监控 Nezha Monitoring](https://github.com/naiba/nezha) 衍生修改，原作者版权保留（Apache-2.0，`Copyright 2020 naiba`）。详见 [`LICENSE`](./LICENSE) 与 [`NOTICE`](./NOTICE)。  
产品品牌为 **三太子 / Santaizi**；Dashboard 与探针须成对升级。
本仓库仅为个人使用方便进行调整修改。

## 安全要求（必读）

**强烈要求：不要把面板 Web 端口直接暴露在公网。**

管理后台（`/admin/`）、OAuth 回调、API 与 WebSocket 一旦可被任意访问，即构成高风险控制面。部署时**必须**将面板置于可信访问控制之后，例如：

- [Cloudflare Zero Trust](https://developers.cloudflare.com/cloudflare-one/)（Access / Tunnel 等）
- 等价方案：自建 SSO + 反向代理鉴权、WireGuard / Tailscale 等私有网络、仅内网可达 + VPN

建议做法：

1. **Web 面**：仅通过 Zero Trust / 私有网络访问；公网侧不开放裸 HTTP(S) 到面板端口。
2. **gRPC（探针上报，默认 `5555`）**：可按需要放行给探针，但不要与未受保护的管理 Web 混在同一公网入口。
3. 仍须配置 OAuth2 管理员白名单；访问控制是**额外**防护，不能替代登录鉴权。

未落实上述防护即公网裸奔面板，后果自负。本项目不以「直接对公网开放管理面」为推荐部署方式。

## Docker Compose 部署 Santaizi Dashboard

### 方式一：一键安装脚本（推荐）

支持交互式填写配置，未安装 Docker 时会询问是否自动安装。

一键运行：

```bash
sh -c "$(curl -fsSL https://raw.githubusercontent.com/hi2shark/santaizi-dashboard/master/script/install_dashboard.sh)"
```

如果当前不是 root，可任选一种系统已有的提权方式后再执行同一条命令：

```bash
sudo -i
# 或 doas sh
# 或 su -
```

然后运行：

```bash
sh -c "$(curl -fsSL https://raw.githubusercontent.com/hi2shark/santaizi-dashboard/master/script/install_dashboard.sh)"
```

也可以先下载脚本再执行：

```bash
curl -fsSL https://raw.githubusercontent.com/hi2shark/santaizi-dashboard/master/script/install_dashboard.sh -o install_dashboard.sh
sh install_dashboard.sh
```

脚本会引导填写工作目录、Web 端口、gRPC 端口、OAuth2 登录信息与站点标题，然后自动生成 `docker-compose.yml` 和 `config/dashboard.yaml` 并启动 Dashboard。公开端固定使用 ServerStatus，不再提供模板主题切换。

### 方式二：手动部署

#### 1. 创建工作目录

```bash
mkdir -p /opt/santaizi && cd /opt/santaizi
```

#### 2. 创建 `docker-compose.yml`

```yaml
services:
  santaizi-dashboard:
    image: ghcr.io/hi2shark/santaizi-dashboard:latest
    container_name: santaizi-dashboard
    restart: unless-stopped
    ports:
      - "${SANTAIZI_PORT:-80}:80"
      - "5555:5555"
    volumes:
      - /etc/timezone:/etc/timezone:ro
      - /etc/localtime:/etc/localtime:ro
      - ./data:/var/lib/santaizi-dashboard
      - ./config/dashboard.yaml:/etc/santaizi/dashboard.yaml:ro
    environment:
      - TZ=Asia/Shanghai
```

> 说明：
> - `80` 为面板 Web 端口，可通过 `SANTAIZI_PORT` 环境变量改为其它端口，如 `SANTAIZI_PORT=8080`。
> - `5555` 为 Agent 上报用的 gRPC 端口。
>
> 映射右侧的容器端口（`80` / `5555`）必须与容器内 `/etc/santaizi/dashboard.yaml` 里的 `httpport` / `grpcport` 保持一致。默认已对应，如需修改请同时调整配置文件。

#### 3. 准备 `config/dashboard.yaml`

```bash
mkdir -p config data
curl -fsSL https://raw.githubusercontent.com/hi2shark/santaizi-dashboard/master/script/config.yaml -o config/dashboard.yaml
```

编辑 `config/dashboard.yaml`，至少填写以下字段：

```yaml
httpport: 80
grpcport: 5555

site:
  brand: "三太子监控"
  primarycolor: "#2563eb"

oauth2:
  type: "github"
  admin: "your-github-username"
  clientid: "xxx"
  clientsecret: "xxx"
  endpoint: ""
```

#### 4. 启动 Dashboard

```bash
docker compose up -d
```

启动后**不要**用公网 IP 直接打开面板。请先接入 Cloudflare Zero Trust（或等价私有网络 / SSO 反向代理），再通过受保护入口登录 OAuth2。本地调试可用 `http://127.0.0.1:<SANTAIZI_PORT>`。

### 开放防火墙端口

- **Web 端口**（默认 `80` / `SANTAIZI_PORT`）：**强烈要求不对公网直接放行**；仅对本机、内网或 Zero Trust / Tunnel 出口开放。
- **gRPC 端口**（`5555`）：按探针所在网络需要放行；与未受保护的管理 Web 入口分离。

### 更新 Dashboard

```bash
cd /opt/santaizi
docker compose pull
docker compose up -d
```

### 安装 Agent

进入 Dashboard 后台 → 服务器 → 添加服务器，保存后在服务器卡片上点击对应平台的一键安装按钮，复制命令到被监控服务器执行即可。

默认 Agent 下载源为 `hi2shark/santaizi-agent`，可通过环境变量 `SANTAIZI_AGENT_REPO` 覆盖：

```bash
SANTAIZI_AGENT_REPO=your-repo/agent curl -fSL ... | bash -s -- install_agent ...
```

### 常见问题

- **面板能否直接挂公网？**：**强烈不建议。** 须置于 Cloudflare Zero Trust 或等价可信访问控制之后，见上文「安全要求」。
- **Agent 无法连接面板**：检查服务器防火墙是否放行 `5555` 端口，以及 `grpcport` 是否配置为 `5555`。
- **一键安装脚本拉取失败**：可在 `config/dashboard.yaml` 的 `installscript` 段替换为可访问的脚本地址。
- **登录后没有管理员权限**：确认 `oauth2.admin` 填写的是 OAuth2 平台返回的 **用户名/ID**。

可靠遥测、Collector 部署、保留策略和升级顺序见 [可靠遥测运维指南](docs/reliable-telemetry.md)。本版本只接受全新数据库；若数据库非空且没有 `schema_migrations`，Dashboard 会拒绝启动并保留原文件供诊断。
