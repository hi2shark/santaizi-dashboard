# 安装指南

## Dashboard 安装

### 方式一：一键安装脚本（推荐）

```bash
sh -c "$(curl -fsSL https://raw.githubusercontent.com/hi2shark/nezha-next/master/script/install_dashboard.sh)"
```

脚本会交互式询问：

- 工作目录
- Web 端口、gRPC 端口
- OAuth2 类型及密钥
- 站点标题、主题

确认后会自动生成 `docker-compose.yml` 和 `data/config.yaml`，并启动容器。

### 方式二：手动 Docker Compose

1. 创建工作目录：

```bash
mkdir -p ~/nezha/data && cd ~/nezha
```

2. 编写 `docker-compose.yml`：

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

3. 准备 `data/config.yaml`，参考 [配置参考](configuration.md)。

4. 启动：

```bash
docker compose up -d
```

### 方式三：手动运行二进制

适用于开发或特殊环境。

```bash
# 下载 release 二进制或自行编译
CGO_ENABLED=1 go build -o nezha-dashboard ./cmd/dashboard/main.go

# 运行
./nezha-dashboard --config data/config.yaml --db data/sqlite.db
```

CLI 参数：

| 参数 | 说明 |
|------|------|
| `-v` | 查看版本 |
| `-c data/config.yaml` | 指定配置文件，默认 `data/config.yaml` |
| `--db data/sqlite.db` | 指定 SQLite 路径，默认 `data/sqlite.db` |

---

## Agent 安装

Agent 默认从 `hi2shark/agent` 仓库下载，可通过环境变量 `NEZHA_AGENT_REPO` 覆盖。

### Linux

```bash
curl -fSL https://raw.githubusercontent.com/hi2shark/nezha-next/master/script/install_agent.sh | bash -s -- install_agent <面板地址> <端口> <密钥>
```

英文版脚本：

```bash
curl -fSL https://raw.githubusercontent.com/hi2shark/nezha-next/master/script/install_agent_en.sh | bash -s -- install_agent <面板地址> <端口> <密钥>
```

安装路径：`/opt/nezha/agent`

### Windows

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -Command "& ([ScriptBlock]::Create((Invoke-WebRequest 'https://raw.githubusercontent.com/hi2shark/nezha-next/master/script/install.ps1' -UseBasicParsing).Content)) '<面板地址>:<端口>' '<密钥>'"
```

安装路径：`C:\nezha`

### macOS

```bash
curl -fSL https://raw.githubusercontent.com/hi2shark/nezha-next/master/script/install.command | sudo bash -s -- install_agent <面板地址> <端口> <密钥>
```

安装路径：`/opt/nezha/agent`

### 参数说明

| 参数 | 示例 | 说明 |
|------|------|------|
| 面板地址 | `10.0.0.10` 或 `nezha.example.com` | Dashboard 的 IP 或域名 |
| 端口 | `5555` | `config.yaml` 中的 `grpcport` |
| 密钥 | `abcdef123456` | 服务器详情中的 Secret |

### 管理 Agent

Linux / macOS 使用 systemd 服务：

```bash
# 查看状态
systemctl status nezha-agent

# 重启
systemctl restart nezha-agent

# 查看日志
journalctl -u nezha-agent -f
```

Windows 使用服务管理器查看 `Nezha Agent` 服务。

---

## 更新

### Dashboard

```bash
cd ~/nezha
docker compose pull
docker compose up -d
```

### Agent

在 Dashboard 的 **服务器器** 页面点击 **强制更新**，或登录目标机器执行：

```bash
systemctl restart nezha-agent
```

Agent 服务启动时会自动检查并下载最新版本。

---

## 卸载

### Dashboard

```bash
cd ~/nezha
docker compose down -v
rm -rf ~/nezha
```

### Agent

```bash
systemctl stop nezha-agent
systemctl disable nezha-agent
rm -rf /opt/nezha/agent
rm -f /etc/systemd/system/nezha-agent.service
systemctl daemon-reload
```
