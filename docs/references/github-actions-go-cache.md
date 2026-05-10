# GitHub Actions Go Cache Notes

本文记录 `agents_im` CI 中 GitHub Actions cache 的工作机制、曾经导致 `go test` 在 GitHub 上远慢于本地的原因，以及 PR #61 中采用的修复方案。

## 背景

在 CI 成本优化过程中发现：

- 本地 warm `go test` 通常约 6 秒；
- GitHub Actions `Backend verification :: Run Go tests` 曾长期约 2 分多钟；
- 显式 `go mod download` 在 CI 中约 5-9 秒，后来 cache 命中后为 0 秒；
- 临时诊断显示 `go test -run '^$'` compile/empty-run 约 140 秒，而 compile warmup 后的 `go test -json` 约 18 秒；
- 慢包统计中真实测试执行最慢包只有数秒级。

结论：主要瓶颈不是单元测试函数，也不是 module 下载，而是 GitHub runner 上的 Go 编译/build cache 没有有效复用。

## GitHub Actions cache 基本机制

GitHub Actions cache 可以理解为一个 key-value 存储：

- **key**：缓存名字/索引，例如 `go-build-Linux-X64-go-1.24.1-...`；
- **value**：某个目录打包压缩后的 archive，例如 `~/.cache/go-build` 或 `~/go/pkg/mod`。

一个 cache key 不是一个普通文件；它对应的是一份压缩后的目录快照。一个仓库可以同时存在多个 cache key。GitHub 默认对仓库 cache 有总容量限制；写本文时默认限制是 10GB，但本次问题不是因为达到容量限制导致 eviction。

### exact key 与 restore keys

`actions/cache/restore` 会先找完全一致的 key。若 exact key 不存在，可以通过 `restore-keys` 按前缀查找相近旧 cache。

示例：

```yaml
key: go-build-Linux-X64-go-1.24.1-<go.sum hash>-<source hash>
restore-keys: |
  go-build-Linux-X64-go-1.24.1-<go.sum hash>-
  go-build-Linux-X64-go-1.24.1-
```

含义：

1. 当前源码完全相同，则 exact key 命中；
2. 源码变化但依赖未变，则通过 `go.sum` 前缀恢复最近旧 build cache；
3. 依赖也变化时，至少可尝试同 Go 版本下的较旧 build cache。

### cache immutable 特性

GitHub Actions cache 基本是 immutable 的：同一个 key 一旦保存，后续不会被正常覆盖。因此，如果一个旧 key 已存在，即使一次 CI 运行后生成了更完整的目录内容，也不会自动把同一个 key 更新成更完整的 cache。

这也是过去 cache “命中了但仍然慢”的关键原因之一。

## Go 相关的两类 cache

Go CI 中至少要区分两类 cache。

### `GOMODCACHE`：module 源码 cache

路径：

```bash
go env GOMODCACHE
```

GitHub runner 通常是：

```text
/home/runner/go/pkg/mod
```

它存储第三方依赖源码，主要影响 `go mod download` 和 `go test` 隐式下载依赖的耗时。该 cache 主要与 `go.sum` 相关，项目源码变化通常不需要重建 module cache。

### `GOCACHE`：Go 编译产物 cache

路径：

```bash
go env GOCACHE
```

GitHub runner 通常是：

```text
/home/runner/.cache/go-build
```

它存储 Go 编译和测试的中间产物，主要影响 `go test ./...` 的 compile/link 阶段。该 cache 与 Go 版本、OS/arch、依赖、源码、build tags/flags 等因素相关。

## 过去失效/低效的原因

过去 workflow 使用 `actions/setup-go@v5` 内置 cache：

```yaml
- uses: actions/setup-go@v5
  with:
    go-version-file: go.mod
    cache: true
```

日志显示它确实命中了旧 cache：

```text
Cache hit for: setup-go-Linux-x64-ubuntu24-go-1.24.1-...
Cache Size: ~202 MB
Cache restored successfully
```

但该 cache 的 key 主要基于 OS、arch、Ubuntu image、Go version 和 `go.sum`。项目 Go 源码大量变化但 `go.sum` 不变时，key 不变，CI 会一直恢复同一份旧 cache。

本次诊断中看到：

```text
GOCACHE before tests: ~394M
GOCACHE after tests: ~1.3G
go test compile/empty-run duration: ~140s
post-warmup go test -json: ~18s
```

这说明 restore 后的 build cache 不完整，`go test` 运行中现场编译了大量内容。但因为 exact key 已存在，旧 `setup-go` cache 不会被自动更新成更完整的目录快照。

另一个潜在问题是 `Backend verification` 和 `PostgreSQL integration` 两个并行 job 过去都使用相同的 `setup-go cache: true`。若某次 cache miss，较早结束的 postgres job 可能先保存较小/不完整的 cache，而 backend job 跑完后同 key 已存在，不能保存更完整的 cache。

## PR #61 的修复方案

PR #61 关闭 `setup-go` 内置 cache，并改为显式管理 Go module/build cache。

### 关闭 `setup-go` 内置 cache

```yaml
- name: Setup Go
  id: setup-go
  uses: actions/setup-go@v5
  with:
    go-version-file: go.mod
    cache: false
```

