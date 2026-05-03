# Friend Request Approval Flow

状态：Completed

## 背景

当前 friends 服务的 `AddFriend` 会直接创建双向有效好友关系；前端已有部分 `pending/accepted` 兼容展示，但后端没有真实审批流。

## 目标

- `AddFriend` 创建 `pending` 申请，不进入普通好友列表。
- 只有申请接收方可以 `accept` 或 `reject` 待处理申请。
- 接受后双方好友列表都返回 `accepted` 关系；拒绝后不返回为好友。
- REST、RPC、前端 API、联系人页 UI、测试和产品契约同步更新。
- 兼容历史 `active` 状态读取，但新行为使用 `pending/accepted/rejected`。

## 非目标

- 不实现好友通知推送、社交收件箱、好友备注、分组或黑名单。
- 不改变消息服务是否强制校验好友关系的现有边界。

## 任务拆分

- [x] 更新 friendship 状态模型、内存 repository 和 PostgreSQL repository。
- [x] 更新 FriendsLogic，增加申请列表、接受、拒绝业务能力。
- [x] 更新 `.api` / `.proto`、REST handler、RPC generated scaffold 和转换代码。
- [x] 更新 ContactsPage API/types/UI，展示 incoming/outgoing pending requests 并支持接受/拒绝。
- [x] 更新后端、前端测试和本地 demo/e2e 辅助脚本。
- [x] 更新产品/设计/交接文档并记录验证结果。

## 决策日志

| 时间 | 决策 | 原因 |
| --- | --- | --- |
| 2026-05-03 | PostgreSQL status `1` 继续表示已接受关系，对外返回 `accepted`，保留 `active` 仅作 legacy 兼容。 | 避免已有 `active` 数据在迁移后被误读为 pending。 |
| 2026-05-03 | Pending 申请只写入 `requester -> recipient` 一条方向；列表 incoming 时按 `friend_account_id` 查询并转换成当前用户视角。 | 不新增 schema 字段也能可靠判断接收方，防止请求方越权接受/拒绝。 |

## 验证方式

- `go test ./...`
- `npm --prefix web test -- --run --reporter=dot`
- `npm --prefix web run build`
- `bash scripts/verify-static.sh`
- `git diff --check`

## 风险与回滚

- 风险：已有测试或 demo 假设添加后立即互为好友，需要同步改为先接受。
- 风险：前端新增申请列表会增加 `/friends/requests` 调用，相关测试需要显式 mock。
- 回滚：恢复 AddFriend 的双向 accepted 写入并移除新增 accept/reject/list requests contract。

## 结果记录

完成：

- `AddFriend` 改为创建单向 `pending` 申请；普通好友列表只返回 `accepted`/legacy `active` 兼容关系。
- 新增 REST/RPC 能力：查询 incoming/outgoing 好友申请、接受申请、拒绝申请。
- 接受申请只允许 pending 接收方操作，成功后写入双向 `accepted`；拒绝申请只允许接收方操作，成功后返回 `rejected` 且不进入好友列表。
- ContactsPage 增加好友申请区域，支持接受/拒绝 incoming 请求，并展示 outgoing 等待确认状态。
- demo/e2e 脚本改为 add 后由接收方 accept。

验证结果：

- `PATH=/tmp/go/bin:$HOME/go/bin:$PATH go test ./...`：通过。
- `npm --prefix web test -- --run --reporter=dot`：通过，10 个 test files / 46 个 tests。
- `npm --prefix web run build`：通过。
- `npm --prefix web run lint`：通过。
- `bash scripts/verify-static.sh`：通过。
- `git diff --check`：通过。
