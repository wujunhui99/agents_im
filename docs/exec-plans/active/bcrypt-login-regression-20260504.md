# bcrypt-login-regression-20260504

状态：Completed

## 背景

生产反馈显示账号注册成功后，使用同一 identifier/password 登录返回 `invalid identifier or password`。当前代码中 auth schema 只持久化 `password_hash + password_algo`，但密码 hasher 仍输出需要独立 salt 的 `sha256-iter-v1`，且 Postgres repo 将 `password_algo=1` 读回为 `sha256-iter-v1`，与 schema v2 中 `1=bcrypt` 的约定不一致。

## 目标

- 将当前密码哈希算法恢复到 bcrypt，并保留可验证 legacy `sha256-iter-v1` 的代码路径。
- 用 Go 测试覆盖当前算法 round trip、legacy hash 验证、以及注册后通过持久化 credential shape 登录。
- 添加 API-only E2E 脚本，支持 local/production 目标，真实调用 `/auth/register` 和 `/auth/login` 并输出脱敏证据。
- 登录页在用户进入密码框前检查 identifier 是否存在，并展示明确友好提示。

## 非目标

- 不绕过密码校验。
- 不在前端或 E2E 中引入 mock 成功路径。
- 不推送分支。
- 不尝试恢复已经因 salt 未持久化而无法验证的历史坏数据。

## 任务拆分

- [x] Task 1：新增失败优先 Go 覆盖，定位 hasher/schema/repo 语义不一致。
- [x] Task 2：新增失败优先前端测试，覆盖登录页密码框 focus 时调用 `/users/exists`。
- [x] Task 3：实现 bcrypt 当前 hasher、legacy SHA 验证和 Postgres `password_algo` 映射修复。
- [x] Task 4：实现登录页 identifier existence UX。
- [x] Task 5：新增 `tests/e2e/auth_register_login_regression.mjs` 与 README 用法。
- [x] Task 6：运行要求的 Go、Node、frontend、static、diff 校验；能启动或已有服务时运行 local E2E。
- [x] Task 7：复核 diff，提交 `fix(auth): restore register login password verification`。

## 决策日志

| 时间 | 决策 | 原因 |
| --- | --- | --- |
| 2026-05-04 | 当前算法使用 bcrypt，legacy SHA 仅用于验证旧 hash | schema v2 已明确 bcrypt hash 内嵌 salt；保留旧 verifier 避免主动破坏仍带 salt 的历史 credential |
| 2026-05-04 | `/users/exists` 用于登录页预检查 | 前后端契约已存在公开 identifier existence API |

## 验证方式

- `export PATH=/tmp/go/bin:$HOME/go/bin:$PATH`
- `node --check tests/e2e/auth_register_login_regression.mjs`
- `go test ./internal/auth/... ./tests -run 'TestAuth|TestPassword|TestRegister|TestLogin' -count=1`
- `npm --prefix web run test:run -- --reporter=dot`
- `npm --prefix web run build`
- `bash scripts/verify-static.sh`
- `git diff --check`

## 风险与回滚

- 风险：生产中若已有 bug 窗口写入的 salted SHA hash 但 salt 已丢失，这类 credential 无法验证；代码不能安全恢复未知 salt。
- 回滚：恢复前一版本 auth 镜像会回到注册后登录失败风险，不建议作为长期回滚；若部署后发现范围问题，应保留 bcrypt fix 并做账号重置/修复流程。

## 结果记录

- Go targeted regression：`go test ./internal/auth/... ./tests -run 'TestAuth|TestPassword|TestRegister|TestLogin' -count=1` 通过。
- E2E script syntax：`node --check tests/e2e/auth_register_login_regression.mjs` 通过。
- Frontend：`npm --prefix web run test:run -- --reporter=dot`、`npm --prefix web run build`、`npm --prefix web run lint` 通过。
- Static/whitespace：`bash scripts/verify-static.sh`、`git diff --check` 通过。
- Local fixed-code E2E：启动本 worktree memory-backed `auth-api` 于 `http://127.0.0.1:18081`，`AGENTS_IM_E2E_TARGET=local AGENTS_IM_E2E_BASE_URL=http://127.0.0.1:18081 node tests/e2e/auth_register_login_regression.mjs` 返回 `register-login-success`。
- Production evidence-mode E2E：`AGENTS_IM_E2E_TARGET=production AGENTS_IM_E2E_BASE_URL=https://agenticim.xyz AGENTS_IM_E2E_ALLOW_REPRO_FAILURE=1 node tests/e2e/auth_register_login_regression.mjs` 观察到 `login-invalid-after-register`。
