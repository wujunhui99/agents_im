# ADR：media_id / msg_id text→bigint 的 wire 兼容与迁移策略

适用场景：实现 #530（media_objects 雪花主键）/ #531（messages 雪花 message_id）前，需要先定清 ID 的对外承载格式与存量迁移方式时读本文。

父 EPIC：#527；本文是子 issue **#529（spike）** 的产出，gate #530 / #531。状态：**已定稿**。

## 背景

#527 把 `media_objects.media_id` 与 `messages.message_id` 从 `text` 主键改为**雪花 bigint**（生成器见 `snowflake-id-generation.md`，随 #528 合入）。这是破坏性变更，落地前必须先定三件事：

1. **wire 承载**：雪花 64 位 > JS 安全整数 2^53（≈9e15），JSON 里 ID 用 string 还是 number？
2. **存量迁移**：主键列 text→bigint；历史消息 `content.mediaId` 引用怎么处理。
3. **兼容窗口**：是否需要新旧 ID 并存过渡，还是停机/清库切换。

### 现状事实（实现约束的依据）

- `media_objects.media_id text primary key`，当前值形如 `med_`+随机 hex（`internal/repository/media_memory.go` 仍有 `med_%06d` 占位）。
- `messages.message_id text primary key`。
- 消息内容里媒体引用：`content ->> 'mediaId'`（JSON **字符串**字段），image/file 两种 content 都带；被 `internal/repository/postgres_message.go:415`（`UserCanAccessMedia`）与 `message_memory.go:582` 读取。
- 头像引用 `profiles.avatar_media_id text`、`groups.avatar_media_id text` 也指向 media_id。
- 前端 `web/src/api/media.ts`：`mediaId: string`，REST 路径 `/media/{mediaId}/...`、`/media/uploads/{mediaId}/complete`。
- **先例**：account_id 已走完同类变更（**D16**，`00-decisions.md` §D16 + 迁移 `013`/`020`）——列 text→bigint，**对外仍以十进制字符串传输**，并配套**数据清零重置**。本 ADR 与 D16 保持一致。
- **数据量**：当前生产为 pre-launch（仅 seed 助手 + 测试账号，无真实用户消息/媒体存量需要保留），与 D16 数据重置时同一前提。

## 决策

### 1. wire 承载：十进制字符串（string），不用 number ✅

JSON / REST / WS 契约 / JWT claim / `content.mediaId` / 前端,所有 media_id、msg_id **一律以十进制字符串承载**,与 account_id（D16）同规则。

- **为什么不用 number**：雪花 ID 当前 epoch（2026）下 tick≈4e10，左移 22 位后 ID≈1.8e17 > 2^53≈9e15。JS `number` 是 IEEE-754 double，超过 2^53 整数精度丢失,前端 `JSON.parse` 会把 ID 读错——静默改值，最难排查的一类 bug。
- **为什么是 string 而非 number+BigInt**：契约要全链路（Go/PG/REST/前端/客户端）一致，string 是唯一各端都无歧义、无精度坑的承载；BigInt 序列化在 JSON 里仍是 string，徒增复杂度。
- **影响面**：媒体引用值 `content.mediaId` **保持 JSON 字符串类型**——变的只是字符串*格式*（`med_xxx` → 十进制数字串），不是 JSON *类型*。同理 `avatar_media_id`、REST 路径参数继续是字符串，无需改契约类型。
- DB 列是 `bigint`，序列化进出在边界做 `int64 ↔ decimal string`（`strconv.FormatInt` / `ParseInt`），与 `RoutedFlake.NextString` 出参一致。

### 2. 存量迁移:clean cutover + 数据清零重置 ✅

**不做** 双写、不做历史 `content.mediaId` 回填、不做新旧 ID 并存。直接:

1. 新增 `db/migrations/*.sql`：清空 `messages` 与 `media_objects`（及只与之相关、无独立保留价值的派生数据，如 `media_objects` 的 pending 行），再 `alter column ... type bigint`，重建主键/外键/索引。
2. `content.mediaId` 无历史值需迁移（消息表已清空）；新消息一开始就写十进制数字串。
3. 头像 `avatar_media_id`：seed 头像若有引用，随媒体重置一并清空/重置（实现 #530 时确认 seed 是否带头像 media 引用，无则零改动）。

