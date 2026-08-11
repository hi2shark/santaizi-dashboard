# API 文档

## 基础信息

- Dashboard Base URL：`https://<你的 Dashboard 地址>`
- API v1 Base URL：`/api/v1`
- 会员 API Base URL：`/api`
- 统一响应格式：

```json
{
  "code": 200,
  "message": "",
  "result": { ... }
}
```

- `code` 为 `200` 表示成功
- 非 `200` 时 `message` 包含错误信息

---

## 认证方式

### 1. API Token

适用于 `/api/v1/...` 接口。

在 Dashboard 的 **API 管理** 页面（`/api`）生成 Token 后，在请求头中携带：

```http
Authorization: <your-token>
```

### 2. Cookie 登录

适用于 Dashboard 会员 API（`/api/...`）。用户登录后，浏览器会写入 Cookie，默认名称为 `santaizi-dashboard`。

第三方前端调用时需要：

```js
fetch('/api/offline-history?server_id=12', {
  credentials: 'include'
})
```

> 跨域时需要 Dashboard 开启 `Access-Control-Allow-Credentials: true`，且 `Access-Control-Allow-Origin` 不能为 `*`。

### 3. 查看密码

部分公开接口（如 `/api/v1/monitor/:id`）可能需要前台查看密码（`site.viewpassword`）。可在请求头中携带：

```http
X-View-Password: <password>
```

---

## 公开 API

以下接口无需 API Token 或登录即可访问公开数据，但如果站点开启了 `site.viewpassword`，可能需要先通过查看密码验证。

携带 API Token 或登录 Cookie 后：
- 会取消 `HideForGuest` 过滤，可查看全部服务器；
- 会额外返回敏感字段（如 `secret`、`note`、`ip`、Cron 命令、通知 URL、DDNS 密钥等）。

### 获取服务器列表

```http
GET /api/v1/server/list?tag=<分组名>
GET /api/v1/server?tag=<分组名>
```

**参数**：

| 参数 | 必填 | 说明 |
|------|------|------|
| `tag` | 否 | 按分组筛选 |

**响应示例**（无 Token）：

```json
{
  "code": 200,
  "message": "success",
  "result": [
    {
      "id": 12,
      "name": "HKG-DVPLUS",
      "tag": "香港",
      "public_note": "",
      "display_index": 0,
      "hide_for_guest": false
    }
  ]
}
```

**响应示例**（带 Token）：

```json
{
  "code": 200,
  "message": "success",
  "result": [
    {
      "id": 12,
      "name": "HKG-DVPLUS",
      "tag": "香港",
      "public_note": "",
      "display_index": 0,
      "hide_for_guest": false,
      "secret": "abcdef123456",
      "note": "管理员备注",
      "ip": "1.2.3.4",
      "ddns_profiles_raw": "[]"
    }
  ]
}
```

### 服务器详情

```http
GET /api/v1/server/details?id=<id1,id2,...>&tag=<分组名>
```

**参数**：

| 参数 | 必填 | 说明 |
|------|------|------|
| `id` | 否 | 服务器 ID，逗号分隔，优先级高于 `tag` |
| `tag` | 否 | 按分组筛选 |

**响应示例**：

```json
{
  "code": 200,
  "message": "success",
  "result": [
    {
      "id": 12,
      "name": "HKG-DVPLUS",
      "tag": "香港",
      "public_note": "",
      "host": { ... },
      "status": { ... }
    }
  ]
}
```

### 单台服务器

```http
GET /api/v1/server/:id
```

返回单台服务器的完整信息，结构与 `/api/v1/server/details` 中的元素一致。

### 服务监控

```http
GET /api/v1/service
```

返回所有 `EnableShowInService` 的服务监控汇总。

### 周期流量

```http
GET /api/v1/cycle-transfer
```

**响应示例**：

```json
{
  "success": true,
  "data": {
    "cycle_transfer_stats": {
      "1": {
        "name": "每月 1TB",
        "from": "2025-07-01T00:00:00Z",
        "to": "2025-08-01T00:00:00Z",
        "max": 1099511627776,
        "min": 0,
        "transfer": {
          "3": 1234567890
        },
        "next_update": {
          "3": "2025-07-15T00:00:00Z"
        }
      }
    }
  }
}
```

### 获取监控历史

```http
GET /api/v1/monitor/:id
```

返回指定服务器最近 24 小时的服务监控历史数据。

### 可用性摘要

```http
GET /api/v1/server/availability?id=<id1,id2,...>&days=30
```

