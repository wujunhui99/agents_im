# issue-90-admin-domain

状态：Active

## 背景

Issue #90 将已实现的只读 Admin Console 从普通用户域名下的 `/admin` 兼容入口迁移到独立域名 `admin.agenticim.xyz`。DNS 已指向当前 ingress IP，但仓库里的 k8s ingress 只有 `agenticim.xyz` host/TLS 规则；React 也只按 `/admin` path 选择 AdminConsole。

## 目标

- `https://admin.agenticim.xyz/` 通过 web SPA 渲染 Admin Console。
- `admin.agenticim.xyz` 下的 `/admin/dashboard`、`/admin/llm-traces`、`/admin/conversations`、`/admin/users` 继续走 `message-api:8083`。
- `https://agenticim.xyz/` 用户 App 不改变，`/admin` 作为兼容路由保留。
- 文档说明域名与安全边界；新增测试/static guard 防止路由回退。

## 非目标

- 不修改 Admin API 的读写能力。
- 不新增用户 mutation、管理员 impersonation 或代发消息。
- 不执行生产部署；部署仍受 Drone PR #89 阻塞。

## 任务拆分

- [x] 添加 React admin host root 渲染回归测试，并确认先失败。
- [x] 添加 ingress admin host/TLS/backend static guard，并确认先失败。
- [x] 更新 `web/src/App.tsx`，用 host-based helper 支持 `admin.agenticim.xyz/`，保留 `/admin` 兼容。
- [x] 更新 `deploy/k8s/ingress.yaml`，增加 admin host TLS 和路由规则。
- [x] 更新 `deploy/README.md` 与 `docs/SECURITY.md`。
- [x] 运行 Issue #90 指定验证命令。
- [ ] commit/push，创建或更新 `feature/issue-90-admin-domain -> develop` PR，PR body 包含 `Closes #90`。
- [ ] 在 Issue #90 评论实现摘要、测试、分支/commit/PR 和 blocker。

## 决策日志

| 时间 | 决策 | 原因 |
| --- | --- | --- |
| 2026-05-19 | 保留 `agenticim.xyz/admin` 兼容路由，不做前端跳转。 | Issue 允许兼容或跳转；保留现有行为改动最小，且不会依赖跨域 API。 |
| 2026-05-19 | `admin.agenticim.xyz` 使用单独 TLS secret。 | 便于独立观察 cert-manager 对管理域名的证书签发状态。 |
| 2026-05-19 | 测试通过可注入 location 覆盖 hostname。 | jsdom 不能可靠跨 origin 修改 `window.location.hostname`；注入 location 让测试确定且生产默认仍使用 `window.location`。 |

## 验证方式

```bash
export PATH=/tmp/go/bin:$HOME/go/bin:$PATH
for f in api/*.api; do goctl api validate -api "$f"; done
go test ./internal/handler ./internal/logic ./internal/repository ./tests
npm --prefix web run test:run -- --reporter=dot src/App.test.tsx -t admin
npm --prefix web run test:run -- --reporter=dot src/pages/AdminConsole.test.tsx
npm --prefix web run build
bash scripts/verify-static.sh
docker compose config
git diff --check
```

## 风险与回滚

风险：ingress host 或 path 配置错误会导致 admin 域名证书未签发或 API 路由落到 web SPA。回滚方式是恢复 `deploy/k8s/ingress.yaml` 中新增的 `admin.agenticim.xyz` TLS/rule 条目，并继续使用 `/admin` 兼容入口。

## 结果记录

2026-05-19 验证已通过：

- `for f in api/*.api; do goctl api validate -api "$f"; done`
- `go test ./internal/handler ./internal/logic ./internal/repository ./tests`
- `npm --prefix web run test:run -- --reporter=dot src/App.test.tsx -t admin`
- `npm --prefix web run test:run -- --reporter=dot src/pages/AdminConsole.test.tsx`
- `npm --prefix web run build`
- `bash scripts/verify-static.sh`
- `docker compose config`
- `git diff --check`

commit、PR 和 Issue 评论待完成后在 GitHub 记录。
