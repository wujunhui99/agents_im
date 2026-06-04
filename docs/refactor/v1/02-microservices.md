# 02 — Auth / User / Friends / Groups 微服务技术债

> 目标：聚焦"业务域"四件套（auth/user/friends/groups），逐个列出当前实现的技术债、边界模糊点、API/RPC 不一致点，并给出收敛建议。
>
> 配套文档：[`01-project-structure.md`](./01-project-structure.md) 决定**搬到哪里**，本文决定**搬完之后里面长什么样**。
>
> **路径约定**：本文中 `internal/auth/...`、`internal/logic/...`、`internal/handler/...`、`internal/rpcgen/...`、`internal/mail/...` 等指 **agents_im 当前**真实位置（重构前现状）。按 00-decisions **D10**，重构后这些代码全部下沉：
> - 业务 → `service/<domain>/{api,rpc}/internal/...`
> - 跨服务 middleware / config / errcode → `pkg/<name>/`
> - 顶层 `internal/` / `proto/` / `api/` / `rpcgen/` 整体退役。

---

## 0. 公共问题（四个域都有）

### CP-1 🚨 同一份业务逻辑出现 2~3 份
以 friends 为例：

```text
internal/logic/friends/                           # 8 个 *_logic.go（单体时代）
service/friends/api/internal/logic/friends/      # 8 个 *_logic.go（迁移后 api 侧）
service/friends/rpc/internal/logic/              # 8 个 *logic.go（迁移后 rpc 侧）
```

三份**几乎一样**的 add/delete/list/get/accept/reject/list_requests 逻辑同时存在。每份会随时间漂移：

- internal 旧版只是没人删；
- service api 版本走 BFF，调用 rpc；
- service rpc 版本是真正的业务实现。

groups 同样的问题：`internal/logic/groups/` 9 个 + `service/groups/rpc/internal/logic/` 10 个。

> **修复优先级最高**。每个域只保留 **rpc.logic（业务真相）** + **api.logic（BFF/聚合）**，删 internal 残骸。

### CP-2 🚨 命名风格 `xxxlogic.go` vs `xxx_logic.go` 同仓库内并存

go-zero `goctl` 生成器在不同版本默认输出不同：
- `service/friends/rpc/internal/logic/addfriendlogic.go`（无下划线）
- `service/friends/api/internal/logic/friends/add_friend_logic.go`（有下划线）

读起来非常分裂。

> 修复：goctl 调用统一加 `--style=goZero`（或都 snake_case），CI 加 lint。

### CP-2b 🚨 RPC 客户端目录命名 `xxxservice/` vs `xxxclient/` 混用

goctl 生成 client 包的目录名 = proto 里 `service` 名小写。当前 5 个 proto 里 service 命名两种风格并存：

| proto                                  | service 声明        | 生成的客户端目录    |
|----------------------------------------|---------------------|---------------------|
| `service/auth/rpc/auth.proto`          | `service AuthService` | `authservice/`     |
| `service/mail/rpc/mail.proto`          | `service MailService` | `mailservice/`     |
| `service/friends/rpc/friends.proto`    | `service Friends`     | `friendsclient/` (goctl 见与 pb 目录 `friends/` 冲突，自动加 `client` 后缀) |
| `service/groups/rpc/groups.proto`      | `service Groups`      | `groupsclient/`    |
| `service/user/rpc/user.proto`          | `service User`        | `userclient/`      |

后果：调用方 import path 一会儿 `userclient`、一会儿 `authservice`，没法形成肌肉记忆；加新 proto 时新人不知道选哪种风格。

> 修复：少数派改向多数派。
>
> 1. 改 `auth.proto`/`mail.proto`：`service AuthService` → `service Auth`、`service MailService` → `service Mail`；
> 2. 重新 goctl 生成 → 产出 `authclient/`、`mailclient/`；
> 3. 全仓 sed 改 import path：`authservice` → `authclient`、`mailservice` → `mailclient`；
> 4. 删旧目录 `authservice/`、`mailservice/`。
>
> 收益：5 个 RPC 全部统一为 `xxxclient/` 风格，01 §3 目标布局示例也按此口径。

