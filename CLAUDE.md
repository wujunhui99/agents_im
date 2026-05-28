# CLAUDE.md — agents_im 项目上下文

本文件是给 Claude（AI agent）的项目上下文，记录身份约定、常用操作和踩过的坑，Claude Code 会话开始时自动读取。

---

## 我的身份

- **Agent 名称**：`claude`
- **分支命名**：`<type>/claude/issue-<N>-<desc>`，例如 `fix/claude/issue-302-auth-rpc-env-expansion`
- **Commit 格式**：`<type>(<scope>)[claude]: <描述>`
- **已注册位置**：`scripts/ci/verify-agent-branch-name.sh` → `TRUSTED_AGENTS`

其他 agent：hermes、achilles、eino、helios、furies、gaia。

---

## 基础设施速查

| 资源 | 地址/命令 |
|------|----------|
| 生产域名 | https://agenticim.xyz |
| Drone CI | https://drone.agenticim.xyz/wujunhui99/agents_im |
| Drone Token | `secret/drone_token`（本地未追踪） |
| 服务器 SSH | `ssh -p 9093 root@207.57.131.50` |
| k8s 命名空间 | `agents-im` |

**合并到 main 会自动触发 Drone CI 部署**，无需手动操作。

---

## 常用排查命令

```bash
# 查看所有 pod 状态（重点看非 Running 的）
ssh -p 9093 root@207.57.131.50 "kubectl get pods -n agents-im"

# 看某服务最近崩溃日志
ssh -p 9093 root@207.57.131.50 \
  "kubectl logs -n agents-im deploy/<service> --tail=50 --previous"

# Drone API 查最近构建
source secret/drone_token
curl -s -H "Authorization: Bearer $DRONE_TOKEN" \
  "$DRONE_SERVER/api/repos/wujunhui99/agents_im/builds?limit=5" | \
  python3 -c "import sys,json; [print(f'#{b[\"number\"]} {b[\"status\"]:10} {b[\"message\"][:60]}') for b in json.load(sys.stdin)]"
```

---

## 坑：go-zero `conf.MustLoad` 不展开 `${VAR}`

**现象**：auth-rpc CrashLoopBackOff，日志末尾：
```
bootstrap admin account: INVALID_ARGUMENT: identifier must start with a letter or digit
```

**根因**：go-zero `conf.MustLoad` 默认不展开 `${ENV_VAR}` 占位符，需要显式传 `conf.UseEnv()` 选项。未展开时 `AdminBootstrap.Identifier` 的值是字面量 `"${ADMIN_BOOTSTRAP_IDENTIFIER}"`（以 `$` 开头），校验失败。

**修复**（`service/auth/rpc/entry/entry.go`）：
```go
// 错误
conf.MustLoad(configFile, &c)

// 正确
conf.MustLoad(configFile, &c, conf.UseEnv())
```

**附加**：`TokenAuth.AccessSecret` 展开后若为空，需补默认值：
```go
if c.TokenAuth.AccessSecret == "" {
    c.TokenAuth.AccessSecret = appconfig.DefaultJWTAuthConfig().AccessSecret
}
```

**相关 PR**：#302

---

## 坑：项目内存在两套配置加载路径

本项目 Go 服务有**两种**配置加载方式，行为不同：

| 方式 | 用于 | 是否自动展开 `${VAR}` |
|------|------|----------------------|
| `internal/config.LoadAPIConfig()` / `LoadRPCConfig()` | 大多数 api/rpc 服务 | ✅ 是（各字段手动 `os.ExpandEnv`） |
| go-zero `conf.MustLoad(file, &c)` | auth-rpc、部分 service layout 服务 | ❌ 否（需加 `conf.UseEnv()`） |

新增 service layout 服务时，如果用 `conf.MustLoad` 加载包含 `${VAR}` 的配置，**必须**加 `conf.UseEnv()`。

---

## 坑：`JWT_ACCESS_SECRET` 未写入 k8s secret

**现状**：`agents-im-secrets` 中**没有** `JWT_ACCESS_SECRET` 这个 key。

**影响**：
- 使用 `LoadAPIConfig`/`LoadRPCConfig` 的服务：`os.ExpandEnv("${JWT_ACCESS_SECRET}")` → `""`，自动回退到默认值 `"dev-jwt-secret-change-me"`。
- 使用 `conf.MustLoad` + `conf.UseEnv()` 的服务（如修复后的 auth-rpc）：展开为空字符串，需代码层面补回默认值。

**结论**：所有服务目前实际上都在用同一个硬编码 dev key 签发/验证 JWT，这在生产环境存在安全隐患。如需修复，向 `agents-im-secrets` 补充 `JWT_ACCESS_SECRET` 字段，所有服务会自动使用真实密钥。

---

## 坑：分支命名不符合 CI 规则会阻止合并

**规则**（`scripts/ci/verify-agent-branch-name.sh`）：
```
<type>/<agent-name>/issue-<number>-<slug>
```

`hotfix/hermes/login-jwt-secret-expansion` 不符合格式（第三段缺少 `issue-<N>-` 前缀），会导致 CI agent branch check 失败。**所有 PR 分支都必须带 issue 编号**。

---

## Drone CI 监控

见 `docs/agent-ops/drone-ci-monitoring.md`。

简版：
1. Drone CLI `drone build watch wujunhui99/agents_im <N>`（**待验证，见文档**）
2. Drone API 轮询（见上方命令）
3. Telegram 通知（pipeline 内已有步骤）

---

## 部署后必做健康检查

```bash
ssh -p 9093 root@207.57.131.50 \
  "kubectl get pods -n agents-im | grep -v Running | grep -v Completed"
# 无输出 = 全部健康

# 重点检查 auth-rpc（历史上曾多次崩溃）
ssh -p 9093 root@207.57.131.50 \
  "kubectl get pod -n agents-im -l app=auth-rpc"
```
