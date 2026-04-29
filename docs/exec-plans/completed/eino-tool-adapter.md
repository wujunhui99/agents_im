# Eino Tool Adapter Contract

Status: Completed

## Background

The Agent registry can store MCP, local, and builtin tool metadata plus per-Agent tool bindings. The Agent runtime still needs a fail-closed contract that resolves Eino-bindable tool specs from those registry rows without letting global tool metadata bypass the Agent binding whitelist.

## Goals

- Add a runtime tool resolver package for per-Agent allowed tool specs.
- Enforce active/admin-configured tool policy, Agent tool binding whitelist, and safe MCP server transport policy at resolution time.
- Expose adapter/provider interfaces that an Eino runtime can depend on without importing Eino from business logic.
- Keep V0 metadata-only for MCP/local/builtin tools unless a safe adapter is explicitly registered.

## Non-Goals

- No real MCP network calls.
- No Eino ChatModelAgent orchestration.
- No DeepSeek/model dependency changes.
- No shell, local process, stdio MCP, Python, or filesystem mutation tools.

## Task Breakdown

- [x] Add repository list binding support needed to resolve all tools for an Agent.
- [x] Add `internal/agentruntime/tools` contracts and resolver.
- [x] Add fail-closed unit tests for whitelist and unsafe metadata.
- [x] Update Agent architecture docs with the runtime resolver boundary.
- [x] Run required verification and record results.

## Decision Log

| Time | Decision | Reason |
| --- | --- | --- |
| 2026-04-30 | Keep Eino out of the package API. | Eino is an adapter implementation detail; business runtime code should depend on local contracts. |
| 2026-04-30 | Resolve metadata and adapters separately. | This lets V0 expose safe specs while refusing execution unless a safe adapter is explicitly registered. |

## Verification

```bash
export PATH=/tmp/go/bin:$HOME/go/bin:$PATH
for f in api/*.api; do goctl api validate -api "$f"; done
gofmt -w $(find . -path './web/node_modules' -prune -o -name '*.go' -print)
go test ./...
bash scripts/verify-static.sh
docker compose config
git diff --check
```

## Risks And Rollback

- Risk: resolver silently drops unsafe bound tools. Mitigation: resolution returns an explicit error for any requested or bound unsafe tool.
- Risk: future runtime binds metadata-only tools as callable tools. Mitigation: `RequireAdapters` mode fails unless a safe adapter is present.

## Result Record

- Added `internal/agentruntime/tools` with `Provider`, `Resolver`, `ToolSpec`, `ToolAdapter`, adapter catalog, and optional resolution audit hook contracts.
- Added repository support for listing Agent tool bindings in memory and PostgreSQL implementations.
- Runtime resolution now enforces Agent binding whitelist, active/admin-configured tool metadata, active/admin-configured MCP servers, safe remote MCP transports only, and rejection of stdio/local-process command-like MCP config metadata.
- Local/builtin tools can resolve as metadata, but `RequireAdapters=true` fails unless an explicit safe adapter is registered.
- Verification passed on 2026-04-30 with the commands listed above.