### CP-3 ⚠️ RPC 间禁止互调，但 API 调用 RPC 时是否能用 `service/<domain>/rpc/<domain>service/` 客户端没明确
`AGENTS.md` 第 1 条："rpc 之间不允许相互调用；跨域组合由 API/BFF 编排"，**但**：

- auth-rpc/api 在注册时调用 user-rpc（创建 account）→ 通过 `internal/auth/useradapter` 抽象；
- 是否所有 RPC 客户端调用都强制走 adapter？还是直接 import `userservice.UserService`？现在两种都有。

> 修复 → 已锁定为 **00-decisions D12**：**RPC 间一律不互调，且不经 `<other>adapter` 间接调**；`rpc/internal` 不持任何他域 rpc client；跨域组合全部由 API 层 import RPC client 编排。落到 auth：注册由 auth-api 编排（user-rpc 建 account + auth-rpc 建 credential），发验证码邮件由 auth-api 编排或走异步事件。

### CP-4 ⚠️ API/RPC 类型重复 + 手工 convert
每个域都有：
- `service/<domain>/api/internal/types/types.go`（API DTO）
- `service/<domain>/rpc/<domain>/<domain>.pb.go`（RPC 消息）
- `service/<domain>/{api,rpc}/internal/logic/convert.go`（两边都有 convert！）
- `internal/model/<domain>.go`（domain model，部分域有）

这是 go-zero 模式的必然结果，但当前**没有一个统一的 domain model**，每加一个字段要改 4 处。

> 修复：在 `service/<domain>/rpc/internal/domain/` 建立单一权威 domain model（00-decisions D10：不再设顶层 `internal/domain/`），convert 函数集中到一个文件；API/RPC types 都从 domain model 转换。

### CP-5 ⚠️ 用户身份字段 `UserID` 含义漂移
- `accounts.account_id` 是 Snowflake 数字（PostgreSQL bigint）；
- API/RPC 里都叫 `user_id`、`UserID string`；
- 友群里的 `creator_user_id`、`friend_id` 都是同样的别名；
- `ARCHITECTURE.md` 说"V0 keeps user_id alias for account_id"，但**已经 V0 阶段了**，全仓没有计划怎么去掉 alias。

> 修复：写一份 `docs/refactor/identifier-cleanup.md`，列出"V1 阶段统一改为 `account_id`"的 schedule；先在内部 domain model 里改名为 `AccountID`，API/RPC 公开字段保留 `user_id` 但加 @deprecated 注释。

### CP-6 ⚠️ 鉴权信息分散
- auth-rpc 持有 `auth.AccessSecret`；
- user-api / friends-api / groups-api / msg-api 各自重复声明 `Auth.AccessSecret`、`Auth.AccessExpire`（见 `etc/*.yaml`）；
- 真正校验 JWT 的中间件在 `internal/auth/...` 和 `internal/ctxuser/...`，但每个 service ServiceContext 都自己 setup 一遍。

> 修复：把 JWT 校验抽成 `pkg/authmiddleware/`（00-decisions D10：跨服务共享中间件），所有 service 通过 `pkg/config.JWTAuthConfig` 注入 access secret，禁止任何 service 自己解 JWT。或者更彻底：API gateway 层（如果以后有）统一鉴权后透传 user claims。

### CP-7 ⚠️ Email 验证码与密码归属
- auth-rpc 既负责密码（hash、校验），又负责 email 验证码（registration code）；
- email 验证码实际上又依赖 mail-rpc 发信；
- 同时 `service/auth/rpc/internal/logic/request_registration_email_code_logic.go` 与 `service/auth/api/internal/logic/auth/request_registration_email_code_logic.go` 各一份。

> 修复方向：保持 auth-rpc 拥有 credential（密码、token）权威；email 验证码可以视为 auth 的子流程，OK；但 mail 发信调用一定要通过 mail-rpc client adapter，不能在 auth 内嵌 mail 实现。

### CP-8 ⚠️ Mail provider 还在 internal/
`internal/mail/{provider.go,tencent_ses.go,config.go}` 还在；但已经存在 `service/mail/rpc/`。说明：
- 第三方 SES 实现没搬过去；
- 别的服务（极可能是 auth）直接 import 了 `internal/mail/`，而不是通过 mail-rpc 调。

