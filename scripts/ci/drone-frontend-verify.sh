#!/usr/bin/env bash
set -euo pipefail

npm --prefix web ci --prefer-offline
npm --prefix web run lint
npm --prefix web run test:run
npm --prefix web run build
