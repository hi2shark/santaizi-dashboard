# 通知方式

进入 **通知方式** 页面（`/notification`）可以配置 Webhook 通知通道，并在下方管理告警规则。

## 创建通知方式

1. 点击 **添加通知方式**。
2. 填写：
   - **名称**：通知方式名称，如“企业微信机器人”
   - **URL**：Webhook 地址
   - **请求方式**：`GET` 或 `POST`
   - **请求类型**：`JSON` 或 `Form`
   - **请求体**：支持变量占位符的模板
   - **SSL 校验开关**：是否校验 HTTPS 证书
   - **跳过测试**：保存时不发送测试消息
3. 点击 **测试** 可以发送一条测试消息验证配置。

## 变量占位符

在 URL 和请求体中可以使用以下变量：

| 占位符 | 说明 |
|--------|------|
| `#SANTAIZI#` | 消息正文 |
| `#DATETIME#` | 当前时间 |
| `#SERVER.NAME#` | 服务器名称 |
| `#SERVER.IP#` | 服务器 IP（可能脱敏） |
| `#SERVER.IPV4#` | IPv4 地址 |
| `#SERVER.IPV6#` | IPv6 地址 |
| `#SERVER.CPU#` | CPU 占用率 |
| `#SERVER.MEM#` | 内存占用率 |
| `#SERVER.SWAP#` | Swap 占用率 |
| `#SERVER.DISK#` | 磁盘占用率 |
| `#SERVER.NETINSPEED#` | 入网速度 |
| `#SERVER.NETOUTSPEED#` | 出网速度 |
| `#SERVER.TRANSFERIN#` | 入站流量 |
| `#SERVER.TRANSFEROUT#` | 出站流量 |
| `#SERVER.LOAD1#` / `#SERVER.LOAD5#` / `#SERVER.LOAD15#` | 系统负载 |
| `#SERVER.ONLINE#` | 在线状态 |

> 实际可用变量取决于通知触发场景和 Agent 上报字段。

## Webhook 示例

### 企业微信机器人（JSON）

**URL**：`https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=xxxxxxxx`

**请求方式**：`POST`

**请求类型**：`JSON`

**请求体**：

```json
{
  "msgtype": "text",
  "text": {
    "content": "#SANTAIZI#"
  }
}
```

### Bark（GET）

**URL**：`https://api.day.app/xxxxxxxx/#SANTAIZI#`

**请求方式**：`GET`

### Telegram Bot（POST Form）

**URL**：`https://api.telegram.org/bot<token>/sendMessage`

**请求方式**：`POST`

**请求类型**：`Form`

**请求体**：

```
chat_id=xxxxxx&text=#SANTAIZI#
```

## 通知组

创建通知方式后，在创建 **监控** 或 **告警规则** 时选择对应的通知方式即可。一个通知方式可以被多个监控/规则复用。

## IP 脱敏

默认情况下，通知中的服务器 IP 会经过脱敏处理。如果需要完整 IP，可以在 **设置** 中开启 **通知中 IP 不打码**（`enableplainipinnotification: true`）。
