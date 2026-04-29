# Auth Service 第一阶段产品规格

状态：Draft

## 背景

`auth` 是认证边界，负责注册、登录、密码哈希和 token 签发。`user` 已提供用户资料权威能力，`auth` 注册流程必须依赖 `user-rpc` 的 `ExistsByIdentifier` 和 `CreateUser`，但不能把密码、密码哈希、salt、验证码或第三方登录凭据写入 `user` 模型或响应。

## 目标

- 支持账号密码注册。
- 支持账号密码登录。
- 注册前按唯一标识符检查账号是否已存在。
- 注册成功时创建 user 资料，并在 auth 内部保存密码哈希与 salt。
- 登录成功后签发带过期时间的 token。
- 提供 token 校验接口，便于后续 gateway 或其他服务验证登录态。
- 为手机号验证码、微信扫码登录等后续方式保留扩展点，但第一阶段不实现。

## 非目标

- 不实现手机号验证码注册或登录。
- 不实现微信扫码、OAuth 或第三方登录。
- 不实现密码找回、多因素认证、设备管理、刷新 token 或登出黑名单。
- 不维护用户展示资料、好友关系或群成员关系。
- 不把 `password`、`password_hash`、`salt` 等认证秘密写入 `user` 服务。

## 用户场景

### 账号密码注册

1. 客户端提交 `identifier`、`password`，可选提交 `display_name`、`name`、`gender`、`age`、`region`。
2. `auth` 调用或适配 `user` 的 `ExistsByIdentifier`。
3. 如果唯一标识符已存在，注册失败。
4. 如果不存在，`auth` 调用或适配 `user` 的 `CreateUser` 创建基础资料。
5. `auth` 生成 salt 和 password_hash，并仅在 auth 内部保存认证记录。
6. `auth` 签发 token 并返回 `user_id`、`identifier`、`token`、`expires_at`。

验收标准：

- 重复 `identifier` 返回明确冲突错误。
- 注册响应不包含明文密码、密码哈希或 salt。
- `user` 模型与 user 响应不出现任何认证秘密字段。
- 注册成功后可用返回 token 通过 auth token 校验。

### 账号密码登录

1. 客户端提交 `identifier` 和 `password`。
2. `auth` 查询内部认证记录。
3. `auth` 使用保存的 salt 与 password_hash 校验密码。
4. 校验通过时签发新 token。

验收标准：

- 不存在账号或密码错误均返回认证失败。
- 登录成功返回 `user_id`、`identifier`、`token`、`expires_at`。
- 登录响应不包含明文密码、密码哈希或 salt。

### Token 校验

1. 调用方提交 token。
2. `auth` 校验 token 签名和过期时间。
3. 校验成功时返回 `valid=true`、`user_id`、`identifier`、`expires_at`。

验收标准：

- token 必须有过期时间。
- 过期、签名非法或格式错误的 token 返回未认证错误。
- 第一阶段 token 使用本地 HMAC/JWT-like 实现；后续可替换为标准 JWT 库或集中鉴权服务。

## 第一阶段接口

### `POST /auth/register`

请求：

```json
{
  "identifier": "alice_001",
  "password": "example-password",
  "display_name": "Alice",
  "name": "Alice",
  "gender": "female",
  "age": 30,
  "region": "Shanghai"
}
```

响应：

```json
{
  "code": "OK",
  "message": "ok",
  "data": {
    "user_id": "usr_000001",
    "identifier": "alice_001",
    "token": "<token>",
    "expires_at": "2026-04-29T12:00:00Z"
  }
}
```

### `POST /auth/login`

请求：

```json
{
  "identifier": "alice_001",
  "password": "example-password"
}
```

响应同注册。

### `POST /auth/validate`

请求：

```json
{
  "token": "<token>"
}
```

响应：

```json
{
  "code": "OK",
  "message": "ok",
  "data": {
    "valid": true,
    "user_id": "usr_000001",
    "identifier": "alice_001",
    "expires_at": "2026-04-29T12:00:00Z"
  }
}
```

## 依赖关系

- `auth` 依赖 `user` 的 `ExistsByIdentifier` 和 `CreateUser`。
- 当前无真实 RPC 网络时，允许通过接口 adapter 调用 user logic/repository。
- 后续接入 go-zero RPC 后，将 adapter 实现替换为 `user-rpc` client，auth 业务逻辑不直接依赖 user repository。

## 扩展点

- 手机号验证码：后续新增验证码发送、校验、手机号绑定和手机号登录 adapter。
- 微信扫码：后续新增扫码会话、二维码状态轮询、微信身份绑定和登录 adapter。
- token：后续支持刷新 token、登出吊销、密钥轮换和 gateway 统一鉴权。
- 密码哈希：第一阶段使用标准库实现的 salted iterative SHA-256；生产化前应切换为 Argon2id、bcrypt 或 scrypt。

## 风险与待决

- `auth` 创建 user 资料后再保存密码哈希，跨服务场景需要补偿、幂等或事务外盒机制处理部分失败。
- token 密钥管理、轮换策略和刷新 token 生命周期需要后续安全设计。
- 账号锁定、登录失败次数限制、密码强度策略第一阶段只做最小校验，后续需要加强。
