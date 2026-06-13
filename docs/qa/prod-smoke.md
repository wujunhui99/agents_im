# 生产冒烟固定账户与流程

适用场景：需要在生产环境做端到端冒烟（注册态账户已就绪），验证好友/消息/ACK/seq/落库/push 链路时读取本文。

prod 冒烟**固定使用**两个 test 账户（不要新建账户）：`smokeb3a1`（accountId `323499294365372416`）、`smokeb3a2`（accountId `323499427278671872`）。密码不落任何文档/日志：每次冒烟先用 admin 账号（k8s secret `agents-im-secrets` 的 `ADMIN_BOOTSTRAP_*`）登录，再 `POST /admin/test-accounts {"identifier":"smokeb3a1"}` 重置并取回一次性密码（对已存在 test 账户该接口语义=重置密码，响应 `alreadyExisted=true`）。

标准流程（每次按此顺序，结束时清理好友关系）：

1. smokeb3a1 → `POST /friends` 加 smokeb3a2，smokeb3a2 → `POST /friends/requests/:user_id/accept` 接受；
2. smokeb3a1 → `POST /messages`（single chat）发消息，按任务验证 ACK/seq/落库/push；
3. smokeb3a1 → `DELETE /friends/:user_id` 删除好友收尾。

入口域名：`/auth/*`、`/admin/test-accounts` 走 `ms.agenticim.xyz`；`/messages`、`/conversations/*`、`/friends/*` 走 `agenticim.xyz`（ms 域名的 `/` 落到 web 静态服务，POST 会被 nginx 405）。集群访问方式见 [`deploy/README.md`](../../deploy/README.md)。
