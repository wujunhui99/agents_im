# Issue 140 implementation plan: agent.create tool for AI 助手

## Goal

Implement the product behavior from GitHub issue #140: the default `agent_creator` / `AI 助手` can create database-driven Agents through a controlled local `agent.create` tool. New Agents are active and usable by default; newly created Agents must not receive `agent.create` or `python.execute` by default.

## Requirements snapshot

- Reuse existing account/profile, `agents`, prompt registry, and tool registry models.
- Add or finish `agent.create` tool provisioning/binding for the default assistant.
- Bind create-agent permission only to `agent_creator` by default.
- Default created Agent status: `active`.
- Minimal input: name + description; service/assistant may generate prompt and safe tools.
- Created Agent must not get `agent.create`, `python.execute`, shell/command/script, external MCP/network tools by default.
- If existing friendship/contact logic supports it, create accepted friendship between requesting user and created Agent.
- Fail visibly and avoid half-created state.

## Proposed implementation steps

1. Inspect existing model/repository/logic:
   - `internal/logic/default_assistant.go`
   - `internal/logic/agentlogic.go`
   - registry logic/repository for prompts/tools/bindings
   - user/profile/account creation logic
   - friendship repository/logic
   - runtime tool resolver and adapter naming.
2. Add tests first:
   - default assistant provisioning creates/binds `agent.create` only to agent_creator;
   - created Agent defaults `active`;
   - created Agent gets prompt binding and safe tool bindings;
   - created Agent does not get `agent.create` or `python.execute`;
   - if supported, requester and created Agent become accepted friends.
3. Implement service logic for controlled Agent creation, preferably in `internal/logic` as a small orchestration type around existing repositories/logic.
4. Add provisioning/migration for global `agent.create` tool metadata and default assistant binding. Preserve safe runtime alias if needed (`agent_create` -> handler key `agent.create`).
5. Update default assistant system prompt to describe agent creation behavior and constraints.
6. Update docs/design notes if behavior changes from existing draft.
7. Run verification gates.

## Verification commands

```bash
export PATH=/tmp/go/bin:$HOME/go/bin:$PATH
gofmt -w $(find . -name '*.go' -print)
go test ./internal/logic ./internal/agentruntime/tools ./internal/repository -count=1
go test ./...
bash scripts/verify-static.sh
git diff --check
```

For SQL/repository changes when local PostgreSQL is available:

```bash
AGENTS_IM_CONFIRM_TRUNCATE=1 scripts/verify-postgres-local.sh
```

## Codex implementation notes

- `agent.create` now accepts the minimal name + description intent and generates a bounded system prompt when `system_prompt` is omitted.
- Runtime calls pass the creator Agent ID into service logic; service logic only allows the active canonical `agent_creator` / `AI 助手` Agent to create Agents.
- New Agents are created active, receive only default low-risk readable context tools unless explicitly supplied safe tools are requested, and never receive `agent.create` or `python.execute` by default.
