---
layout: home
title: Santaizi API
hero:
  name: Santaizi API v2
  text: 完整、稳定、可生成的接口契约
  tagline: 管理后台、ServerStatus 和自动化客户端共享同一份 OpenAPI 3.0.3 契约。
  actions:
    - theme: brand
      text: 浏览接口
      link: /reference
    - theme: alt
      text: 下载 OpenAPI
      link: /openapi/v2.yaml
features:
  - title: 统一响应
    details: 单项使用 {data}，列表使用 {data, meta}，错误采用 application/problem+json。
  - title: 会话与 Token
    details: 浏览器使用 OAuth Cookie + CSRF，自动化客户端使用 Bearer Token。
  - title: 可生成 SDK
    details: Go 服务接口与 Axios TypeScript SDK 从规范生成，避免手写 URL 和重复 DTO。
---

## 快速开始

管理 API 需要登录后的 Cookie 会话或 API Token。浏览器写操作还需要从 `GET /api/v2/auth/session` 获取 CSRF Token，并通过 `X-CSRF-Token` 发送。

```bash
curl -H "Authorization: Bearer $SANTAIZI_API_TOKEN" \
  https://monitor.example.com/api/v2/admin/servers
```

规范文件同时公开为 [`/openapi/v2.yaml`](/openapi/v2.yaml) 和 [`/openapi/v2.json`](/openapi/v2.json)。
