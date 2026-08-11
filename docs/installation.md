# 安装指南

## Dashboard 安装

### 方式一：一键安装脚本（推荐）

```bash
sh -c "$(curl -fsSL https://raw.githubusercontent.com/hi2shark/santaizi-dashboard/master/script/install_dashboard.sh)"
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
mkdir -p ~/santaizi/data && cd ~/santaizi
```

2. 编写 `docker-compose.yml`：

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
CGO_ENABLED=1 go build -o santaizi-dashboard ./cmd/dashboard/main.go

# 运行
./santaizi-dashboard --config data/config.yaml --db data/sqlite.db
```

CLI 参数：

| 参数 | 说明 |
|------|------|
| `-v` | 查看版本 |
| `-c data/config.yaml` | 指定配置文件，默认 `data/config.yaml` |
| `--db data/sqlite.db` | 指定 SQLite 路径，默认 `data/sqlite.db` |

---

## Agent 安装

Agent 默认从 `hi2shark/santaizi-agent` 仓库下载，可通过环境变量 `SANTAIZI_AGENT_REPO` 覆盖。

### Linux

```bash
curl -fSL https://raw.githubusercontent.com/hi2shark/santaizi-dashboard/master/script/install_agent.sh | bash -s -- install_agent <面板地址> <端口> <密钥>
```

英文版脚本：

```bash
curl -fSL https://raw.githubusercontent.com/hi2shark/santaizi-dashboard/master/script/install_agent_en.sh | bash -s -- install_agent <面板地址> <端口> <密钥>
```

安装路径：`/opt/santaizi/agent`

### Windows

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -Command "& ([ScriptBlock]::Create((Invoke-WebRequest 'https://raw.githubusercontent.com/hi2shark/santaizi-dashboard/master/script/install.ps1' -UseBasicParsing).Content)) '<面板地址>:<端口>' '<密钥>'"
```

安装路径：`C:\santaizi`

### macOS

```bash
curl -fSL https://raw.githubusercontent.com/hi2shark/santaizi-dashboard/master/script/install.command | sudo bash -s -- install_agent <面板地址> <端口> <密钥>
```

安装路径：`/opt/santaizi/agent`

### 参数说明

| 参数 | 示例 | 说明 |
|------|------|------|
| 面板地址 | `10.0.0.10` 或 `santaizi.example.com` | Dashboard 的 IP 或域名 |
| 端口 | `5555` | `config.yaml` 中的 `grpcport` |
| 密钥 | `abcdef123456` | 服务器详情中的 Secret |

### 管理 Agent

Linux / macOS 使用 systemd 服务：

```bash
# 查看状态
systemctl status santaizi-agent

# 重启
systemctl restart santaizi-agent

# 查看日志
journalctl -u santaizi-agent -f
```

Windows 使用服务管理器查看 `Santaizi Agent` 服务。

---

## 更新

### Dashboard

```bash
cd ~/santaizi
docker compose pull
docker compose up -d
```

### Agent

在 Dashboard 的 **服务器器** 页面点击 **强制更新**，或登录目标机器执行：

```bash
systemctl restart santaizi-agent
```

Agent 服务启动时会自动检查并下载最新版本。

---

## 卸载

### Dashboard

```bash
cd ~/santaizi
docker compose down -v
rm -rf ~/santaizi
```

### Agent

```bash
systemctl stop santaizi-agent
systemctl disable santaizi-agent
rm -rf /opt/santaizi/agent
rm -f /etc/systemd/system/santaizi-agent.service
systemctl daemon-reload
```
