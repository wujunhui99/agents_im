# SECURITY.md

本文档记录安全边界和安全要求。

## 核心风险

- Agent 代码执行工具可能带来命令执行风险。
- 网络搜索或外部访问可能带来数据泄露风险。
- IM 工具调用需要防止越权访问会话和消息。
- Webhook 需要防止伪造请求和重放攻击。

## 初始要求

- 所有用户请求必须鉴权。
- Agent 工具调用必须有权限模型和审计日志。
- Webhook 请求必须签名验证，并支持时间戳防重放。
- 敏感配置必须通过密钥管理，不得提交到仓库。
- 生产部署 secret 只保存在服务器/k3s 或 GitHub repository secrets 中；`deploy/middleware/.env.example` 与 `deploy/k8s/secrets.example.yaml` 只能保留占位示例。
- GitHub Actions deploy workflow 只使用内置 `GITHUB_TOKEN` 推送 GHCR 镜像和刷新服务器侧 `ghcr-pull-secret`，不得把真实 token 写入仓库或日志；文档和示例命令中必须用 `***` 或 `[REDACTED]` 占位。
- 日志中不得记录明文 token、密码或敏感个人信息。

## 待设计

- Agent 沙箱策略
- 工具调用 allowlist / denylist
- 多租户数据隔离
- 安全审计事件 schema