> 修复：mail provider 实现必须搬进 `service/mail/rpc/internal/provider/`，对外只暴露 mail-rpc gRPC；任何业务 service 不能再 import internal/mail。

---

## 1. Auth 服务

### 1.1 已迁移情况
- `service/auth/api/`、`service/auth/rpc/` 完整存在；
- `service/auth/api/auth.go`、`service/auth/rpc/auth.go`（goctl 生成的 package main 即入口），干净；
- `service/auth/rpc/internal/config/config_test.go` 存在（最近 #301、#302 修了 env 展开 bug）；
- `internal/auth/{logic,mailadapter,repository,token,useradapter,model}/` 还在。

### 1.2 技术债

| 编号 | 问题 | 影响 |
|------|------|------|
| AU-1 | `internal/auth/logic/` 与 `service/auth/rpc/internal/logic/` 都有 register/login/validate 逻辑 | 谁是真相不清楚 |
| AU-2 | `internal/auth/repository/credential_repo` 是 PostgreSQL 实现；service/auth/rpc 通过 svc 依赖它 | 改为 goctl model 落 `service/auth/rpc/internal/model/`，废 repository 层（D13） |
| AU-3 | `useradapter` 在 internal/auth；rpc 内持 user-rpc client | D12：删除 useradapter，注册编排上移 auth-api（rpc 间不互调） |
| AU-4 | auth.proto 同时定义 `ValidateToken` 和 `ParseToken`，两个 RPC 实际语义有点重叠 | 调用方不知道用哪个 |
| AU-5 | `auth-api` 的 `ValidateToken` HTTP 入口暴露给客户端做 token 校验 | 应只内部使用：gateway/中间件 |
| AU-6 | email_verification_code 与 password reset、二次验证、TOTP 等都属于"auth secondary flow"，但 auth 没有 internal/auth/secondaryflow 抽象 | 后续加新流程会再多堆 logic 文件 |

### 1.3 收敛建议
1. 全部 `internal/auth/*` 搬进 `service/auth/rpc/internal/`：
   ```
   service/auth/rpc/internal/
     credential/       # repository、hasher
     token/            # JWT manager
     verification/     # email 验证码、未来 TOTP
     logic/            # 仅入口逻辑
   ```
   > 注（D12）：auth-rpc **不**持 user-rpc / mail-rpc 客户端（删去原 `adapter/user.go`、`adapter/mail.go`）；建 account、发信等跨域编排上移 `service/auth/api`。
2. 把 ValidateToken HTTP 端点改为只供内部 service 调用（gateway 走 JWT middleware，不调 RPC）。
3. auth.proto 拆 ValidateToken / ParseToken 二选一；前者返回 valid+exp，后者返回完整 claims——但当前两者 message 一模一样，可以合并。

---

## 2. User (Account) 服务

### 2.1 现状
- `service/user/api`、`service/user/rpc` 完整；
- `service/user/rpc/internal/model/{accounts_model.go,profiles_model.go,...}` 是 goctl 生成的 sqlx model；
- API 还在 `internal/handler/user/`（应该已经全部走 `service/user/api/internal/handler`，要 grep 验证）。

### 2.2 技术债

| 编号 | 问题 |
|------|------|
| US-1 | `internal/logic/userlogic.go`、`internal/handler/user/` 与 `service/user/{api,rpc}/internal/...` 三份并存（同 CP-1） |
| US-2 | account ↔ profile 关系：`accounts` 表 + `profiles` 表是两张表，但 API/RPC 类型只有 `User`/`Account` 一个；前端不知道哪个字段对应哪张表 |
| US-3 | media 相关字段（`avatar_media_id`、`avatar_url`、`avatar_url_expires_at`）混在 User 类型里，但 media 实际上是单独的资源——见文档 06 关于 media 寄生在 user-api 的问题 |
| US-4 | `internal/model/user.go`、`service/user/rpc/internal/model/accounts_model.go`、`service/user/rpc/internal/model/profiles_model.go` 三份 model |
| US-5 | `account_type=user|agent|admin` 在多处硬编码字符串而非常量 enum |
| US-6 | 14 个 migration 里 `014_account_email_migration.sql` 表示账号邮箱模型刚刚变过，但 API 类型字段一致性需 audit |

