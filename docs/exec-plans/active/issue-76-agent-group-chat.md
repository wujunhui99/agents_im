# Issue #76 Agent Group Chat V1 Plan

Status: Implemented in `feature/issue-76-agent-group-chat`.

## Goal

Enable targeted Agent replies in group conversations without making ordinary group chatter trigger Agent noise.

## Scope

- Use the existing Agent-IM trigger seam with explicit `TargetAgentAccountIDs`.
- Keep free-form text mention parsing out of V1; upstream code can translate mentions into structured target metadata.
- Require target Agents to be active group members before scheduling runtime work.
- Record rejected target attempts as failed Agent trigger idempotency records.
- Keep Agent replies on the normal Message Service writeback path with `message_origin=ai` metadata, conversation seq assignment, and outbox events.
- Preserve default loop prevention for AI-origin group messages.

## TDD Checkpoints

- Red: add `internal/agentim` tests for targeted group trigger, untargeted group chatter, AI-origin loop prevention, non-member target rejection, and Message Service writeback metadata.
- Green: add a narrow group-member lookup dependency to `ConversationHostingService`, validate group targets before scheduling, and record invalid targets as failed triggers.
- Refactor: keep direct AI hosting and single-chat trigger paths unchanged.

## Verification

Required final verification follows Issue #76:

```bash
export PATH=/tmp/go/bin:$HOME/go/bin:$PATH
for f in api/*.api; do goctl api validate -api "$f"; done
go test ./internal/agentim ./internal/agentruntime ./internal/repository ./tests
go test ./...
bash scripts/verify-static.sh
git diff --check
```
