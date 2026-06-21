# 主题与自定义

## 前台主题

前台主题目录位于 `resource/template/theme-<name>/`，内置主题包括：

- `default`
- `daynight`
- `mdui`
- `hotaru`
- `angel-kanade`
- `server-status`
- `custom`（本地自定义）

对应静态资源位于 `resource/static/theme-<name>/`。

在 **设置** 页面（`/setting`）的 `site.theme` 中选择主题，或修改 `config.yaml`：

```yaml
site:
  theme: "server-status"
```

---

## 后台主题

后台主题目录位于 `resource/template/dashboard-<name>/`，内置：

- `default`
- `custom`（本地自定义）

在 **设置** 页面选择后台主题，或修改 `config.yaml`：

```yaml
site:
  dashboardtheme: "default"
```

---

## 自定义主题

### 前台自定义

1. 创建文件 `resource/template/theme-custom/home.html`
2. 在 `config.yaml` 中设置 `site.theme: custom`
3. 参考其他主题模板编写页面

### 后台自定义

1. 创建文件 `resource/template/dashboard-custom/setting.html` 等
2. 在 `config.yaml` 中设置 `site.dashboardtheme: custom`

### 静态资源

自定义静态文件可放在 `resource/static/custom/`，运行时会覆盖嵌入资源。

### 自定义代码

无需创建完整主题，也可以在 **设置** 中直接填写：

- `site.customcode`：前台自定义 HTML/JS/CSS
- `site.customcodedashboard`：后台自定义 HTML/JS/CSS

适合添加统计代码、自定义样式等。

---

## 禁用前台主题切换

如果不希望游客切换主题，可在 **设置** 中开启 **前台禁用切换主题**：

```yaml
disableswitchtemplateinfrontend: true
```
