# 常见问题

## Dashboard

### Q: Dashboard 启动失败，提示端口被占用

A: 检查 `httpport` 和 `grpcport` 是否被其他进程占用。使用：

```bash
ss -tlnp | grep -E ':80|:5555'
```

停止占用进程或更换端口。

### Q: 修改配置后没有生效

A: 大部分配置在 **设置** 页面保存后立即生效。涉及端口、TLS 等底层配置需要重启 Dashboard。

### Q: 如何查看 Dashboard 日志

Docker：

```bash
docker logs -f nezha-dashboard
```

二进制：

```bash
journalctl -u nezha-dashboard -f
```

### Q: 如何备份数据

备份以下文件：

```
data/sqlite.db
data/sqlite.db-shm
data/sqlite.db-wal
data/config.yaml
```

---

## Agent

### Q: Agent 安装后没有上线

A: 检查：

1. Dashboard 的 gRPC 端口是否可达
2. Agent 配置中的地址、端口、Secret 是否正确
3. 防火墙是否放行 gRPC 端口
4. Agent 服务是否已启动：`systemctl status nezha-agent`

### Q: Agent 日志在哪里

Linux / macOS：

```bash
journalctl -u nezha-agent -f
```

Windows：事件查看器或服务日志目录。

### Q: 如何更新 Agent

在 Dashboard 的 **服务器器** 页面点击 **强制更新**，或重启 Agent 服务使其自动升级。

### Q: Agent 是否需要 root

Agent 普通用户即可运行，但部分指标（如某些温度、进程信息）可能需要 root 权限才能采集。

---

## 监控与告警

### Q: 监控状态正常但没有收到通知

A: 检查：

1. 通知方式配置是否正确，可点击 **测试** 验证
2. 监控项是否选择了通知组
3. 告警规则是否配置了通知组
4. 通知中的 IP 是否被脱敏，但不影响送达

### Q: 离线历史里有很多“原因未知”

A: 原因判定依赖 Agent 上报的 `BootTime` 和 `Uptime`。如果 Agent 版本较旧或上报字段缺失，会显示为“原因未知”。不影响离线记录功能本身。

---

## 安全

### Q: 是否可以不使用 OAuth2

A: 生产环境强烈建议使用 OAuth2 / OIDC。`mock` 类型仅在本地开发 `debug: true` 时可用，不适合生产。

### Q: 如何保护 gRPC 端口

A: 建议：

1. 仅对 Agent 开放 gRPC 端口
2. 使用防火墙限制访问来源
3. 如需公网传输，开启 gRPC TLS（`tls: true`）并配置反向代理

### Q: 前台查看密码是什么

A: `site.viewpassword` 用于给未登录用户访问前台页面时加一道密码。登录后的管理员不受影响。

---

## 其他

### Q: 如何切换语言

A: 修改 `config.yaml` 中的 `language`，支持 `zh-CN`、`zh-TW`、`es-ES`、`en` 等。

### Q: 如何自定义 Agent 安装脚本源

A: 修改 `config.yaml` 中的 `installscript` 配置项，或设置环境变量 `NEZHA_SCRIPT_URL`。

### Q: 如何贡献主题

A: 参考 `resource/template/theme-default/` 和 `resource/static/theme-default/` 创建新的主题目录，然后提交 PR。
