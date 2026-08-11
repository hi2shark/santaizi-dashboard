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
- 站点标题与 ServerStatus 品牌外观

确认后会自动生成 `docker-compose.yml` 和 `config/dashboard.yaml`，并启动容器。

### 方式二：手动 Docker Compose

1. 创建工作目录：

```bash
mkdir -p ~/santaizi/data ~/santaizi/config && cd ~/santaizi
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
      - ./data:/var/lib/santaizi-dashboard
      - ./config/dashboard.yaml:/etc/santaizi/dashboard.yaml:ro
    environment:
      - TZ=Asia/Shanghai
```

3. 准备 `config/dashboard.yaml`，参考 [配置参考](configuration.md)。

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
./santaizi-dashboard --config /etc/santaizi/dashboard.yaml --db /var/lib/santaizi-dashboard/sqlite.db
```

CLI 参数：

| 参数 | 说明 |
|------|------|
| `-v` | 查看版本 |
| `-c /etc/santaizi/dashboard.yaml` | 指定配置文件，默认 `/etc/santaizi/dashboard.yaml` |
| `--db /var/lib/santaizi-dashboard/sqlite.db` | 指定 SQLite 路径，默认 `/var/lib/santaizi-dashboard/sqlite.db` |

---

## Agent 安装

Agent 默认从 `hi2shark/santaizi-agent` 仓库下载，可通过环境变量 `SANTAIZI_AGENT_REPO` 覆盖。

### Linux

```bash
curl -fSL https://raw.githubusercontent.com/hi2shark/santaizi-dashboard/master/script/install_agent.sh | bash -s -- install_agent <面板地址> <端口> <密钥> --clean-install --confirm-clean-install
```

英文版脚本：

```bash
curl -fSL https://raw.githubusercontent.com/hi2shark/santaizi-dashboard/master/script/install_agent_en.sh | bash -s -- install_agent <面板地址> <端口> <密钥> --clean-install --confirm-clean-install
```

安装路径：`/opt/santaizi/agent`；默认配置 `/etc/santaizi/agent.yaml`；可靠遥测数据 `/var/lib/santaizi-agent/`

### Windows

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -Command "& ([ScriptBlock]::Create((Invoke-WebRequest 'https://raw.githubusercontent.com/hi2shark/santaizi-dashboard/master/script/install.ps1' -UseBasicParsing).Content)) -Server '<面板地址>:<端口>' -Key '<密钥>' -CleanInstall -ConfirmCleanInstall"
```

安装路径：`C:\santaizi`

### macOS

```bash
curl -fSL https://raw.githubusercontent.com/hi2shark/santaizi-dashboard/master/script/install.command | sudo bash -s -- install_agent <面板地址> <端口> <密钥> --clean-install --confirm-clean-install
```

安装路径：`/opt/santaizi/agent`

### 参数说明

| 参数 | 示例 | 说明 |
|------|------|------|
| 面板地址 | `10.0.0.10` 或 `santaizi.example.com` | Dashboard 的 IP 或域名 |
| 端口 | `5555` | `config.yaml` 中的 `grpcport` |
| 密钥 | `abcdef123456` | 服务器详情中的 Secret |

### 清洁安装

管理后台默认选择清洁安装。确认后命令会同时带上 `--clean-install --confirm-clean-install`（PowerShell 使用 `-CleanInstall -ConfirmCleanInstall`），安装器才会停止现有服务并删除 Agent 配置、节点身份、WAL 和程序目录。缺少确认标志时安装器会拒绝执行清理。

清洁安装会生成全新身份与节点绑定，不导入已有历史数据。

### 采集与能力参数

安装弹窗可选择 **标准**、**轻量**、**仅存活** 预设，也可以组合以下参数：

| 参数 | 作用 |
|------|------|
| `--disable-cpu` | 不采集 CPU 与负载 |
| `--disable-memory` | 不采集内存与 Swap |
| `--disable-disk` | 不采集磁盘指标 |
| `--disable-network` | 不采集网络速率与流量 |
| `--disable-connections` | 不采集 TCP/UDP 连接数 |
| `--disable-processes` | 不采集进程数 |
| `--temperature` | 启用温度采集 |
| `--gpu` | 启用 GPU 信息与使用率采集 |
| `--disable-host-info` | 不采集硬件与系统信息 |
| `--disable-ip-report` | 不查询或上报 IP/位置 |
| `--disable-http-probe` | 不参与 HTTP 服务探测 |
| `--disable-icmp-probe` | 不参与 ICMP 服务探测 |
| `--disable-tcp-probe` | 不参与 TCP 服务探测 |
| `--disable-nat` | 禁止建立 NAT 通道 |

心跳和可靠节点身份不可关闭。禁用指标会明确声明为“未采集”，不会伪装为数值零。

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

Dashboard 不提供远程或自动更新。升级时在目标机器重新执行管理后台生成的安装命令，或通过系统包管理流程替换 Agent；协议破坏性升级使用已确认的清洁安装。

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
rm -rf /var/lib/santaizi-agent
rm -f /etc/santaizi/agent.yaml
rm -f /etc/systemd/system/santaizi-agent.service
systemctl daemon-reload
```