`setup-go` 只负责安装 Go，cache 策略由 workflow 显式控制。

### module cache：按 `go.sum` 保存

```yaml
- name: Restore Go module cache
  uses: actions/cache/restore@v4
  with:
    path: ~/go/pkg/mod
    key: go-mod-${{ runner.os }}-${{ runner.arch }}-go-${{ steps.setup-go.outputs.go-version }}-${{ hashFiles('go.sum') }}
```

backend job 在运行后保存相同 key：

```yaml
- name: Save Go module cache
  if: always()
  uses: actions/cache/save@v4
  continue-on-error: true
  with:
    path: ~/go/pkg/mod
    key: go-mod-${{ runner.os }}-${{ runner.arch }}-go-${{ steps.setup-go.outputs.go-version }}-${{ hashFiles('go.sum') }}
```

module cache 与源码无关，依赖不变即可复用。

### build cache：按依赖和源码保存，并使用 restore keys

```yaml
- name: Restore Go build cache
  id: go-build-cache
  uses: actions/cache/restore@v4
  with:
    path: ~/.cache/go-build
    key: go-build-${{ runner.os }}-${{ runner.arch }}-go-${{ steps.setup-go.outputs.go-version }}-${{ hashFiles('go.sum') }}-${{ hashFiles('cmd/**/*.go', 'internal/**/*.go', 'tests/**/*.go', 'go.mod', 'go.sum') }}
    restore-keys: |
      go-build-${{ runner.os }}-${{ runner.arch }}-go-${{ steps.setup-go.outputs.go-version }}-${{ hashFiles('go.sum') }}-
      go-build-${{ runner.os }}-${{ runner.arch }}-go-${{ steps.setup-go.outputs.go-version }}-
```

backend job 在运行后保存 exact key：

```yaml
- name: Save Go build cache
  if: always() && steps.go-build-cache.outputs.cache-hit != 'true'
  uses: actions/cache/save@v4
  continue-on-error: true
  with:
    path: ~/.cache/go-build
    key: go-build-${{ runner.os }}-${{ runner.arch }}-go-${{ steps.setup-go.outputs.go-version }}-${{ hashFiles('go.sum') }}-${{ hashFiles('cmd/**/*.go', 'internal/**/*.go', 'tests/**/*.go', 'go.mod', 'go.sum') }}
```

设计目的：

- exact key 包含源码 hash，同一源码状态可以精确命中；
- 源码变化时 exact key 变化，避免长期使用过旧 build cache；
- `restore-keys` 允许从相近旧 build cache 增量恢复；
- `cache-hit != 'true'` 避免同 key 已命中时重复保存不可覆盖的 cache。

### backend 负责保存，postgres 只 restore

`Backend verification` 跑完整普通 Go packages 测试，最适合生成完整 build cache。因此 PR #61 让 backend job restore + save，而 `PostgreSQL integration` 只 restore，不 save。

这样避免并行 job 竞争同一个 key，尤其避免 postgres job 先结束并保存较小/不完整 cache。

## 为什么修改后 cache 生效

PR #61 的第一轮新策略 CI 中，新 key 尚不存在，因此：

```text
Restore Go module cache: Cache not found
Restore Go build cache: Cache not found
Go module download duration: 9s
Go test duration: 215s
```

第一轮结束时 backend job 成功保存：

```text
Cache saved with key: go-mod-Linux-X64-go-1.24.1-...
Cache saved with key: go-build-Linux-X64-go-1.24.1-...
```

保存后的 cache 大小约为：

```text
go-mod:   ~245 MB compressed
go-build: ~275 MB compressed
```

同一 run rerun 后，新 cache 命中：

```text
Cache hit for: go-mod-Linux-X64-go-1.24.1-...
Cache Size: ~245 MB
Go module download duration: 0s

Cache hit for: go-build-Linux-X64-go-1.24.1-...
Cache Size: ~276 MB
Go test duration: 6s
```

CI job 耗时从第一轮的：

```text
Backend verification: 4m43s
Run Go tests: 215s
```

降到 rerun 的：

```text
Backend verification: 1m11s
Run Go tests: 6s
```

因此，修改后 cache 生效的原因是：

1. 旧的 `setup-go` cache 不再作为黑盒 cache 使用；
2. module cache 和 build cache 分开管理，key 更符合用途；
3. build cache key 包含源码 hash，不会长期命中过旧 cache；
4. restore keys 让源码变化后仍可复用相近旧 cache；
5. backend job 是唯一 build cache 保存者，避免并行 job 保存不完整 cache。

## 维护注意事项

- 第一轮新 key 或 cache 被清理后仍可能较慢，这是重新建立 cache 的正常成本。
- Go 源码频繁变化会产生多个 `go-build-*` key；如果 repo cache 接近容量限制，可用 `gh cache list` 和 `gh cache delete` 清理旧 build cache。
- 不要把 `PostgreSQL integration` 改成也保存 build cache，除非同时确保不会与 backend job 竞争同一个 key。
- 若 Go version、OS image、`go.sum` 或源码范围变化，cache key 也会变化，应预期第一轮重建。
