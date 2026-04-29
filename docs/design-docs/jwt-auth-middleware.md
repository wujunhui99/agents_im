# JWT Auth Middleware

状态：Implemented

## 背景

auth service 负责账号密码注册和登录，并为客户端后续请求签发 access token。user、friends、groups、message 的受保护 HTTP 接口需要统一鉴权方式，不能继续依赖客户端可伪造的 `X-User-Id` 作为生产身份来源。

## 目标

- auth 注册和登录签发 JWT compact serialization，使用 HS256，payload 至少包含 `user_id`、`identifier`、`iat`、`exp`。
- 受保护 HTTP route 使用 go-zero `.api` 的 `jwt: Auth` 声明和 `rest.WithJwt` route option。
- 业务 logic adapter 通过统一 helper 从 request context 获取 `user_id`。
- 缺少 Bearer token、token 格式错误、签名错误或过期时返回 HTTP 401。
- `X-User-Id` 仅允许出现在测试中的 bypass 断言或历史文档说明，不作为生产鉴权路径。

## 非目标

- 不实现 refresh token、logout、revoke 或 token blacklist。
- 不实现手机号、微信、OAuth 等登录方式。
- 不实现 PostgreSQL 持久化、docker-compose 或 SQL migration。
- 不定义 gateway 长连接鉴权细节。

## 配置

HTTP API 和 auth RPC 配置包含：

```yaml
Auth:
  AccessSecret: dev-jwt-secret-change-me
  AccessExpire: 86400
```

- `AccessSecret` 是本地开发 placeholder，生产环境必须由部署系统或密钥管理系统覆盖。
- `AccessExpire` 单位为秒。
- 各受保护 API 与 auth service 必须使用同一 active secret。

## 路由边界

Public endpoints:

- `POST /auth/register`
- `POST /auth/login`
- `POST /auth/validate`
- `POST /users`
- `GET /users/exists`
- `GET /users/:identifier`
- `GET /groups/:group_id`
- `GET /groups/:group_id/members`
- `GET /healthz`

Protected endpoints:

- `GET /me`
- `PATCH /me`
- `POST /friends`
- `GET /friends`
- `DELETE /friends/:user_id`
- `GET /friends/:user_id`
- `POST /groups`
- `POST /groups/:group_id/members`
- `DELETE /groups/:group_id/members/me`
- `POST /messages`
- `GET /conversations/:conversation_id/messages`
- `GET /conversations/seqs`
- `POST /conversations/:conversation_id/read`

## Context User

go-zero JWT middleware parses `Authorization: Bearer <token>` and injects non-standard JWT claims into `context.Context`. The repository helper `internal/ctxuser.UserID(ctx)` reads the `user_id` claim and returns `UNAUTHENTICATED` if it is missing or blank.

Handlers continue to stay thin: they parse typed request data and instantiate logic. Logic adapters decide the current user from context and call service logic with that user id.

## Message Sender Rule

`POST /messages` uses the token `user_id` as `sender_id`. If a client sends a `senderId` field in the request body, it must match the token `user_id`; otherwise the request fails with `INVALID_ARGUMENT`. This prevents a client from forging the sender while keeping compatibility with clients that already include an explicit sender field.

## Error Handling

- Missing or invalid Bearer token: HTTP 401, `UNAUTHENTICATED`.
- Missing `user_id` claim after JWT middleware: HTTP 401, `UNAUTHENTICATED`.
- `senderId` mismatch in message send: HTTP 400, `INVALID_ARGUMENT`.
- Auth public `POST /auth/validate` keeps its existing behavior and validates a token passed in the JSON body.

## 验证方式

- `goctl api validate -api api/*.api`
- `go test ./...`
- `bash scripts/verify-static.sh`

Tests cover:

- register/login return JWT-shaped tokens;
- protected route without token returns 401;
- invalid token returns 401;
- Bearer token can access `/me`;
- friends/groups/message logic uses token `user_id`;
- `X-User-Id` alone cannot bypass protected routes.
