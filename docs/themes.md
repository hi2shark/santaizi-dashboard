# 前端与安全定制

Dashboard 仅提供两个独立 Vue 3 应用：

- `/admin/*`：Element Plus 管理后台；
- `/`、`/service`、`/network`：默认 ServerStatus 公开站。

项目不再加载 Go HTML 模板，也没有前台/后台主题选择；公开站固定使用 `server-status`。自定义 HTML/JavaScript 不会执行。

## 安全外观定制

可在管理后台的“设置 → 外观定制”修改品牌色、页脚、Logo、背景和受限 CSS：

```yaml
site:
  primarycolor: "#2563eb"
  footertext: "Santaizi Monitoring"
  logourl: "/static/logo.svg"
  backgroundurl: "/static/theme-server-status/img/bg.jpg"
  safecustomcss: ""
```

Logo 和背景只接受 `/static/` 本地资源或 `data:image/` 图片。CSS 拒绝 `@import`、远程 `url()`、`expression`、`javascript:` 和可执行标签。

## 嵌入与外置交付

默认在执行 `pnpm build` 后将三个应用嵌入 Go 二进制：

```yaml
web:
  delivery: embedded
```

外置模式必须使用同域反向代理托管静态产物，并继续把 `/api`、`/oauth2`、`/ws` 和 `/openapi` 转发到 Go：

```yaml
web:
  delivery: external
```

不支持通过跨域 Cookie 或宽松 CORS 拆分部署。
