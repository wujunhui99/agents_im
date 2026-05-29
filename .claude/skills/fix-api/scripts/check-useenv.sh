#!/usr/bin/env bash
# 检查【live 入口】是否缺少 conf.UseEnv()——缺了则 yaml 里的 ${VAR} 不会展开。
#
# live 入口链：service/<svc>/<api|rpc>/<svc>.go（或 service/<name>/main.go）→ <svc>entry.Start() → entry/entry.go
# 只扫 entry/entry.go（真正加载配置处）；main 仅做 flag 解析与委托，不在此判定。
found=0
for f in $(find service/ internal/ -path "*/entry/entry.go" 2>/dev/null | sort); do
  grep -q "conf.MustLoad" "$f" || continue
  if ! grep -q "UseEnv" "$f"; then
    echo "MISSING UseEnv: $f"
    found=1
  fi
done
[ "$found" -eq 0 ] && echo "OK: 所有 live 入口的 conf.MustLoad 均带 UseEnv()"
# 注意：改完 UseEnv 必须 rollout 对应 deployment 才生效（见 SKILL.md 第 2 节）。
