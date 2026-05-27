# Issue #289: Move Agent API/RPC into service layout

## Goal

Move Agent service code into the current go-zero service layout:

- `service/agent/api`
- `service/agent/rpc`

The migration should follow `service/user` and `service/friends` conventions while preserving existing Agent behavior.

## Scope

- Move/copy Agent REST API contract and generated API code under `service/agent/api`.
- Move/copy Agent RPC/proto code under `service/agent/rpc` if an Agent RPC contract exists; if there is no Agent RPC yet, document/verify that `service/agent/rpc` is intentionally not created as fake scaffold.
- Keep `cmd/agent-api` as a thin entry wrapper that calls `service/agent/api/entry.Start`.
- Update imports, Docker/deploy/CI/static checks, docs references from old root Agent paths to the new service layout.
- Do not change Agent product behavior, persistence semantics, runtime safety boundaries, or tool execution capability.

## Required reading

- `AGENTS.md`
- `.ai-context/zero-skills/SKILL.md`
- `.ai-context/zero-skills/references/goctl-commands.md`
- `.ai-context/zero-skills/references/rest-api-patterns.md`
- `.ai-context/zero-skills/references/rpc-patterns.md`
- `docs/design-docs/go-zero-service-layout.md`
- `docs/design-docs/agent-system-architecture.md`
- `docs/design-docs/im-agent-contract.md`

## Implementation plan

1. Inspect existing `service/user` and `service/friends` layouts and current Agent paths.
2. Establish Agent API canonical source under `service/agent/api` using goctl-generated layout style.
3. Add `service/agent/api/entry/entry.go`; make `cmd/agent-api/main.go` a thin wrapper.
4. Search/fix all imports and path references to canonical Agent API source.
5. Determine whether a real Agent RPC exists. If none, do **not** create fake RPC code just to satisfy a directory shape; document that only API moved in this PR.
6. Run verification gates.
7. Commit with Achilles identity and required trailers; push, PR to `main`, enqueue via Merge Queue after CI is green.

## Verification gates

Minimum:

```bash
export PATH=/tmp/go/bin:$HOME/go/bin:$PATH
gofmt -w <changed-go-files>
git diff --check
go test ./...
bash scripts/verify-static.sh
```

If API specs changed, validate the canonical API spec:

```bash
goctl api validate -api service/agent/api/agent.api
```

If DB schema/repository SQL changes appear unexpectedly, run PostgreSQL integration or report the blocker explicitly.
