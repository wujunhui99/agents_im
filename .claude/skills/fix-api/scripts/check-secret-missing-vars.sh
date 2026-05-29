#!/usr/bin/env bash
# 对比 YAML 中引用的环境变量与 K8s secret 中已有的 key，输出缺失项
grep -roh '\${[^}]*}' deploy/k8s/etc/ | grep -oP '\$\{\K[^}]+' | sort -u > /tmp/yaml_vars.txt

ssh -p 9093 root@207.57.131.50 "kubectl get secret agents-im-secrets -n agents-im -o json" | \
  python3 -c "import sys,json; [print(k) for k in json.load(sys.stdin)['data']]" | sort > /tmp/secret_keys.txt

missing=$(comm -23 /tmp/yaml_vars.txt /tmp/secret_keys.txt)
if [ -z "$missing" ]; then
  echo "OK: 所有 YAML 变量均已在 secret 中定义"
else
  echo "MISSING（会展开为空）:"
  echo "$missing"
fi