### 2.3 收敛建议
1. account_type 抽 `service/user/rpc/internal/domain/account/types.go` 常量 + 校验函数（00-decisions D10）；replace_all `"user"/"agent"/"admin"` 字符串。
2. media-related avatar 字段从 user 类型剥离，由前端单独调 media API 解析（见文档 06）。
3. `internal/logic/user/`、`internal/handler/user/` 全删，确认 `service/user/api` 没用上。
4. profile / account model 合二为一 domain entity，repository 内部维护两张表 join。

---

## 3. Friends 服务

### 3.1 现状（典型"未完成迁移"形态）
- `proto/friends.proto` + `service/friends/rpc/friends.proto` 双源 proto；
- `internal/logic/friends/` 8 个文件 + `service/friends/api/internal/logic/friends/` 8 个文件 + `service/friends/rpc/internal/logic/` 8 个文件——**3 份**！
- `internal/handler/friends/` 7 个 handler；
- `internal/rpcgen/friends/` 还有生成的 pb 客户端。

### 3.2 技术债

| 编号 | 问题 |
|------|------|
| FR-1 | 三份 logic（CP-1）最严重案例 |
| FR-2 | proto 双源（`proto/friends.proto` + `service/friends/rpc/friends.proto`） |
| FR-3 | `internal/rpcgen/friends/` 仍存活，被谁 import 需 grep 清理 |
| FR-4 | `Friendship.IsFriend` 与 `Friendship.Status` 信息冗余（`status="accepted"` 等价于 `is_friend=true`） |
| FR-5 | `AddFriend` 语义：是发起 pending 请求还是直接加为好友？API/RPC 文档说"一条单向 pending"，但 RPC return `created=true` 没标注是 pending 还是 accepted |
| FR-6 | `FriendProfile` 嵌在 `Friendship` 里，导致每次 list friends 都要查 user 表 join；如果用 friends-rpc → user-rpc，跨域 RPC 调用违反 CP-3 边界 |

### 3.3 收敛建议
1. **立刻**删 `internal/logic/friends/`、`internal/handler/friends/`、`internal/rpcgen/friends/`、`proto/friends.proto`；CI grep 验证。
2. friends-rpc 不应 hydrate `FriendProfile`；只返回 `friend_id`，让 friends-api 调用 user-rpc 聚合（**API 才能跨域**）。
3. `Friendship.Status` 改 enum 常量；删 `is_friend` 字段（status 决定）。
4. AddFriend 返回 enum：`pending_created` / `already_pending` / `already_friend` / `auto_accepted`，避免 bool 模糊。

---

## 4. Groups 服务

### 4.1 现状
- proto 双源：`proto/groups.proto` + `service/groups/rpc/groups.proto`；
- `internal/logic/groups/` 9 文件、`service/groups/rpc/internal/logic/` 10 文件；
- `internal/handler/groups/` 8 个 handler；
- `internal/rpcgen/groups/`。

### 4.2 技术债

| 编号 | 问题 |
|------|------|
| GR-1 | 同 FR-1：双份 logic |
| GR-2 | proto 双源 |
| GR-3 | `GroupMember` 携带 `identifier/display_name/name/avatar_*` → 与 friends-rpc 同样的"跨域 hydrate"问题 |
| GR-4 | `JoinGroup` 与 `AddMember` 并存：前者用户主动加，后者管理员加。两个 RPC 但实现往往交叉 |
| GR-5 | `KickMember` 没有"是否允许踢管理员"的 ACL 边界文档；`groupslogic_acl_test.go` 是唯一防线 |
| GR-6 | 群组 announcement、avatar、description 都是 group meta，但与"成员变更""消息分发"放在同一个 RPC 服务里——meta 操作放 rpc，messaging fanout 放 msg-rpc/transfer，**边界 OK 但当前 groups-rpc 没有任何 message 通知** |

