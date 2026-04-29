# Frontend Shell Implementation Plan

> **For Hermes/Codex:** use test-driven-development for behavior changes and keep the feature on `feature/frontend-shell` until verified.

**Goal:** Build the first Web frontend shell for Agents IM, referencing WeChat's four-tab information architecture.

**Architecture:** Add a `web/` React + TypeScript + Vite app inside the existing repository. Keep this first slice focused on navigation, layout, and mock data; API and WebSocket integration will follow the existing frontend-backend handoff contract in later features.

**Tech Stack:** React, TypeScript, Vite, Vitest, Testing Library, lucide-react, native CSS.

---

## Tasks

### Task 1: Create frontend workspace

- Add root npm helper scripts.
- Add `web/package.json`, Vite config, TypeScript config, HTML entry, and test setup.
- Install dependencies with npm.

### Task 2: Test the WeChat-style four-tab shell

- Add `web/src/App.test.tsx`.
- Verify the four primary tabs exist: `消息`, `联系人`, `发现`, `我的`.
- Verify bottom navigation switches page headings and representative page content.

### Task 3: Implement the shell

- Add `web/src/App.tsx`, `web/src/main.tsx`, and `web/src/styles.css`.
- Implement mock conversation, contacts, discover, and profile page content.
- Keep auth/API/WebSocket wiring out of this slice.

### Task 4: Document frontend conventions

- Update `docs/FRONTEND.md` with stack, current scope, commands, and design rules.
- Update `scripts/verify-static.sh` to require the frontend shell files and core tab labels.

### Task 5: Verify and push

Run:

```bash
npm run frontend:test
npm run frontend:build
npm run frontend:lint
PATH=/tmp/go/bin:$HOME/go/bin:$PATH go test ./...
bash scripts/verify-static.sh
python3 - <<'PY'
# Markdown link check helper, excluding docs/references if needed.
PY
```

Commit with:

```bash
git add package.json package-lock.json web docs/FRONTEND.md docs/exec-plans/active/frontend-shell.md scripts/verify-static.sh
git commit -m "feat(frontend): add wechat-style app shell"
git push -u origin feature/frontend-shell
```
