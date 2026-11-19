# 迁移说明：8 个 `*-api` → 单一 `apigateway` + `resource-rpc`

> 状态：代码完成，本地端到端验证通过；线上部署与真实邮件验证待集群可访问后执行。
> 分支：`refactor/gateway-architecture`

## 1. 目标形态

```
客户端 ── HTTPS ──▶ Traefik（TLS / 唯一域名）
                      ├── /ws ──▶ msggateway（WebSocket 长连接 · presence · 下行广播）
                      └── /*  ──▶ apigateway（验签 · jti 会话 · 限流 · 路由）
                                      ├──▶ user-rpc / auth-rpc / friends-rpc / groups-rpc
                                      ├──▶ msg-rpc / agent-rpc / admin-rpc
                                      └──▶ resource-rpc（media + mail）
```

- **入口只有一个 HTTP 进程**：`apigateway`（8080）。原 8 个 `*-api` 的 handler 全部并入其中，
  JWT 验签、jti 会话校验、限流、身份注入都发生在这一个进程里。
- **WebSocket 独立部署**：`msggateway` 仍是单独 Deployment，与 `apigateway` 共享同一套
  JWT/会话语义（同一 secret + 同一 Redis session key）。入口仍是一个域名，由 Traefik 分流。
- **下游只有内部 gRPC**：`*-rpc` 不再对外暴露 HTTP；网关按域路由并透传身份。

### 部署单元变化

| 之前 | 之后 |
|---|---|
| 8 个 `*-api` Deployment | 1 个 `apigateway` |
| `media-rpc` + `third-rpc` | 1 个 `resource-rpc`（一个进程，两个 gRPC service） |
| 22 个 Deployment | 14 个 |

## 2. 服务映射

| 旧服务 | 新位置 | 说明 |
|---|---|---|
| `service/<domain>/api`（8 个） | `service/apigateway/internal/<domain>/{handler,logic,types,config,svc}` | 行为保持不变，仅合并进程 |
| `service/media/rpc` | `service/resource/rpc/internal/media` | 对象存储生命周期 + 批量 presign |
| `service/third/rpc` | `service/resource/rpc/internal/mail` | 腾讯云 SES；未配置凭据时降级为 `FailedPrecondition` |
| `service/*/rpc/*.proto` + 生成代码 | `github.com/wujunhui99/agents_im_protocol` v0.3.0 | 契约仓库（见 §4） |

## 3. 调用边变化（断环）

历史上有两条双向环，均来自“注册时开通默认助手”：

| 环 | 迁移前 | 迁移后 |
|---|---|---|
| `user-rpc ↔ agent-rpc` | user-rpc 同步调 agent-rpc 建默认助手 | user-rpc 只发 `user.created.v1`；agent-rpc 消费后装配 |
| `user-rpc ↔ friends-rpc` | user-rpc 同步调 friends-rpc 建好友 | 由 agent-rpc 调 friends-rpc（方向向下） |

新的边：`agent-rpc → user-rpc / friends-rpc`，`user-rpc` 不再有反向调用。
事件 topic：`user.created.v1`（key = account_id，消费组 `agent-user-created`）。

## 4. 契约仓库

- 仓库：`github.com/wujunhui99/agents_im_protocol`（tag `v0.3.0`）
- 主仓库不再保存 `.proto` 与 `*.pb.go`，`go.mod` 直接依赖契约模块。
- 契约仓库 CI：`make verify`（生成代码漂移检查）+ Rust `cargo check`。
- 变更流程：改 proto → `make gen` → PR → 打 tag → 主仓库 `go get`。
- 多语言：Rust crate 在 `rust/`，由 `tonic-build` 在构建期生成；新增语言只需加生成目标。

## 5. 媒体：presigned 直连对象存储

- `media.v1.GetDownloadURLs`：批量签发（去重、跳过未 ready、上限 200）。
- `msg-rpc.PullMessages` 在属主域内一次签齐图片/文件附件 URL，写入 `Message.media_url` /
  `media_url_expires_at`（TTL 10 分钟）。
- 客户端直接 GET RustFS，不再为每次下载做一次鉴权往返。
- 兼容：`media_url` 为空（无附件或签发失败）时，前端回退 `GET /media/{id}/download-url`。
- 头像仍走 `avatar_url` 字段（历史实现），不在本次范围内改动。

## 6. 部署变更

- Traefik：所有 API 前缀 → `apigateway:8080`；`/ws` → `msggateway:8084`；`/agents-im-media`
  仍直连 RustFS；`/` 仍是 web。
- k8s：新增 `apigateway` Deployment/Service，删除 8 个 `*-api`；`media-rpc` + `third-rpc`
  合并为 `resource-rpc`（端口 9096）。
- 配置：`deploy/k8s/etc/apigateway.yaml` 使用嵌套域配置；`services.json` 决定镜像构建列表。
- 本地：`scripts/dev-up.sh` 只启动一个 `apigateway`；Vite 所有 HTTP 前缀代理到 8080。

## 7. 验证

已执行（本地，`scripts/dev-up.sh` + Vite 5173）：

| 场景 | 结果 |
|---|---|
| 管理员登录（bootstrap） | 200，token 正常 |
| 创建测试账户 + 登录 | 200（`/admin/test-accounts` → `/auth/login`） |
| 单聊发消息 + 拉历史 | 200，seq 正确 |
| 图片：上传意图 → PUT → complete → 发消息 → 拉取 | `media_url` 非空 |
| 用 `media_url` 直接 GET RustFS | 200，`image/png`，字节一致（无二次鉴权） |
| WebSocket 下行 | 连接建立后收到 `message_received` |
| `user.created.v1` | 新账户自动与 `agent_creator` 成为好友 |

自动化：`go build ./...`、`go test ./...`、前端 165 个测试、`make verify` 全绿。

未执行：**真实邮件注册验证**。本地没有腾讯云 SES 凭据，且集群 SSH（9093）暂不可达；
待部署环境可用后用指定邮箱补跑。

## 8. 回滚

每个切片是独立提交，可单独 revert：

```
ff2aedf fix(e2e): ...              # 本地/事件修复
a9ad667 refactor(gateway): ...     # 8 api → apigateway
eba1b5b feat(media): presigned ... # 附件 URL
7b6aa3f refactor(events): ...      # user.created 断环
bca36d4 refactor(contract): ...    # 契约模块
6a2c0c5 refactor(resource): ...    # resource-rpc
```

部署回滚：恢复上一版 `deployments.yaml`/`services.yaml`（8 个 api + media/third）即可，
数据库无破坏性变更（`user.created.v1` 只是新增事件）。

## 9. 后续可选项

- 把网关内的 DTO 拼装继续下沉到 RPC，收敛成真正的“纯路由 + 协议转换”。
- 为 `apigateway` 增加按路由的限流与配额（当前沿用各域原有配置）。
- JWT 换非对称（Ed25519/ES256）+ JWKS，网关只持公钥。