返回服务器可用性摘要。需要 `showavailabilitytoguest` 开启，或携带 Token/登录 Cookie。

---

## API v1（需 API Token 或 Cookie）

### 注册服务器

```http
POST /api/v1/server/register?simple=true
Content-Type: application/json

{
  "name": "new-server",
  "tag": "default"
}
```

**参数**：

| 参数 | 必填 | 说明 |
|------|------|------|
| `simple` | 否 | `true` 时只返回 Secret |

**响应示例**（`simple=true`）：

```json
"abcdef123456"
```

**响应示例**（`simple=false`）：

```json
{
  "code": 200,
  "message": "success",
  "result": { ... }
}
```

---

## 离线历史 API v1

以下接口支持 API Token 或 Cookie 认证。

### 查询离线历史列表

```http
GET /api/v1/offline-history?server_id=<ID>&page=1&page_size=20
```

**参数**：

| 参数 | 必填 | 说明 |
|------|------|------|
| `server_id` | 是 | 服务器 ID |
| `page` | 否 | 页码，默认 `1` |
| `page_size` | 否 | 每页条数，默认 `20`，最大 `100` |

**响应示例**：

```json
{
  "code": 200,
  "message": "",
  "result": {
    "items": [
      {
        "id": 4,
        "server_id": 45,
        "started_at": "2026-06-21T15:33:39+08:00",
        "detected_at": "2026-06-21T15:33:39+08:00",
        "ended_at": null,
        "duration_seconds": 0,
        "reason": "unknown",
        "status": "open",
        "threshold_seconds": 30,
        "last_seen_at": "2026-06-21T15:33:09+08:00",
        "last_boot_time": 0,
        "recovered_boot_time": 0,
        "last_ip": "",
        "recovered_ip": ""
      }
    ],
    "total": 1
  }
}
```

**字段说明**：

| 字段 | 说明 |
|------|------|
| `id` | 记录 ID |
| `started_at` | 离线开始时间 |
| `detected_at` | 系统检测到离线的时间 |
| `ended_at` | 恢复时间，`null` 表示仍在离线中 |
| `duration_seconds` | 持续秒数 |
| `reason` | 原因：`machine_reboot` / `network_disconnect` / `unknown` / `agent_restart` / `dashboard_restart` / `manual` |
| `status` | 状态：`open` 未恢复 / `closed` 已恢复 |
| `threshold_seconds` | 离线判定阈值 |
| `last_ip` | 离线前 IP |
| `recovered_ip` | 恢复后 IP |

### 查询离线统计摘要

```http
GET /api/v1/offline-history/summary?server_id=<ID>&days=30
```

**参数**：

| 参数 | 必填 | 说明 |
|------|------|------|
| `server_id` | 是 | 服务器 ID |
| `days` | 否 | 统计天数，默认 `30`，最大 `3660` |

**响应示例**：

```json
{
  "code": 200,
  "message": "",
  "result": {
    "server_id": 45,
    "days": 30,
    "offline_count": 1,
    "total_offline_seconds": 3002,
    "longest_offline_seconds": 3002,
    "availability_percent": 99.88,
    "reboot_count": 0,
    "network_disconnect_count": 0,
    "unknown_count": 1
  }
}
```

### 手动清理离线历史

```http
POST /api/v1/offline-history/cleanup
Content-Type: application/json
Authorization: <your-token>

{
  "before_days": 365
}
```

> 需要超级管理员权限。

**响应示例**：

```json
{
  "code": 200,
  "message": "",
  "result": {
    "deleted": 10
  }
}
```

### 删除单条离线历史

```http
DELETE /api/v1/offline-history/<id>
Authorization: <your-token>
```

**响应示例**：

```json
{
  "code": 200,
  "message": ""
}
```

### 重置单台服务器可用性

清空该服务器全部离线历史并复位运行态（计时从当前时刻重新开始），用于修复异常数据
（如遗留未关闭记录导致的“无限离线”）或人工重新统计。

```http
POST /api/v1/server/<id>/reset-availability
Authorization: <your-token>
```

**响应示例**：

```json
{
  "code": 200,
  "message": "",
  "result": {
    "deleted": 9
  }
}
```

---

## 统一模型列表（需 API Token 或 Cookie）

```http
GET /api/v1/:model
```

统一返回各类前端模型的列表。支持的 `:model`：

