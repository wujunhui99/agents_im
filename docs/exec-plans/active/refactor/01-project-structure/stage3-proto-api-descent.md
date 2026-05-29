# Stage 3 — proto/api 下沉 + gozero_routes 退役

> 前置：Stage 1、Stage 2 完成  
> 后置：Stage 4 各域迁移可以开始  
> 风险：中（需重新 protoc 生成，改 go_package，更新所有 import）  
> 预计 PR 数：2（步骤 7+8 一个 PR，步骤 9 一个 PR）

---

## 目标

- proto/api 文件单源下沉到 `service/<domain>/`，删根 `proto/` 和 `api/`
- 退役单体路由聚合器 `internal/handler/gozero_routes.go`

---

## 步骤

### 步骤 7 — proto 下沉到 service

根 `proto/` 还存活的文件：`friends.proto`、`groups.proto`、`message.proto`（auth/user/mail 已在 service 下有单源）。

```bash
# friends
git mv proto/friends.proto service/friends/rpc/friends.proto

# groups
git mv proto/groups.proto service/groups/rpc/groups.proto

# message（message 服务尚未建 service/，先建目录占位）
mkdir -p service/message/rpc
git mv proto/message.proto service/message/rpc/message.proto
```

各 proto 改 `option go_package`：

```proto
// friends.proto
option go_package = "github.com/wujunhui99/agents_im/service/friends/rpc/friendspb";

// groups.proto
option go_package = "github.com/wujunhui99/agents_im/service/groups/rpc/groupspb";

// message.proto
option go_package = "github.com/wujunhui99/agents_im/service/message/rpc/messagepb";
```

重新生成 pb：

```bash
cd service/friends/rpc && goctl rpc protoc friends.proto --go_out=. --go-grpc_out=. --zrpc_out=.
cd service/groups/rpc  && goctl rpc protoc groups.proto  --go_out=. --go-grpc_out=. --zrpc_out=.
cd service/message/rpc && goctl rpc protoc message.proto --go_out=. --go-grpc_out=. --zrpc_out=.
```

更新 import path（从 `internal/rpcgen/` 到 `service/<domain>/rpc/`）：

```bash
MODULE="github.com/wujunhui99/agents_im"
find . -type f -name '*.go' \
  | xargs sed -i '' "s|${MODULE}/internal/rpcgen/friends|${MODULE}/service/friends/rpc/friendsclient|g"
find . -type f -name '*.go' \
  | xargs sed -i '' "s|${MODULE}/internal/rpcgen/groups|${MODULE}/service/groups/rpc/groupsclient|g"
# message 的 rpcgen 引用暂不替换，等 Stage 4b message 服务完整迁移时一并处理
```

删根 `proto/`（message.proto 已移走，确认目录为空）：

```bash
test -z "$(ls proto/ 2>/dev/null)" && git rm -r proto/ || echo "proto/ still has files, check manually"
```

---

### 步骤 8 — api 下沉到 service

根 `api/` 的 `.api` 文件下沉到各 service：

```bash
# 找到所有根 api/ 文件
ls api/

# 逐一 mv（以 admin.api 为例，其他类推）
git mv api/admin.api service/admin/api/admin.api   # admin 服务 Stage 4a 建，先建目录占位
# message.api → Stage 4b 时迁
# 其他已在 service/ 下的 .api 文件确认单源后，删根 api/ 副本
```

删根 `api/`：

```bash
test -z "$(ls api/ 2>/dev/null)" && git rm -r api/ || echo "api/ still has files"
```

---

### 步骤 9 — `internal/handler/gozero_routes.go` 退役

该文件目前为 message-api 注册多域路由。退役步骤：

1. 确认 auth/user/friends/groups 的路由已全部走 `service/<domain>/api/entry`，在该文件中的注册函数无人调用：
   ```bash
   grep -n 'RegisterAuthGoZeroHandlers\|RegisterUserGoZeroHandlers\|RegisterFriendsGoZeroHandlers\|RegisterGroupsGoZeroHandlers' \
     --include='*.go' -r .
   ```

2. 仅保留 message/admin 相关路由函数（等 Stage 4a/4b 迁走后彻底删文件）：
   - 删除已迁域的注册函数
   - 确保 `cmd/message-api/main.go` 不再调用被删函数

3. Stage 4a（admin）和 Stage 4b（message）完成后，`gozero_routes.go` 应完全为空，届时：
   ```bash
   git rm internal/handler/gozero_routes.go
   # 若 internal/handler/ 目录此时为空
   git rm -r internal/handler/
   ```

---

## 验收

```bash
# 根 proto/ 和 api/ 已删
test ! -d proto && test ! -d api

# proto 单源在 service/
test -f service/friends/rpc/friends.proto
test -f service/groups/rpc/groups.proto
test -f service/message/rpc/message.proto

# 无旧 rpcgen/friends、rpcgen/groups import（message 暂豁免）
! grep -r 'agents_im/internal/rpcgen/friends\|agents_im/internal/rpcgen/groups' --include='*.go' .

go build ./...
```
