# group_members 引入代理主键 id

## Purpose
goctl `model pg datasource` 不支持复合主键，无法为 `group_members`（主键 `(group_id, account_id)`）
原生生成 model。引入自增代理主键 `id` 后可原生生成，配合 groups 微服务重构（issue #415）。

## Impacted tables/fields/indexes
- `group_members`：
  - 删除复合主键约束 `group_members_pkey`。
  - 新增唯一约束 `group_members_group_account_key UNIQUE (group_id, account_id)`（保留原唯一性）。
  - 新增列 `id bigint GENERATED ALWAYS AS IDENTITY`，设为新主键。

## Destructive?
否。仅替换主键形态：删除复合主键后立即用唯一约束保留 `(group_id, account_id)` 唯一性，
并新增 `id` 主键。既有行由 IDENTITY 自动回填。新增列对既有列名 SELECT 透明。

## Apply order
单文件单事务执行即可（migrations 以 `psql -1` 应用）。无依赖其它未应用迁移。

## Backward compatibility
message monolith（`internal/repository/postgres_groups.go`）仍读此表：
- SELECT 使用显式列名，不受新增列影响；
- `ON CONFLICT (group_id, account_id) DO UPDATE` 依赖唯一性，由新唯一约束保留 → 不受影响。

## Rollback
```sql
ALTER TABLE group_members DROP CONSTRAINT IF EXISTS group_members_pkey;   -- 即 id 主键
ALTER TABLE group_members DROP COLUMN IF EXISTS id;
ALTER TABLE group_members DROP CONSTRAINT IF EXISTS group_members_group_account_key;
ALTER TABLE group_members ADD PRIMARY KEY (group_id, account_id);
```

## Verification
```sql
select conname, contype from pg_constraint where conrelid='group_members'::regclass;
-- 期望：id 主键(p) + group_members_group_account_key 唯一(u)
\d group_members
```