| `:model` | 说明 | 无 Token 时 |
|----------|------|-------------|
| `cron` | 计划任务 | 隐藏 `command`、`servers` |
| `notification` | 通知方式 | 隐藏 `url`、`request_header`、`request_body` 等 |
| `ddns` | DDNS 配置 | 隐藏 `access_id`、`access_secret`、webhook 配置 |
| `nat` | NAT 配置 | 返回全部字段 |
| `alert-rule` | 告警规则 | 隐藏 `rules`、触发任务列表 |
| `user` | 用户 | 隐藏 `token` |
| `transfer` | 流量记录 | 返回全部字段 |
| `server-runtime` | 服务器运行状态 | 需要 Token |
| `api-token` | API Token 列表 | 需要 Token，仅返回当前用户的 Token |
| `setting` | 站点配置 | 无 Token 时返回 `PublicConfig`，带 Token 时返回完整配置 |

**响应示例**（`GET /api/v1/cron`，带 Token）：

```json
{
  "code": 200,
  "message": "success",
  "result": [
    {
      "id": 1,
      "name": "Debian版本",
      "task_type": 1,
      "scheduler": "",
      "push_successful": true,
      "notification_tag": "default",
      "cover": 1,
      "last_executed_at": "2025-04-18T14:01:24Z",
      "last_result": true,
      "command": "cat /etc/debian_version",
      "servers": []
    }
  ]
}
```

---

## Dashboard 会员 API（需登录 Cookie）

Base URL：`/api`

以下接口用于 Dashboard 页面内部调用，第三方前端如需使用请携带登录 Cookie。

### 服务器管理

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/search-server` | 搜索服务器 |
| POST | `/api/server` | 添加/编辑服务器 |
| POST | `/api/server/:id/reset-secret` | 重置 Secret |
| POST | `/api/batch-update-server-group` | 批量修改分组 |
| POST | `/api/batch-delete-server` | 批量删除服务器 |
| POST | `/api/force-update` | 强制 Agent 更新 |

### 服务监控

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/api/monitor` | 添加/编辑监控 |

### 计划任务

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/api/cron` | 添加/编辑任务 |
| GET | `/api/cron/:id/manual` | 手动触发任务 |

### 通知与告警

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/api/notification` | 添加/编辑通知方式 |
| POST | `/api/alert-rule` | 添加/编辑告警规则 |

### DDNS / NAT

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/api/ddns` | 添加/编辑 DDNS |
| POST | `/api/nat` | 添加/编辑 NAT |

### 设置

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/api/setting` | 保存设置 |

### API Token

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/token` | 列出 Token |
| POST | `/api/token` | 创建 Token |
| DELETE | `/api/token/:token` | 删除 Token |

### 通用删除

| 方法 | 路径 | 说明 |
|------|------|------|
| DELETE | `/api/:model/:id` | 删除指定模型的一条记录 |

支持的 `:model` 包括：`server`、`monitor`、`cron`、`notification`、`alert-rule`、`ddns`、`nat` 等。

### 离线历史（会员 API）

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/offline-history?server_id=&page=&page_size=` | 查询某服务器的离线历史 |
| GET | `/api/offline-history/summary?server_id=&days=` | 查询离线统计摘要 |
| POST | `/api/offline-history/cleanup` | 手动清理历史（需超级管理员） |
| DELETE | `/api/offline-history/:id` | 删除单条历史 |
| POST | `/api/server/:id/reset-availability` | 重置单台服务器可用性 |

> 数据格式与 `/api/v1/offline-history*` 一致，但仅支持 Cookie 登录。

---

## 错误码

| 状态码 | 含义 |
|--------|------|
| `200` | 成功 |
| `400` | 请求参数错误 |
| `403` | 未登录、无权限或 CSRF 校验失败 |
| `404` | 资源不存在 |
| `500` | 服务器内部错误 |

---

## 调用示例（curl）

### 使用 API Token 获取服务器列表

```bash
curl -s -H 'Authorization: <your-token>' \
  'https://santaizi.example.com/api/v1/server/list'
```

### 使用 API Token 获取统一模型列表

```bash
curl -s -H 'Authorization: <your-token>' \
  'https://santaizi.example.com/api/v1/cron'
```

### 使用 API Token 获取离线历史

```bash
curl -s -H 'Authorization: <your-token>' \
  'https://santaizi.example.com/api/v1/offline-history?server_id=12&page=1&page_size=20'
```

### 使用 Cookie 调用会员 API

```bash
curl -s -b 'santaizi-dashboard=<your-cookie>' \
  'https://santaizi.example.com/api/offline-history/summary?server_id=12&days=30'
```
