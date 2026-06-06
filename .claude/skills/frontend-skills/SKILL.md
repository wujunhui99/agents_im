---
name: frontend-skills
description: WeChat 风格 React IM 前端（web/，React+TS+Vite+Vitest）开发知识包。改 Agents IM 前端 UI/交互、加页面或组件、写前端测试、对接 REST/WebSocket 契约前先读。
---

# Frontend Skill: WeChat-style React IM UI

改动 Agents IM 前端（`web/`）前先读本知识包。

## Product direction

- Build a WeChat-inspired IM web app.
- The top-level app has four tabs: `消息`, `联系人`, `发现`, `我的`.
- Mobile-first layout is required. Desktop may render a phone-frame preview.
- Keep UI simple and close to WeChat: light gray page background, white list cards, compact rows, green active state.
- The frontend must not contain real tokens, passwords, connection strings, or secrets.

## Tech stack

- React + TypeScript + Vite
- Vitest + Testing Library
- ESLint
- lucide-react icons
- Native CSS for now; do not add a UI component framework unless explicitly requested.

## Repository conventions

- Frontend lives under `web/`.
- Root scripts are available:
  - `npm run frontend:dev`
  - `npm run frontend:test`
  - `npm run frontend:build`
  - `npm run frontend:lint`
- Read these files before changing frontend behavior:
  - `docs/FRONTEND.md`
  - `docs/product-specs/frontend-backend-contract.md`
  - `docs/DEVELOPMENT.md`
  - `web/src/App.tsx`
  - `web/src/App.test.tsx`
  - `web/src/styles.css`

## TDD and verification

For behavior changes:

1. Add/update a Vitest + Testing Library test first.
2. Run the targeted test and confirm it fails for the expected reason.
3. Implement the minimal UI/API code.
4. Run the targeted test and full frontend checks.

Required checks before commit:

```bash
npm install --prefix web
npm run frontend:test
npm run frontend:build
npm run frontend:lint
PATH=/tmp/go/bin:$HOME/go/bin:$PATH go test ./...
bash scripts/verify-static.sh
docker compose config
```

## Backend contract

Use `docs/product-specs/frontend-backend-contract.md` as the source of truth for REST and WebSocket paths.

Current local backend expectations:

- Auth token: `Authorization: Bearer ***
- User: `/me`, `/users/exists`, `/users/:identifier`
- Friends: `/friends`
- Groups: `/groups`, `/groups/:id/members`
- Messages: `/messages`, `/conversations/seqs`, `/conversations/:id/messages`, `/conversations/:id/read`
- WebSocket: `/ws`, with token header preferred and `?token=***` fallback.

## Scope control

- Keep each feature branch focused.
- Preserve the four-tab shell unless the task explicitly changes navigation.
- Use mock data only for visual-only scaffolding, test fixtures, or explicit demo/mock modes; API integration must prove real contract paths and token flow.
- Do not break backend MVP tests.