### 4.3 收敛建议
1. 同 friends：删 internal 三件套（logic/handler/rpcgen + proto/groups.proto）。
2. GroupMember 不带 profile 字段；list_members 由 groups-api 聚合 user-rpc 多 ID 查询。
3. JoinGroup vs AddMember：保留两个，但 logic 抽公共 `applyMembership(ctx, action)`，避免两份。
4. 加 group event hook：当成员变化时，直接 produce Kafka 事件 `group.member.added.v1` / `group.member.removed.v1`（不走 outbox，对齐 00-decisions D1/D6；topic 命名规范见 00-decisions D5），由 transfer/push 消费。

---

## 5. 不一致点矩阵（四服务横向）

| 维度                          | auth                  | user                  | friends            | groups             |
|-------------------------------|-----------------------|-----------------------|--------------------|--------------------|
| service/<dom>/ 目录           | ✅                    | ✅                    | ✅                 | ✅                 |
| internal/<dom>/ 仍有代码      | 大量（logic/repo/token） | 中（logic/handler）    | 全部（3 份）       | 全部（3 份）       |
| proto 单源                    | ✅ 只在 service/      | ✅                    | ❌ 双源            | ❌ 双源            |
| 三层 model（domain/api/rpc）  | ❌ 没 domain          | ❌ 没 domain          | ❌                 | ❌                 |
| convert.go 单一               | 两份（api、rpc）      | 两份                  | 两份               | 两份               |
| RPC 间跨调                    | auth → user-rpc 通过 useradapter | n/a            | 想 hydrate user 要么破例要么 N+1 | 同 friends |
| 命名风格                      | `xxx_logic.go`        | `xxx_logic.go`        | API snake / RPC 无下划线 | 同 friends   |
| etc/<svc>.yaml 双份           | 是                    | 是                    | 是                 | 是                 |

---

## 6. 收敛 epic（按 PR 粒度切）

依赖关系：先公共问题，再逐域。

1. **CP-2 命名统一**：一次性 rename `service/friends/rpc/internal/logic/addfriendlogic.go → add_friend_logic.go`，groups 同。配 `goctl --style=goZero`。
2. **CP-8 mail provider 搬家**：`internal/mail/` → `service/mail/rpc/internal/provider/`。
3. **FR/GR proto 单源**：删 `proto/{friends,groups}.proto`，CI 确认。
4. **FR/GR 删 internal/logic 残骸**：grep 验证后删。
5. **FR/GR/US/AU 删 internal/handler、internal/rpcgen、internal/servicecontext 残骸**：合并到 1 个 PR。
6. **CP-4 引入 domain model 层**：在各自 `service/<domain>/rpc/internal/domain/` 建立单一权威类型（00-decisions D10）；convert 集中。
7. **AU 收敛**（D12/D13/D14）：`internal/auth/{logic,token}` 搬进 `service/auth/rpc/internal/`；`repository` 改 goctl model 落 `service/auth/rpc/internal/model/`（废 repository）；**删 `useradapter`/`mailadapter`**——建 account、发信编排上移 auth-api（rpc 间不互调）。鉴权改 **D14 模型**：`AuthRuntime`/`ActiveSessionRepository` 退役，登录 `HSET` 共享 Redis `user_active_sessions:{uid}`（按设备类型分 field，value 含 UUID v4 jti），每请求校验收口到 `pkg/jwtauth`（本地验签 + `HGET` 比对 jti，不调 auth-rpc）。auth-rpc 只保留登录/注册/登出业务。
8. **CP-3 文档化跨域规则**（已锁定 D12）：在 AGENTS.md 加一行"RPC 间一律不互调（含不经 adapter），API 自由调 RPC 聚合"。
9. **CP-5 identifier alias 退役计划**：写一份 V1 schedule。

每步独立可 merge，CI 通过即推 Drone deploy。

---

## 7. 风险

- 三份 logic 的同步漂移意味着**有可能某些 bug 在 internal 修了，但 service 没修**——删之前要 diff 一次，把任何 internal 独有的修复带过去。
- proto 双源里如果 `internal/rpcgen` 的 pb 与 `service/<dom>/rpc/<dom>` 的 pb 字段编号不一致，二进制兼容性会出问题——升级前要做一次 protoc descriptor diff。
- `etc/` 与 `service/<domain>/etc/` 双源化的处理影响生产 yaml，必须在低峰期做且 k8s ConfigMap 同步更新。
