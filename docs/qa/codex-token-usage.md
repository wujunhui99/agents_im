# Codex task token usage log

This document records token consumption for Codex-delegated tasks in `agents_im`.

Policy:

- Every completed Codex task must add one row before the branch is considered ready to merge.
- Record `input tokens`, `output tokens`, and `total tokens` when Codex exposes them.
- If the Codex CLI only prints total usage, record input/output as `unavailable` instead of guessing.
- Do not include prompts that contain secrets, real tokens, passwords, cookies, Authorization headers, or connection strings.
- Keep evidence paths local-only when they point to `/tmp`; do not commit generated evidence.

## Entries

- Date: 2026-05-03
  - Task: WebSocket live-push E2E regression harness
  - Branch: `feature/ws-live-push-e2e-regression`
  - Commit: `809327f test(ws): add live push e2e regression harness`
  - Codex session: `proc_01ca8cec18d4`
  - Input tokens: unavailable
  - Output tokens: unavailable
  - Total tokens: 144,922
  - Notes: Codex CLI output showed `tokens used 144,922` but did not expose input/output split in the captured log.
