---
name: fix-api
description: Fix backend API errors in the agents_im project. Use this skill when a user reports an API endpoint returning an HTTP error code (401, 403, 500, 503, etc.), uses phrases like "接口报错", "接口不通", "fix API", or when a URL + HTTP status code is mentioned.
---

# Fix API Error

## 1. 诊断 — 先读错误，再读代码

沿数据流定位断点：HTTP 入口 → API handler → RPC → 数据库/外部依赖。先确认错误发生在哪一跳，再读对应代码。

**有 `x-trace-id`** → 先查 Jaeger（地址见 `ARCHITECTURE.md`）再看代码：
```bash
curl "http://<jaeger-host>/api/traces/<x-trace-id>"
```

**常见根因（按状态码缩小范围）**

| 状态码 | 排查方向 |
|--------|----------|
| 401/403 | token 过期/无效；路由鉴权配置；JWT secret 不一致 |
| 503 | RPC endpoint 配置错误；依赖服务未启动；配置未正确加载 |
| 500 | nil pointer；DB 连接串错误；handler 未处理 RPC error；model scan 类型不匹配 |

不要预设是某个已知 bug——先用上面表格缩小方向，再进第 2 节按层扫描确认范围。

**`500 "internal server error"` 是 `apperror.From` 兜底任意非 apperror 的通用文案——真因不在 message 里，且常没打日志。** 别纠结 message：
- 按 trace_id 看「最后一条成功 SQL」之后断在哪跳；某跳（如 model scan）**无日志却返回 raw error** = 该跳内部失败被吞。
- DB 层疑点用只读副本复现 go-zero 真实查询路径（一眼看到 raw error）：`secret/pg-replica/.env` + `postgres.New(dsn)` 调 model 方法。例：`QueryRowCtx` 扫单列进 `*time.Time` 报 `not matching destination to scan`（go-zero 把 time.Time 当 struct）→ 改单字段 struct（#624）。
- **「首次 500、二次 400/已存在」= 副作用先提交、后续步骤才失败**（验证码已消费、行已写）——查那个「已提交但流程没走完」的写操作。

## 2. 相似漏洞扫描 — 修之前先确认范围

同类 bug 往往多服务同时存在，**代码、配置、运行时三层都要查**，一并修。

**① 代码层：缺 UseEnv() 的服务**
```bash
bash .claude/skills/fix-api/scripts/check-useenv.sh
```

**② 配置层：YAML 里数字变量未加引号**（展开后被 YAML 解析为 number，与 string struct 不匹配）
```bash
bash .claude/skills/fix-api/scripts/check-yaml-numeric-vars.sh
```

**③ 运行时层：K8s secret 缺失 YAML 引用的变量**（展开为空 → 鉴权/连接失败）
```bash
bash .claude/skills/fix-api/scripts/check-secret-missing-vars.sh
```
有缺失 → patch secret 后重启所有 app pod：
```bash
ssh -p 9093 root@207.57.131.50 "kubectl rollout restart deployment/user-api deployment/groups-api deployment/auth-api deployment/auth-rpc deployment/message-api deployment/friends-api deployment/gateway-ws deployment/agent-api deployment/third-rpc deployment/user-rpc deployment/groups-rpc deployment/friends-rpc deployment/message-rpc deployment/message-transfer -n agents-im"
```

有相似漏洞 → 同一 PR 修，body 列明受影响服务。

## 3. 修复流程

```bash
gh issue create --title "[BUG]: <描述>" --body "<root cause>\n\nAgent: claude\nHuman-Owner: wujunhui99"
git checkout -b fix/claude/issue-<N>-<short-desc>
# 改代码
go build ./...
go test ./service/<affected>/...
git add <files>
git commit -m "fix(<scope>)[claude]: <title>\n\nFixes #<N>\n\nCo-Authored-By: Claude <noreply@anthropic.com>"
git push -u origin fix/claude/issue-<N>-<short-desc>
gh pr create --title "fix(...): ..." --body "Fixes #<N>. <root cause>." --base main
gh pr merge <PR号> --squash
```

## 4. 监控 Drone CI（异步）

用 `run_in_background: true` 执行 `scripts/drone-watch.sh`，脚本结束后自动通知，期间用户可继续对话。

```bash
bash scripts/drone-watch.sh
```

收到通知后：
- **success** → 第 5 步
- **failure/error/killed** → 查日志：`curl -sf -H "Authorization: Bearer $(grep DRONE_TOKEN secret/drone_token | cut -d= -f2 | tr -d '\n')" "https://drone.agenticim.xyz/api/repos/wujunhui99/agents_im/builds/<N>/logs/1/<step>" | python3 -c "import sys,json; [print(l['out'],end='') for l in json.load(sys.stdin)]" | tail -50`
- **pod 启动失败** → `ssh -p 9093 root@207.57.131.50 "kubectl logs -n agents-im -l app=<svc> --tail=30"`

## 5. 回归测试

```bash
curl -s -o /dev/null -w "%{http_code}" \
  -H "Authorization: Bearer <JWT>" \
  https://agenticim.xyz/<原始失败路径>
```

2xx → 完成。仍失败 → 回第 1 步。
