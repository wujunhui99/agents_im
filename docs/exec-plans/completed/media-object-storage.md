# Media Object Storage

状态：Completed

## 背景

用户头像、图片消息和文件消息需要从纯消息 JSON 扩展到 MinIO/S3-compatible object storage。第一阶段目标是完成后端、基础设施和契约，不做完整前端上传 UI。

## 目标

- 增加对象存储抽象和 MinIO 实现，测试/显式 dev mode 使用 memory store。
- 增加 `media_objects` 元数据模型、PostgreSQL migration、memory/PostgreSQL repository。
- 增加受保护的媒体上传意图、上传完成、下载 URL REST contract。
- 增加 `/me/avatar` 头像绑定 contract。
- 扩展消息 `text` / `image` / `file` 内容校验，图片/文件必须引用 ready 且归发送者所有的 media。
- 更新本地和部署 middleware MinIO 配置、文档和静态检查。

## 非目标

- 不实现前端上传 UI。
- 不实现 conversation participant 下载授权；phase 1 下载 URL 仅 owner 可取。
- 不把 Agent skill 文件表迁移到通用 media 表。

## 任务拆分

- [x] Task 1：添加 object storage interface、MinIO implementation、memory implementation。
- [x] Task 2：添加 media metadata model/repository/migration。
- [x] Task 3：添加 Media REST API contract 和 user-api route wiring。
- [x] Task 4：添加 avatar media validation 和 `/me/avatar`。
- [x] Task 5：扩展 message validation 和 repository content validation。
- [x] Task 6：更新 MinIO compose/env/deploy/docs/static checks。
- [x] Task 7：补充 media/message focused tests。

## 决策日志

| 时间 | 决策 | 原因 |
| --- | --- | --- |
| 2026-05-01 | Phase 1 media REST routes 挂在 `user-api`，不新增独立进程。 | 当前部署矩阵没有 media-api；user-api 已持有用户身份和头像 profile 更新入口。 |
| 2026-05-01 | Message API/RPC/Gateway 只依赖 media metadata repository，不依赖 object store。 | 发送消息只需要验证 `mediaId` 的 owner/purpose/status/type/size；对象读写由 Media REST 负责。 |
| 2026-05-01 | 下载 URL phase 1 owner-only。 | 会话参与者附件读取需要结合 message authorization，留到前端媒体消息联调阶段。 |

## 验证方式

```bash
export PATH=/tmp/go/bin:$HOME/go/bin:$PATH
goctl --version
for f in api/*.api; do goctl api validate -api "$f"; done
gofmt -w $(find . -name '*.go' -print)
go test ./...
bash scripts/verify-static.sh
docker compose -f deploy/middleware/docker-compose.yml config
bash -n scripts/dev-up.sh
bash -n scripts/dev-demo-data.sh
git diff --check
```

## 风险与回滚

- MinIO 凭据或 endpoint 配错时 `user-api` 会启动失败或媒体请求失败，不回退到 memory store。
- 已生成的 presigned PUT URL 不强制对象大小；`complete` 会通过 object stat 校验 size/content type 后才置为 ready。
- 回滚需同时移除 `/media` 契约、`media_objects` migration 使用方和 message image/file 校验。

## 结果记录

- 实现完成于 `feature/media-object-storage`。
- 验证结果记录在最终任务回复中。
