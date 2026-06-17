# nezha-next  

## 版本区别
 - v0-final基础上，让AI优化后的版本  
 - 来自ipinfo.io的GeoIP数据库只会Release时更新，因此，这里的v0-next会每周自动Release一次，并使用最新的GeoIP数据库。
 - 关于unpkg.com近期出现了不稳定的情况，因此，这里的v0-next下载了对应前端资源到项目中（目前覆盖控制台、默认主题、ServerStatus主题）。
 - 特别注意，本仓库只提供了docker镜像，不提供其它分发系统的构建内容，因为没有时间DEBUG。  

【版本声明】  
Nezha面板所有版权归属原作者，本仓库仅为自己个人使用方便，进行调整修改。  
[README](./README-OLD.md)

## Docker Compose 部署 Nezha Dashboard

### 方式一：一键安装脚本（推荐）

支持交互式填写配置，未安装 Docker 时会询问是否自动安装。

```bash
curl -fsSL https://raw.githubusercontent.com/hi2shark/nezha-next/master/script/install_dashboard.sh | sudo bash
```

脚本会引导填写：工作目录、Web 端口、gRPC 端口、OAuth2 登录信息、站点标题与主题，然后自动生成 `docker-compose.yml` 和 `data/config.yaml` 并启动 Dashboard。

### 方式二：手动部署

#### 1. 创建工作目录

```bash
mkdir -p /opt/nezha && cd /opt/nezha
```

#### 2. 创建 `docker-compose.yml`

```yaml
services:
  nezha-dashboard:
    image: ghcr.io/hi2shark/nezha-next-dashboard:latest
    container_name: nezha-dashboard
    restart: unless-stopped
    ports:
      - "${NEZHA_PORT:-80}:80"
      - "5555:5555"
    volumes:
      - /etc/timezone:/etc/timezone:ro
      - /etc/localtime:/etc/localtime:ro
      - ./data:/dashboard/data
    environment:
      - TZ=Asia/Shanghai
```

> 说明：
> - `80` 为面板 Web 端口，可通过 `NEZHA_PORT` 环境变量改为其它端口，如 `NEZHA_PORT=8080`。
> - `5555` 为 Agent 上报用的 gRPC 端口。
>
> 映射右侧的容器端口（`80` / `5555`）必须与容器内 `data/config.yaml` 里的 `httpport` / `grpcport` 保持一致。默认已对应，如需修改请同时调整配置文件。

#### 3. 准备 `data/config.yaml`

```bash
mkdir -p data
curl -fsSL https://raw.githubusercontent.com/hi2shark/nezha-next/master/script/config.yaml -o data/config.yaml
```

编辑 `data/config.yaml`，至少填写以下字段：

```yaml
httpport: 80
grpcport: 5555

site:
  brand: "哪吒监控"
  theme: "default"

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

启动后访问 `http://<服务器IP>:<NEZHA_PORT>`，使用 OAuth2 登录。

### 开放防火墙端口

确保服务器防火墙放行以下端口：

- Web 端口：默认 `80`（或你自定义的 `NEZHA_PORT`）
- gRPC 端口：`5555`

### 更新 Dashboard

```bash
cd /opt/nezha
docker compose pull
docker compose up -d
```

### 安装 Agent

进入 Dashboard 后台 → 服务器 → 添加服务器，保存后在服务器卡片上点击对应平台的一键安装按钮，复制命令到被监控服务器执行即可。

默认 Agent 下载源为 `hi2shark/agent`，可通过环境变量 `NEZHA_AGENT_REPO` 覆盖：

```bash
NEZHA_AGENT_REPO=your-repo/agent curl -fSL ... | bash -s -- install_agent ...
```

### 常见问题

- **Agent 无法连接面板**：检查服务器防火墙是否放行 `5555` 端口，以及 `grpcport` 是否配置为 `5555`。
- **一键安装脚本拉取失败**：可在 `data/config.yaml` 的 `installscript` 段替换为可访问的脚本地址。
- **登录后没有管理员权限**：确认 `oauth2.admin` 填写的是 OAuth2 平台返回的 **用户名/ID**。
