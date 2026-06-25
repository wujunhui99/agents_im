#!/usr/bin/env bash
set -euo pipefail

# Frontend (web/) static gates: design tokens, localized shell strings, no mock
# flows. Owned by the frontend step so the backend orchestrator stays backend-only.
bash scripts/verify/verify-frontend-static.sh

npm --prefix web ci --prefer-offline
npm --prefix web run lint
npm --prefix web run test:run
npm --prefix web run build
