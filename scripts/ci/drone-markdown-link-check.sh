#!/usr/bin/env bash
# Markdown 死链检查（#488 从 drone-backend-verify.sh 拆出）：跑在 node 镜像，
# 免去在 golang 镜像里走 NodeSource 装 node；npx 缓存经 npm_config_cache 复用。
set -euo pipefail

mapfile -t md_files < <(
  find . -name "*.md" \
    -not -path "./.git/*" \
    -not -path "./docs/references/*" \
    -not -path "./web/node_modules/*" \
    -print
)
if ((${#md_files[@]} > 0)); then
  npx --yes markdown-link-check@3.13.7 --config .github/markdown-link-check.json "${md_files[@]}"
fi