**为什么 clean cutover 而非渐进迁移**：

- 生产 pre-launch，无真实存量消息/媒体要保留;清库代价≈0,而双写/回填要写转换函数（`med_xxx` 无法映射到雪花 bigint，本就只能丢弃或重新生成,等价于清）、加兼容分支、拉长窗口,纯属为不存在的数据买单。
- **与 D16 同策**:account_id 迁移就是清零重置(`020` 注释明言 "runs as part of the D16 data reset"),media/msg ID 同一前提、同一做法,避免两套迁移心智。
- 雪花 ID **无法从旧 `med_`/旧 text msg_id 推导**(旧值非数字、不含时间/机器位),即便想保留也只能重新发号——这本身就排除了"原地转换"路径。

> **更新(refine,#531 落地)**:`messages.message_id` 的迁移(`022_messages_message_id_snowflake.sql`)按 owner 指示改为**就地按 `created_at` 重算雪花、不清表**,而非 truncate:
> - 每行用 `created_at` 构造雪花 bigint(41 时间戳 + 中段 0 + 同毫秒 `row_number` 占序列号),并同步重写自引用 `trigger_message_id` 与 `conversation_threads.last_message_id`,最后 `alter ... type bigint`。
> - 旧值即便非数字也无妨:迁移在 `alter` 前已把所有 `message_id` 覆盖为新十进制串,`::bigint` 只对新值求值。
> - 生产 messages 为 pre-launch 空表 / CI 全新库时,该迁移对 0 行求值,**退化为与 clean cutover 等效**;有存量时则原地转换、不丢数据。
> - `trigger_message_id` 本次**保留 `text`**(承载十进制串,与 wire=string 一致),不随 message_id 一起改 bigint——避免 `not null default ''` → bigint 的 "0 vs 空" 语义迁移牵连 model / convert / keystone repo;#531 范围仅 `message_id` 主键类型。media_objects 仍按本 ADR 原 clean cutover(#541/#546)。

### 3. 兼容窗口:无 ✅

不设新旧 ID 并存过渡期。配合 clean cutover,部署即切换:旧客户端持有的旧格式 ID 在重置后的库里查不到(消息/媒体已清),按正常 404/not-found 处理,无需特殊兼容码。

## 给 #530 / #531 的实现约束(产出)

实现下列子 issue 时**必须**遵守:

1. **列类型**:`media_objects.media_id`、`messages.message_id` 及所有引用列(`avatar_media_id`、`trigger_message_id`、消息表内 `message_id` 等)→ `bigint`。
2. **wire = 十进制字符串**:proto/REST/WS/JWT/前端中 media_id、msg_id 字段类型保持 `string`;Go 边界用 `strconv.FormatInt(id,10)` / `ParseInt(s,10,64)` 转换。**禁止**在 JSON 里以 number 承载。
3. **`content.mediaId`**:保持 JSON 字符串类型,值改为十进制数字串;读取处(`UserCanAccessMedia` 等)按字符串比较不变,但 #532 链路校验里与入参 media_id 比较时两侧统一为同一字符串形或同一 int64 形,避免 `med_xxx` 残留导致恒不等。
4. **迁移方式**:新增 `db/migrations/*.sql` 做 clean cutover(清空 messages/media_objects → 改列类型 → 重建约束/索引),不写历史回填、不加双写、不留兼容窗口。已发布 migration 不可改(AGENTS.md 规则 7)。
5. **生成**:新 ID 由 `pkg/idgen` 的 `RoutedFlake` 发(#528);media `HintBits=0`、msg `HintBits=1`。
6. **前置确认(实现时)**:核对生产此刻确无需保留的真实消息/媒体存量(同 D16 重置前提);若届时已有真实用户数据,本 ADR 的 clean cutover 决策需回炉重审(改为停机导出/丢弃声明)。

## 参考

- `snowflake-id-generation.md`：RoutedFlake 生成器(#528,随其合入本目录)
- `docs/refactor/v1/00-decisions.md` §D16:account_id 类型化先例(text→bigint、string wire、数据重置)
- 迁移先例:`db/migrations/013_internal_agent_ids_bigint.sql`、`020_default_assistant_facet_account_id.sql`
