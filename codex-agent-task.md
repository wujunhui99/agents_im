
You are a Codex agent working in github.com/wujunhui99/agents_im. Follow Planner -> Generator -> Evaluator.

MANDATORY READ FIRST:
- AGENTS.md
- ARCHITECTURE.md
- docs/product-specs/agent-system.md
- docs/design-docs/agent-system-architecture.md
- docs/exec-plans/active/agent-system-v0.md
- docs/exec-plans/active/agent-infrastructure-parallel-baseline.md
- .ai-context/zero-skills/SKILL.md
- .ai-context/zero-skills/references/goctl-commands.md
- .ai-context/zero-skills/references/rest-api-patterns.md
- .ai-context/zero-skills/references/rpc-patterns.md
- .ai-context/zero-skills/references/database-patterns.md

GLOBAL RULES:
- Strict fail-first: visible errors are better than hidden incorrect success. Do not swallow errors, fake success, or silently fallback to mock.
- No fake implementation: no mock/stub/hardcoded empty success in real code. Test fakes/in-memory fixtures are allowed only if clearly test-scoped.
- TDD required: write/adjust tests first, run them to see the expected failure, then implement, then prove green.
- LLM execution is explicitly out of scope. Do not add any real LLM provider/framework integration.
- No shell/command execution capability for agents. Python execution must be sandbox-contract only unless your task says otherwise; never directly run arbitrary Python from the Go service process.
- Do not commit secrets/tokens/credentials. Use placeholders only.
- Keep default go test ./... independent of external services; integration tests requiring PG/Redis/Kafka/MinIO must skip unless env vars are set.
- Use go-zero/goctl spec-first where adding REST/RPC surfaces. Keep business logic in internal/logic and dependencies in ServiceContext.
- Update relevant docs and exec plan notes.

REQUIRED VALIDATION before final commit/push:
export PATH=/tmp/go/bin:$HOME/go/bin:$PATH
goctl --version
for f in api/*.api; do goctl api validate -api "$f"; done
gofmt -w $(find . -name '*.go' -print)
go test ./...
bash scripts/verify-static.sh
docker compose config
git diff --check

At the end: commit all changes with the requested conventional commit message and push your feature branch to origin. If blocked, fail explicitly with root cause and do not fake success.

BRANCH: feature/agent-prompts-tools-skills
COMMIT MESSAGE: feat(agent): add prompt tool and skill registries

TASK: Implement prompt/tool/skill metadata and binding registries.
Scope:
- System prompts: content, version, status, created_by, timestamps.
- Tools: type mcp/local/builtin only; MCP server ref/config metadata; local handler_key only; builtin key. Reject shell/command/script execution types.
- Skills: metadata, version, object_key, sha256, content_type, size, status. PG stores metadata only, not file content.
- Binding tables/repositories/logic for agent_prompt_bindings, agent_tool_bindings, agent_skill_bindings.
- MCP tools first version admin-configured and Agent whitelist binding only; encode validation fields/logic/tests.
- Update docs.
Do NOT implement MinIO binary upload beyond metadata contract unless minimal config placeholders are already present; do NOT execute tools or LLM.
