---
name: retire-internal-domain
description: 把一个业务域从 internal monolith 的 god-package internal/logic 退役、迁入其 owner service，并删除 internal/logic 中该域的生产代码文件。Epic #394 Phase 3/4 逐域复用。
---

# 退役 internal 业务域（按域迁移）

把单个域（media/user/friends/groups/agent/...）的业务逻辑从共享的 `internal/logic`
god-package 退役到它的 owner service，并删除 `internal/logic` 里该域的生产代码文件。
模板来自 media 域（issue #400 / PR，首个跑通）。

## 关键前提与坑（先读）

- **`internal/logic` 是一个 god-package**：很多域的 `*Logic` 混在同一个 `package logic` 里，
  且共享 DTO（如 `UserProfile`）、helper（如 `formatTime`）。删一个文件可能连带影响同包其它域。
- **message monolith 是 keystone**：`internal/servicecontext/message` 与
  `internal/rpcgen/message/internal/svc` 这两个 mega service-context 会 in-process 构造
  几乎所有域的 `*Logic`（media/feedback/groups/user-exists/aihosting/...）。它们在 Phase 6 之前
  一直存在 → **任何被它们消费的域，无法在 Phase 6 之前从 internal/logic 彻底搬走**，
  除非给 monolith 一个过渡替代（见下）。
- **Go `internal/` 可见性**：`service/X/<svc>/internal/...` 只能被 `service/X/<svc>/` 下的代码导入。
  repo-root 的 `internal/`、其它 service、以及 `tests/` 都**不能**导入它。
  → 需要被多方（owner service + 集成测试 + 其它）导入的域逻辑，放 **`service/<domain>/core`**
  （与 `rpc`/`api` 平级，不在 `internal/` 下），而不是 `service/X/rpc/internal/...`。
- **选叶子域先做**：先迁“被依赖最少”的域。判断：
  ```bash
  # 该域 *Logic 是否被 internal/logic 其它文件引用（>1 文件=非叶子）
  grep -rln "\b<Domain>Logic\b" internal/logic/*.go | grep -v "_test.go"
  # 该域是否被 internal monolith（非 logic 包）消费
  grep -rln "\b<Domain>Logic\b" internal/ --include=*.go | grep -vE "internal/logic/|_test.go"
  ```
  media/feedback 是叶子；**user 是被依赖最多的，最后做**（agent_definition/agentlogic/auth/adminbootstrap 都依赖它）。
- **不要 `gofmt -w` 整个目录**：本仓库 import 分组不是 gofmt-canonical（`common/share/*` 排在 `internal/*` 后）。
  `gofmt -w internal/logic/` 会顺手重排几十个无关文件，污染 diff。**只格式化你新建/改动的文件**，
  或重排后用 `git checkout --` 还原纯格式化的无关文件。

## 步骤

### 1. 摸清消费方（务必全量）
```bash
D=media   # 域名小写
grep -rln "NewMediaLogic\|MediaLogic\|MediaObject\|CreateMedia.*Request" --include=*.go . | grep -v _test.go
```
分三类：① owner service（如 media-rpc）；② 其它 service（如 user-rpc 头像）；
③ **internal monolith**（servicecontext/message、rpcgen/message）。

### 2. 迁移 owner 逻辑 → `service/<domain>/core`
```bash
git mv internal/logic/<domain>logic.go service/<domain>/core/<domain>.go
git mv internal/logic/<domain>logic_test.go service/<domain>/core/<domain>_test.go
sed -i '' '1s/^package logic$/package core/' service/<domain>/core/*.go
```
- 补齐该文件用到、但定义在 `internal/logic` 其它文件里的 helper（如 `formatTime`）——复制进 core。
- 删掉只被“非 owner 消费方”用到的方法（如 media 的 `ValidateMessageMedia` 只被 message 用 → 不放 core，
  放过渡包；顺带删掉它独有的 import 如 `encoding/json`）。

### 3. owner service 重新指向 core
owner service 的 logic/svc 文件把 `business "…/internal/logic"` 改成 `…/service/<domain>/core`
（导出名不变时只改 import path；alias 沿用仓库惯例 `business` 或更清晰的域名）。

### 4. 给“无法导入 core”的消费方做过渡替代
- **internal monolith + 其它 service**：因可见性/避免 service 间 in-process 耦合，新建过渡包
  `internal/<domain>validate`（如 `internal/mediavalidate`），把它们真正需要的那一小段校验/读取逻辑
  按 `repository.<Domain>Repository` 重写一份（**不加同步 RPC 跳**，尤其热路径如发消息）。
  在包注释里写明“transitional，Phase 6/数据层迁移时删除”。
- 替换 mega service-context 里的 `logic.New<Domain>Logic(...)` 为过渡校验器；删掉随之无用的 struct 字段。

### 5. 拆测试
- 纯域测试 → 跟去 `service/<domain>/core`（注意 core 不能 import `internal/logic`；
  core 测试里若用了 `assertLogicAppCode` 等 helper，复制一份本地 helper）。
- 跨域集成测试（需要 `internal/logic` 的 MessageLogic 等）→ 留在 `internal/logic`
  （新建 `internal/logic/<domain>_..._test.go`，import `service/<domain>/core`；
  消息校验器用过渡包，因为 core 已不再实现该接口）。

### 6. 删除并自检
```bash
ls internal/logic | grep -i <domain>            # 应无生产代码文件（测试文件可留）
grep -rn "logic\.<Domain>Logic\|logic\.New<Domain>Logic" --include=*.go . | grep -v service/<domain>/core
gofmt -l <你改过的文件...>                        # 仅自检你的文件
go build ./... && go vet ./... && go test ./...   # 全绿
```

### 7. 交付（按 CLAUDE.md 工作流）
issue → worktree（`.claude/worktrees/<branch>`，从 `origin/main`）→ commit → PR →
squash merge（`--delete-branch`；本地 main 被主 checkout 占用导致 gh 报 “main already used by worktree”
是正常的，merge 本身已成功）→ `bash scripts/drone-watch.sh`（后台）→ prod 冒烟 → 更新 Epic #394 勾选。

## 验收清单
- [ ] `internal/logic` 无该域生产代码文件
- [ ] 无任何代码从 `internal/logic` 引用该域符号
- [ ] owner 逻辑在 `service/<domain>/core`，monolith/其它 service 走过渡包或 RPC
- [ ] build/vet/test 全绿；diff 不含无关 gofmt 噪音
- [ ] Drone CI 绿 + prod 冒烟该域关键路径

## 已迁移域（更新此表）
| 域 | PR | owner 落点 | 过渡包 | 备注 |
|----|----|-----------|--------|------|
| media | #400 | `service/media/core` | `internal/mediavalidate`（message 发送校验 + user 头像校验）| 数据层仍用 internal/repository |
