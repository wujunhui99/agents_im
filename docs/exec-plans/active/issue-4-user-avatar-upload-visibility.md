# Issue 4 User Avatar Upload And Contact Visibility

状态：Completed

## 背景

GitHub Issue #4 要求用户在“我的”页通过真实媒体对象存储链路上传头像，并让本人、已接受联系人、单聊会话列表和聊天头部看到头像。当前代码已存在 `profiles.avatar_media_id`、`PATCH /me/avatar`、`/media/uploads` 和 media ready/owner/purpose 校验，但响应没有头像展示 URL，好友 API 也没有使用真实 media repository/object store 来生成联系人头像展示数据。

## 目标

- 前端“我的”页支持选择头像，校验静态图片 MIME/大小，尽量压缩到约 512px/256KiB，再按 `/media/uploads -> PUT -> complete -> PATCH /me/avatar` 顺序更新资料。
- 后端在当前用户和已接受联系人响应中返回短期授权的 `avatar_url` / `avatar_url_expires_at`，不返回 object key 或存储凭据到 UI。
- 后端继续只允许当前用户绑定自己 ready、purpose=`avatar`、静态图片、大小不超过 5 MiB 的 media。
- 联系人列表、单聊会话列表和聊天头部优先使用 peer avatar，缺失或加载失败时显示干净占位。
- 增加 focused backend/frontend tests 和 avatar E2E regression harness classifications。

## 非目标

- 不改动 Issue #1 图片消息发送模型，也不依赖 Issue #2 群聊分支。
- 不新增裁剪 UI。
- 不存储图片 bytes 到 PostgreSQL。
- 不在 UI 中展示内部 account id、object key、签名、token 或永久私有 URL。

## 任务拆分

- [x] Task 1：后端 tests-first，覆盖 avatar 静态 MIME/size 校验、`PATCH /me/avatar` profile update、`/me` 和 `/friends` avatar display URL 映射。
- [x] Task 2：后端实现 `MediaLogic` avatar display URL、`User`/`FriendProfile` response fields、user/friends go-zero mapping、friends-api media repo/object store wiring。
- [x] Task 3：前端 tests-first，覆盖 profile avatar upload order、validation failure、success UI update、contacts/messages avatar rendering and fallback、Chinese failure copy。
- [x] Task 4：前端实现 reusable avatar image component props、avatar upload preparation/compression helper、MePage upload UI、UserApi avatar patch、contacts/messages profile avatar propagation。
- [x] Task 5：增加 `tests/e2e/avatar_upload_visibility_regression.mjs` 和 README 说明，分类 `avatar-upload-success`、`avatar-validation-failed`、`avatar-visibility-success`、`setup-or-harness-failed`，输出脱敏。
- [x] Task 6：运行 focused 和全量可行验证，记录 blockers；通过后按用户指定提交信息提交。

## 决策日志

| 时间 | 决策 | 原因 |
| --- | --- | --- |
| 2026-05-05 | 使用现有 `avatar_media_id`，新增响应展示字段 `avatar_url` / `avatar_url_expires_at` | 避免重复 schema/API wrapper，同时让前端无需暴露 object key 或自行拼接存储 URL。 |
| 2026-05-05 | Avatar 展示 URL 由后端 media logic 签发，TTL 使用 24h | Issue 指出头像 URL 不应短期过期，同时不能使用永久私有 URL。 |
| 2026-05-05 | V1 前端拒绝 GIF 头像，后端 avatar purpose 也只允许 JPEG/PNG/WebP 绑定 | Issue 非目标要求 V1 不支持 animated avatars；消息图片仍可继续支持 GIF。 |
| 2026-05-05 | `friends-api` 启动时接入真实 media repository/object store | 已接受联系人需要通过 `/friends` 获得 avatar display URL，内存 media repo 会导致生产不可见。 |

## 验证方式

Focused:

```bash
go test ./internal/logic ./internal/logic/user ./internal/logic/friends ./internal/repository
npm --prefix web test -- --run MePage ContactsPage MessagesPage user
```

Required best effort:

```bash
go test ./...
npm --prefix web test -- --run
npm --prefix web run build
bash scripts/verify-static.sh
git diff --check
```

If Docker/PostgreSQL/Playwright or object storage are unavailable, only unit/contract/static verification will be claimed and the blocker will be recorded.

## 风险与回滚

- Avatar URL signing now participates in `/me` and `/friends`; if object storage is unavailable while a profile has an avatar, those calls fail visibly instead of showing stale fake success.
- `friends-api` now depends on media repository and object storage config. Rollback is to remove avatar URL fields and use only `avatar_media_id`, but that would fail Issue #4 visibility acceptance.
- Frontend uses object URLs only during browser-side image processing and revokes them; final displayed avatar must come from backend `avatar_url`.

## 结果记录

已完成 backend/frontend/unit/contract/static verification。已运行：

- `PATH=/tmp/go/bin:$HOME/go/bin:$PATH go test ./...`
- `npm --prefix web test -- --run`
- `npm --prefix web run build`
- `bash scripts/verify-static.sh`
- `git diff --check`
- `PATH=/tmp/go/bin:$HOME/go/bin:$PATH bash -lc 'for f in api/*.api; do goctl api validate -api "$f"; done'`
- `node --check tests/e2e/avatar_upload_visibility_regression.mjs`
- `node tests/e2e/avatar_upload_visibility_redaction_check.mjs`

未执行 live production/local E2E，因为本任务未启动真实 API、对象存储和浏览器会话；已提交可复用 API-level regression harness。
