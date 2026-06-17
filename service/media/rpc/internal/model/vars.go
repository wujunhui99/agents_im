package model

import "github.com/zeromicro/go-zero/core/stores/sqlx"

var ErrNotFound = sqlx.ErrNotFound

// media_objects 整型枚举取值（与 db/migrations smallint 列一致，是这些常量的唯一来源）。
// purpose/status 的字符串契约在 logic 层（media_rules.go）映射。
const (
	MediaPurposeAvatar       int64 = 1
	MediaPurposeMessageImage int64 = 2
	MediaPurposeMessageFile  int64 = 3
	MediaPurposeAgentSkill   int64 = 4

	MediaStatusPending  int64 = 1
	MediaStatusReady    int64 = 2
	MediaStatusRejected int64 = 3
	MediaStatusDeleted  int64 = 4

	// StorageProviderRustFS 是建库默认 storage_provider（见 001_init storage_provider default 1）。
	// 值 1 历史上代表 MinIO，现承载 RustFS（同 S3 协议，#569）；DB 枚举值不变。
	StorageProviderRustFS int64 = 1

	// digest_algo：object_key 承载的哈希算法（EPIC #527 §2，见 021 migration）。
	// 0=未指定（object_key 尚未迁到 agents_im/{sha256}，#546 OSS 迁移前的存量行）；
	// 1=SHA256（object_key=agents_im/{整文件 sha256}）。建库默认 0，不谎称 SHA256。
	MediaDigestAlgoUnspecified int64 = 0
	MediaDigestAlgoSHA256      int64 = 1
)
