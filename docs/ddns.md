# 动态 DNS（DDNS）

DDNS 功能可以根据 Agent 上报的 IP 自动更新域名解析记录。

## 进入方式

**动态 DNS** 页面（`/ddns`）。

## 创建 DDNS

1. 点击 **添加 DDNS**。
2. 填写：
   - **名称**：DDNS 配置名称
   - **提供商**：
     - `Dummy`：仅记录日志，不实际操作
     - `Cloudflare`：通过 API Token 管理解析
     - `腾讯云 DNS`：通过 SecretId/SecretKey 管理解析
     - `Webhook`：调用自定义接口
   - **启用 IPv4 / IPv6**：是否更新 A 记录 / AAAA 记录
   - **域名**：需要更新的域名，多个用逗号分隔
   - **重试次数**：更新失败时的重试次数（1-10）
3. 保存。

## 在服务器上启用 DDNS

创建 DDNS 配置后，在 **服务器器** 页面编辑服务器，勾选 **启用 DDNS**，并选择对应的 DDNS 配置。

Agent 上报 IP 发生变化时，Dashboard 会自动调用 DDNS 提供商更新解析。

## Cloudflare 示例

- **API Token**：在 Cloudflare 控制台创建，权限：`Zone:DNS:Edit`
- **域名**：`home.example.com`
- 确保 Token 有对应 Zone 的访问权限

## Webhook 示例

配置 Webhook URL，Dashboard 会在 IP 变化时 POST 当前 IP 信息到该地址。请求体格式取决于具体实现。

## 相关配置

- `enableipchangenotification`：IP 变动时是否发送通知
- `ipchangenotificationtag`：IP 变动通知使用的通知组
- `cover` 和 `ignoredipnotification`：IP 变动通知的覆盖范围
