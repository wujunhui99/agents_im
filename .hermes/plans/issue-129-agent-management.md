# Agent management and agent.create implementation plan

> **For Hermes:** Use Codex as worker; Hermes reviews diff/tests and integrates to develop.

**Goal:** Add backend foundation for Agent management/assembly and let the built-in `agent_creator` create new agents via a controlled local tool.

**Architecture:** Reuse the existing Agent registry tables (`agent_prompts`, `agent_tools`, `agent_prompt_bindings`, `agent_tool_bindings`) instead of inventing a parallel model. Add Agent Definition logic/API for system prompt + bound tools. Add a whitelisted local tool `agent.create` bound only to `agent_creator`; the tool calls server-side business logic to create an agent account/profile, create agent metadata, bind prompt/tools, and add the requesting user as an accepted friend.

**Tech Stack:** Go, go-zero API, PostgreSQL migrations, existing repository abstractions, existing `internal/agentruntime/tools` adapter catalog.

---

## Scope

1. Agent Definition backend contract:
   - return an agent with its system prompt and tools;
   - update/assemble an agent definition using existing prompt/tool registries;
   - V1 supports one active system prompt.
2. Tool registry improvements:
   - register `agent.create` as a local whitelisted tool;
   - list active tools for management/runtime use;
   - keep DB metadata-only; no script/shell/command tools.
3. `agent.create` local tool:
   - bound only to `agent_creator` by default;
   - creates a new `account_type=agent` account/profile;
   - creates `agents` row;
   - creates/binds system prompt;
   - binds requested allowlisted tools;
   - auto-adds current human user as accepted friend of the new agent.
4. Tests and docs:
   - unit tests for definition assembly and agent.create;
   - migration/change log for schema/default tool data;
   - update Agent system architecture docs.

## Non-goals

- Do not add arbitrary shell/script/command execution.
- Do not let normal users register MCP servers or local handler keys.
- Do not enable `python.execute` for arbitrary new agents by default.
- Do not implement frontend UI in this PR.
- Do not require Kubernetes/Python executor availability.

## Acceptance Criteria

- `agent_creator` has an active `agent.create` local tool binding.
- `agent.create` creates a new agent and an accepted friendship with the requesting user.
- Created agent has a system prompt and only allowed tools are bound.
- Runtime/tool resolver still fails closed for unbound or inactive tools.
- Tests pass:
  - targeted agent/registry/runtime tests;
  - `go test ./...`;
  - `bash scripts/verify-static.sh`;
  - `git diff --check`.

## Implementation tasks

### Task 1: Repository/model support

- Add list-by-name/list-active helpers required by Agent Definition and available tools.
- If adding per-agent tool binding config/policy is too invasive, keep V1 binding unchanged and document V2 fields.
- Ensure PostgreSQL and memory repositories stay consistent.

### Task 2: Agent Definition logic/API

- Add logic structs for `AgentDefinition`, `PromptDefinition`, `ToolDefinition`.
- Implement `GetAgentDefinition(agent_id)`.
- Implement `UpdateAgentDefinition(agent_id, system_prompt, tool_names, updated_by)` with V1 single system prompt semantics.
- Add go-zero API routes if straightforward:
  - `GET /agents/:agent_id/definition`
  - `PUT /agents/:agent_id/definition`

### Task 3: agent.create local tool

- Add local handler key constant `agent.create`.
- Add `AgentCreateAdapter` in `internal/agentruntime/tools` or equivalent package.
- Adapter input schema includes name, description, system_prompt, tool_names.
- Adapter must require requesting user id and call business logic; it must not mutate DB directly if a logic/service layer exists.
- Add allowlist for default creatable tools, initially low risk only:
  - `im.get_conversation_context` / existing read-context equivalent if registered;
  - optional `skill.read_file` only if already supported safely.
- Do not allow `python.execute`, MCP external tools, or `im.send_agent_message` by default.

### Task 4: Default agent_creator provisioning

- Extend default assistant provisioning to register and bind `agent.create` to `agent_creator`.
- Update prompt text so agent_creator knows it can create agents through the tool.
- Keep `python.execute` binding behavior unchanged if already present.

### Task 5: Tests and docs

- Add tests for:
  - default assistant binds `agent.create`;
  - `agent.create` creates agent account/profile/agent/prompt/tool bindings/friendship;
  - disallowed high-risk tools are rejected;
  - definition reads back prompt + tools.
- Update `docs/design-docs/agent-system-architecture.md` with final tool management design.
- Add SQL migration/change log for default `agent.create` tool registration/binding.

## Verification

Run:

```bash
export PATH=/tmp/go/bin:$HOME/go/bin:$PATH
gofmt -w $(find internal -name '*.go' -print)
go test ./internal/logic ./internal/agentruntime/tools ./internal/repository ./tests -count=1
go test ./...
bash scripts/verify-static.sh
git diff --check
```
