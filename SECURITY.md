# 安全策略

## 受支持的版本

仅 `main` 分支与最新 SemVer Release。旧 tag 不回溯修复。

主面板、从端与探针（[`santaizi-agent`](https://github.com/hi2shark/santaizi-agent)）线协议不兼容旧版，须成对升级。

## 报告漏洞

请通过 [GitHub 私密安全公告](https://github.com/hi2shark/santaizi-dashboard/security/advisories/new) 提交，**不要**用公开 Issue 或 PR 披露。

请附复现步骤、受影响版本与部署形态（Primary / Collector / 探针）。报告中请勿包含真实主机地址、Token 或私钥。

修复发布前请勿公开细节。

## 不在范围内

* 面板 Web 端口直接暴露公网导致的后果 —— 见 [README 安全要求](./README.md#安全要求必读)，这是部署选择，不是漏洞
* 上游 [哪吒监控](https://github.com/naiba/nezha) 未经本项目修改的代码 —— 请报给上游
* 第三方依赖自身漏洞 —— 请报给对应项目；如本项目未及时升级，可另行提 Issue
