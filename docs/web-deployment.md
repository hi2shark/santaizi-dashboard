# Vue 前端构建与部署

Dashboard 包含三个独立项目：`web/apps/admin`、`web/apps/status`、`web/apps/api-docs`。它们共享生成 SDK、i18n 和设计令牌。

## 嵌入二进制（默认）

```bash
corepack enable
pnpm install --frozen-lockfile
pnpm build
go build ./cmd/dashboard
```

产物进入 `resource/web/` 并由 `embed.FS` 编入 Dashboard。入口 HTML 使用 `no-store`，带内容哈希的静态资源使用长期缓存。

## 同域外置静态容器

将 Dashboard 配置为：

```yaml
web:
  delivery: external
```

构建静态容器：

```bash
docker build -f web/Dockerfile -t santaizi-web .
```

运行时设置 `DASHBOARD_UPSTREAM=http://<dashboard-service>:80`。容器在同一域名提供 `/`、`/admin/` 和 `/docs/api/`，并将 `/api`、`/oauth2`、`/ws`、`/openapi`、`/static` 转发给 Go。不要拆成跨域部署；OAuth Cookie、CSRF 与 WebSocket 均按同源设计。

所有 SPA 路由均配置 fallback，直接刷新 `/admin/telemetry`、`/service` 或文档子页不会返回 404。
