# 前端与安全定制

Dashboard 提供独立 Vue 3 应用：

- `/admin/*`：Element Plus 管理后台（主题固定 `spa`）
- `/`、`/service`、`/network`：公开站壳，内置主题白名单切换

## 公开站内置主题

| ID | 说明 |
| -- | ---- |
| `server-status` | 默认表格/分组视图（现有 Status UI） |
| `nazhua` | Nazhua 忠实移植：独立页面壳、地图、卡片/列表/ServerStatus、详情地球仪、周期流量 |

Admin「设置 → 外观」可选择默认主题，并可开关「允许访客切换主题」。访客切换写入 `localStorage`（`santaizi-public-theme`），在允许时覆盖站点默认。

公开站仍共用 Santaizi V2 API、WebSocket 和状态 store，但每个主题通过内部 `PublicThemeDefinition` 分别注册 Shell、首页、详情、服务状态和网络页面。同一时刻仅挂载一个 Shell，因此主题头部、背景、主体和页脚不会与另一主题叠加。Nazhua 的功能菜单集中提供主题、语言、明暗模式、服务状态、网络和后台入口。

Nazhua 的视觉和资产固定参考上游 commit `d08c973bb4446a24356f49b81d75d6773286596e`，并以在线原版同视口截图、DOM 尺寸和计算样式复核；来源和 MIT 许可见 `NOTICE` 与 `web/packages/theme-nazhua/LICENSE`。主题使用系统中文字体栈，不内置 Sarasa 字体文件。视觉取证、并排比较和可接受差异记录在 `design-qa.md`。

不支持：用户上传主题包、Go HTML 模板主题、任意 `config.js`、对 Nezha v0/v1 的兼容层。

## 安全外观定制

可在管理后台的“设置 → 外观定制”修改品牌色、页脚、Logo、背景和受限 CSS：

```yaml
site:
  theme: server-status # 或 nazhua
  primarycolor: "#2563eb"
  footertext: "Santaizi Monitoring"
  logourl: "/static/logo.svg"
  backgroundurl: "/static/theme-server-status/img/bg.jpg"
  safecustomcss: ""
```

Logo 和背景只接受 `/static/` 本地资源或 `data:image/` 图片。CSS 拒绝 `@import`、远程 `url()`、`expression`、`javascript:` 和可执行标签。自定义 HTML/JavaScript 不会执行。

## 嵌入与外置交付

默认在执行 `pnpm build` 后将三个应用嵌入 Go 二进制：

```yaml
web:
  delivery: embedded
```

外置模式必须使用同域反向代理托管静态产物，并继续把 `/api`、`/oauth2`、`/ws` 和 `/openapi` 转发到 Go。
