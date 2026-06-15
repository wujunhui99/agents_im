# Snowflake ID 生成器（RoutedFlake）

适用场景：需要为 media_id / msg_id（及后续任何"可路由分类 + 机器标记 + 时间有序"的实体）生成 64 位雪花 ID 时读本文。事实源代码在 `pkg/idgen/routedflake.go`。

父 EPIC：#527（media 业务逻辑重做）；本生成器由子 issue #528 落地，是 §0（msg_id）/§1（media_id）的共用地基。account_id 另有 D16 布局（`pkg/idgen/account.go`），与本生成器同思想但位段不同（见末节）。

## 背景与目标

- media_objects 主键从 `text`（`med_`+随机 hex）改为雪花 bigint，msg 域 `messages.message_id` 同步雪花 bigint（#527 §0/§1）。两者共用一套 64 位布局，仅"路由 hint 位宽"不同。
- 多副本 rpc 同毫秒生成必须无碰撞 → **机器位强制存在**，且机器号必须唯一且稳定。
- 非目标（#528）：不接入 media/msg 域，纯库；不提供进程级默认实例（由各 rpc 自行从 config + `ResolveMachineID` 构造）。

## 位布局（64 位）

```
 63        62 .................. 22 21 ........ 10 9 ......... 0
┌──┬───────────────────────────┬───────────────┬─────────────┐
│符│        时间戳 41 位        │  中段 12 位   │  序列号 10  │
│号│  1ms / since 2026-01-01    │ hint｜保留｜机器│  0..1023    │
└──┴───────────────────────────┴───────────────┴─────────────┘
  0   高位 → 近似时间有序(PK 局部性)
```

- **符号位**：恒 0。正数 → 十进制字符串传输保持数值序（同 account_id 不变量）。
- **时间戳 41 位**：1ms 粒度，自定义 epoch `2026-01-01T00:00:00Z`（与 `snowflake.go` / `account.go` 共用 `snowflakeEpochMs`），约 69.7 年。
- **中段 12 位 = 路由 hint + 机器号，边界动态**：
  - **路由 hint** 占中段**最高** `hintBits` 位，从 MSB 往右长；
  - **机器号** 占中段**最低** `machineBits` 位，从 LSB 往左长；
  - 中间剩余位**保留恒 0**。两个字段从中段两端相向生长，谁加宽都不挪动对方已有取值 → 部署可独立调机器位数（扩副本）或 hint 位数（加路由类别）。
- **序列号 10 位**：每 ms 每实例 1024 个（≈100w ID/s/实例），耗尽自旋到下一 ms。

### 各域 hint 用法

| 域 | `HintBits` | 含义 |
|----|-----------|------|
| media | 0 | 无单/群语义，hint 位段**预留**，中段除机器位外恒 0 |
| msg | 1 | 中段 MSB 区分单聊/群聊：`100… vs 000…`（取**最左**位，不是最低位 `001 vs 000`） |

> hint 取最左位、机器号靠右、边界动态——这条约定与 account_id（D16）通用。

## API

```go
gen, err := idgen.NewRoutedFlake(idgen.RoutedFlakeConfig{
    HintBits:    1,   // msg: 1；media: 0
    MachineBits: 5,   // 按部署副本规模选，hintBits+machineBits ≤ 12
    MachineID:   ordinal,
})
id, err := gen.Next(hint)          // hint ∈ [0, 2^HintBits)；media 传 0
s,  err := gen.NextString(hint)    // 十进制字符串传输形（见 #529）
```

构造期校验：`MachineBits ≥ 1`（强制机器位）、`hintBits + machineBits ≤ 12`、`MachineID ∈ [0, 2^MachineBits)`。

时钟回拨：当前 ms < 上次 ms → **返回错误拒绝生成**（不静默 fallback，调用方 fail-first）。序列号本 ms 耗尽 → `waitNextMillis` 自旋到下一 ms。

## 机器号来源选型（定稿）

机器号**必须唯一且稳定**——两个副本拿到同一机器号会静默生成重复 ID。`ResolveMachineID()` 按优先级解析：

1. **`AGENTS_IM_SNOWFLAKE_MACHINE_ID`**：显式注入（任何部署可直接给号）。专用环境变量，与旧 `Snowflake` 的 `AGENTS_IM_SNOWFLAKE_NODE_ID` 区分（后者带 hash fallback，本生成器禁止）。
2. **`POD_NAME` ordinal**：StatefulSet pod 名为 `<name>-<ordinal>`（如 `media-rpc-3` → 3），ordinal 唯一且跨重调度稳定，正是机器号所需。**只认最后一个 `-` 分隔段为纯数字的形状**——Deployment/ReplicaSet pod 名（`<name>-<rs-hash>-<随机后缀>`）末段非纯数字，即便后缀恰好以数字结尾（如 `media-rpc-6d4b8f9c7-q2v85`）也**拒绝**，绝不把不唯一的尾数当 ordinal。
3. **`HOSTNAME` ordinal**：StatefulSet pod 内 `HOSTNAME` == pod 名，作兜底，同样按"末段纯数字"判定。

**全部取不到序号即返回错误**（fail-first），不猜。**显式禁用 `hash(pod name)`**：hash 会碰撞，碰撞即重复 ID（#527 §1 / #528 硬约束）。

> 部署落地：
> - **单副本（现状，media-rpc / msg-rpc 均 `kind: Deployment` `replicas: 1`）**：直接在各 Deployment 容器 `env:` 写死 `AGENTS_IM_SNOWFLAKE_MACHINE_ID="0"`（media 与 msg 是独立 ID 空间，都给 `0` 互不碰撞）。**扩副本前必须改下一条**——manifest 已就近注释该约束。
> - **多副本**：改 **StatefulSet** 部署并用 downward API 注入 `POD_NAME`（`fieldRef: metadata.name`），靠稳定 ordinal 自动发号，**删掉**上面的常量；副本数上限受 `MachineBits` 约束（如 5 位 → 32 副本）。
> - 真正构造 `RoutedFlake` 实例（读 env / 调 `ResolveMachineID`）在接入 media/msg 的子 issue #530/#531 内做，非本库职责。

## 与既有生成器的关系

`pkg/idgen` 现有三套，互不替换：

| 类型 | 布局 | 用途 |
|------|------|------|
| `Snowflake`（`snowflake.go`） | 41 ts + 10 node + 12 seq，node 带 hash fallback | 旧通用 ID（退役中，新代码勿用其 hash 默认） |
| `AccountIDGenerator`（`account.go`） | 41 ts + 3 facet + 9 machine + 10 seq | D16 account_id，facet 编码人类/agent |
| **`RoutedFlake`（`routedflake.go`）** | 41 ts + 12 中段(hint+machine 动态) + 10 seq | **本文**：media_id / msg_id |

## 验证

```bash
go test ./pkg/idgen/ -count=1
go test ./pkg/idgen/ -run=XXX -bench=BenchmarkRoutedFlakeNext -benchtime=2s
```

单测覆盖：位布局/hint 编码、media 无 hint、tick 内单调、跨 ms 序列重置、序列耗尽自旋、时钟回拨拒绝、16 并发 8w ID 无重复、时间序、坏配置拒绝、`ResolveMachineID` 各来源。压测：单副本 ≈ 1.3µs/op、0 alloc（量级 ~70w+ ID/s，远超需求）。
