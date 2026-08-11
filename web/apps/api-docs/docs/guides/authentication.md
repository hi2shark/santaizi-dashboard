# 认证与 CSRF

管理端浏览器使用 OAuth Session Cookie。`GET /api/v2/auth/session` 返回当前用户、能力列表及 CSRF Token；所有写操作携带 `X-CSRF-Token`。

自动化客户端使用 `Authorization: Bearer <token>`。

管理员可通过受认证的详情接口查看并复制服务器连接密钥、Collector 注册 Token 和 API Token。列表与日志不返回完整明文。
