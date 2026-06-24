# Handoff — #609 删除 internal/ 中仅测试引用的 prod-dead 代码

> 给下一个接手的 agent。本文档不进 PR（清理时删除），只为交接。
> 分支：`refactor/claude/issue-609-drop-test-only-internal`，PR **#610**，Issue **#609**（Refs #394/#344）。
> worktree：`.claude/worktrees/drop-test-only-internal`。

## 目标与背景
EPIC #394/#344 退役顶层 `internal/`。本 PR 只做**增量死代码清理**：删除 `internal/logic`、`internal/repository` 中
**生产不可达、仅被测试引用**的代码。用 `deadcode ./...`（不带 `-test`）定位，初始报告 **87** 个 internal 函数 prod-unreachable。

工具：`go install golang.org/x/tools/cmd/deadcode@latest`，跑 `deadcode ./...`（生产不可达）vs `deadcode -test ./...`（含测试也不可达=真死）。
环境：`export PATH=/usr/local/go/bin:$(go env GOPATH)/bin:$PATH GOTOOLCHAIN=local`（系统 `go` 是坏的 1.25 wrapper）。

## 关键教训：CI 契约标记守卫（最重要）
`scripts/verify/verify-contract-markers.sh`（CI backend-verification 步骤跑）用 `rg -q` 断言一批
**安全/契约标记必须存在**。退役 internal 时**很容易踩它**——它正是为防重构静默丢覆盖而存在。
第一轮全量删除导致 CI build #173 红，命中 3 处守卫：
- `bearerTokenForUser`（在 `tests/`）—— JWT 鉴权测试 helper
- `TestMessageGroupSendRequiresActiveMembership` + `client-group-outsider` + `client-group-left`（在 `tests/`）—— 群发鉴权（外人→FORBIDDEN，成员→OK）
- `NewMediaRepositoryForStorage`（line 235，在 internal/repository）—— media 存储工厂

> 退任何 internal 代码前，先 `grep <symbol> scripts/verify/verify-contract-markers.sh`。守卫命中就要么保留代码，要么迁移契约 + 同步改守卫。

## 用户决策
用户选了 **「安全子集先合，迁移后做」**：保留无歧义的安全删除，**回退**契约守卫相关的删除，发一个绿色 PR，迁移留作后续。

## 本 PR 当前状态（已落地）
### 保留的删除（安全 + 已处理守卫）
- **media repo 整套**（已迁 service/media，media-rpc 自带 goctl `MediaObjectsModel`）：删 `media_repository.go`、`media_memory.go`、`postgres_media.go`、`postgres_media_test.go`；`postgres_common.go` 删 `NewMediaRepositoryForStorage`；`schema_v2_enums.go` 删 media 枚举。
  → **守卫 line 235 已 repoint** 到 `NewMediaObjectsModel`（service/media service_context.go）。
- **task-report repo 整套**（已迁 service/admin goctl）：删 `task_report_repository.go`、`postgres_task_report.go`、`postgres_task_report_test.go`；`postgres_common.go` 删 `NewTaskReportRepositoryForStorage`。无守卫。
- 死工厂/helper：`postgres_common.go` 删 `NewRepositoryForStorage`（仅 storage_factory_test 用，已同步改测试）；`postgres_feedback.go` 删 `NewFeedbackRepositoryForStorage`；`postgres_groups.go` 删 `insertGroupMember`；`message_storage_contract.go` 删 4 个死 `MessageStorage*`（`SingleConversationID`/`GroupConversationID`/`UnreadCount`/`AdvanceReadSeq`，保留 live 的 `OrderedSingleUsers`/`UnreadCountFromVisibleStart`）；`schema_v2_enums.go` 删 `memberRoleToDB`。均无守卫。

### 回退的删除（契约守卫保护，**本 PR 不动**）
从 `origin/main` 恢复了：`internal/logic/userlogic.go`(+`_batch_test`)、`internal/repository/memory.go`、
`internal/logic/groupslogic.go`、`internal/logic/messagelogic.go`(+`messagelogic_test`、`message_media_test`)、
`tests/{message_service_test,gozero_test_helpers,postgres_persistence_integration_test}.go`，
以及 5 个 service/agent orchestrator/aihosting 测试（曾被改去用 `NewMessageLogicWithMediaValidator`，一并恢复）。
> 注意：恢复 memory.go 会带回 `normalizeAdminLimit`，所以**不要**再往 postgres_common.go 加它（已撤回）。

## 验证（在 worktree，限额前最后状态）
- `go build ./...` 绿、`go vet ./internal/... ./tests/...` 绿
- `bash scripts/verify/verify-contract-markers.sh` → 修 line 235 后预期绿（恢复文件已补回 bearerTokenForUser/membership 标记）
- `go test ./internal/... ./service/{agent,msg,admin}/... ./tests/...` → 提交时跑（见 /tmp/t4.log）
- `deadcode ./...` 此刻 internal/ 仍有剩余 prod-dead（被回退代码占大头），**这是预期**——它们被契约测试钉住，不是本 PR 范围。

## 收尾步骤（如限额中断，下个 agent 接力）
1. 删除本文件 `HANDOFF-issue-609.md`。
2. `git fetch origin` 紧贴 rebase 校验新鲜（AGENTS.md 失败优先），rebase `origin/main`。
3. 确认 markers/build/test 全绿后 `git commit --amend`（或新 commit）+ `git push -f`（已有 PR #610）。
4. 后台 `scripts/drone-watch.sh`（token 在主仓库 `secret/drone_token`，**worktree 没有**，从主仓库目录跑或软链）报告 Drone。
5. 在 #609 评论实现方式。

## 后续迁移工作（**新 issue，本 PR 之外**）
要真正退役被回退的代码，需把契约迁到 owner service 并同步改守卫：
1. **群发鉴权**：在 `service/msg/rpc/internal/logic` 写测试覆盖 `SendMessageLogic.resolveGroupParticipants`
   （外人→`apperror.Forbidden("sender is not a group member")`，成员→OK）。注意 `SendMessageLogic` 需整个 `svc.ServiceContext`
   （`model` store + `Groups`=internal business.GroupsLogic keystone），现有 `roundtrip_integration_test.go` 是 `//go:build integration` 要真 PG。
   迁好后把守卫 line 217-218 的 `tests` 改指 service/msg 测试。
2. **JWT bearer**：`bearerTokenForUser` 已在 `service/agent/api/agent_http_test.go:145` 存在；把守卫 line 202-203 从 `tests` 改指 service/agent/api，然后可删 `tests/gozero_test_helpers.go`。
3. 之后 `UserLogic`、account/friend `MemoryRepository`、message 死构造器、groupslogic `UserProfileLookup` 富集路径
   （两处 `NewGroupsLogic` 生产均传 nil userExists，富集分支生产不可达）才能安全删，并删对应 legacy `tests/`。
4. `internal/` 其余仍 live（被 msg/agent/admin/auth/user keystone import）：logic/{messagelogic,groupslogic,feedbacklogic,agentauditlogic}、postgres account/friend/message/agent-audit/feedback repo、servicecontext/common——属 Phase 3-6 正式迁移，非死代码。
