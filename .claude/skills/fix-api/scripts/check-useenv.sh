#!/usr/bin/env bash
# 检查【live 入口】是否缺少 conf.UseEnv()——缺了则 yaml 里的 ${VAR} 不会展开。
#
# live 入口链：cmd/<svc>/main.go → <svc>entry.Start() → service|internal 下的 entry/entry.go
# 只扫 entry/entry.go；*.v1.go 是重构前的旧 main，已无 cmd 引用，是死代码，不扫（否则全是误报）。
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
