#!/usr/bin/env bash
# 检查 deploy/k8s/etc/*.yaml 中引用的变量，哪些在 K8s secret 里值为纯数字（需在 YAML 加引号）
ssh -p 9093 root@207.57.131.50 "kubectl get secret agents-im-secrets -n agents-im -o json" | \
  python3 -c "
import sys, json, base64
d = json.load(sys.stdin)['data']
for k, v in d.items():
    val = base64.b64decode(v).decode()
    if val.isdigit():
        print(f'NUMERIC: {k}={val}')
"

echo ""
echo "--- YAML 中未加引号的数字变量引用 ---"
grep -rn '\${' deploy/k8s/etc/ | grep -v '"${'
