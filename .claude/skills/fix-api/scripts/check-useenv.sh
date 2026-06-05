#!/usr/bin/env bash
# 检查所有 conf.MustLoad 调用是否缺少 conf.UseEnv()——缺了则 yaml 里的 ${VAR} 不会展开。
#
# 入口已内联到各 service main（user/auth/friends/groups 的 api+rpc、third-rpc 在
# service/<svc>/<api|rpc>/<svc>.go；message-rpc 在 internal/rpcgen/message/message.go）。
# agent-api、gateway-ws、message-api、message-transfer 用自定义 pkg/config 加载器
# （逐字段 os.ExpandEnv），不走 conf.MustLoad，故不在此检查范围。
found=0
for f in $(grep -rln "conf.MustLoad" --include="*.go" service/ internal/ 2>/dev/null | grep -v "_test.go" | sort); do
  if ! grep -q "UseEnv" "$f"; then
    echo "MISSING UseEnv: $f"
    found=1
  fi
done
[ "$found" -eq 0 ] && echo "OK: 所有 conf.MustLoad 均带 UseEnv()"
# 注意：改完 UseEnv 必须 rollout 对应 deployment 才生效（见 SKILL.md 第 2 节）。
