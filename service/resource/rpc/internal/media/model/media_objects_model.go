package model

import (
	"context"
	"fmt"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ MediaObjectsModel = (*customMediaObjectsModel)(nil)

type (
	// MediaObjectsModel is an interface to be customized, add more methods here,
	// and implement the added methods in customMediaObjectsModel.
	MediaObjectsModel interface {
		mediaObjectsModel
		// CreateMediaObject 插入一条 media object 并返回入库行（含 DB 默认填充的
		// storage_provider/expires_at/时间戳）。metadata 传 JSON 文本（如
		// {"sha256":..,"width":..,"height":..}），purpose/status/digest_algo 传 vars.go 整型常量。
		CreateMediaObject(ctx context.Context, data *MediaObjects) (*MediaObjects, error)
		// FindReadyByObjectKey 取一行 status=ready 且 object_key 匹配的记录，用于文件级秒传命中
		// （object_key=agents_im/{sha256}，整文件去重下多行共享同 object_key，故 LIMIT 1）。
		// 无命中返回 ErrNotFound。
		FindReadyByObjectKey(ctx context.Context, objectKey string) (*MediaObjects, error)
		// MarkReady 把 pending 行落成 ready：写最终 object_key + digest_algo + status=ready，
		// 刷新 updated_at，返回更新后的行；无此 media_id 返回 ErrNotFound。
		MarkReady(ctx context.Context, mediaId int64, objectKey string, digestAlgo int64) (*MediaObjects, error)
	}

	customMediaObjectsModel struct {
		*defaultMediaObjectsModel
	}
)

// NewMediaObjectsModel returns a model for the database table.
func NewMediaObjectsModel(conn sqlx.SqlConn) MediaObjectsModel {
	return &customMediaObjectsModel{
		defaultMediaObjectsModel: newMediaObjectsModel(conn),
	}
}

func (m *customMediaObjectsModel) CreateMediaObject(ctx context.Context, data *MediaObjects) (*MediaObjects, error) {
	// 只写业务列，storage_provider/expires_at 交给 DB 默认（见 001_init）。digest_algo 显式写入
	// （pending tmp 行传 0；秒传 ready 行传 SHA256，object_key 已是 agents_im/{sha256}）。
	query := fmt.Sprintf(`insert into %s
  (media_id, uploader_id, bucket, object_key, original_filename, content_type,
   size_bytes, purpose, status, metadata, digest_algo)
values ($1, $2, $3, $4, $5, $6, $7, $8::smallint, $9::smallint, $10::jsonb, $11::smallint)
returning %s`, m.table, mediaObjectsRows)
	var resp MediaObjects
	err := m.conn.QueryRowCtx(ctx, &resp, query,
		data.MediaId, data.UploaderId, data.Bucket, data.ObjectKey, data.OriginalFilename,
		data.ContentType, data.SizeBytes, data.Purpose, data.Status, data.Metadata, data.DigestAlgo)
	switch err {
	case nil:
		return &resp, nil
	case sqlx.ErrNotFound:
		return nil, ErrNotFound
	default:
		return nil, err
	}
}

func (m *customMediaObjectsModel) FindReadyByObjectKey(ctx context.Context, objectKey string) (*MediaObjects, error) {
	query := fmt.Sprintf(`select %s from %s where object_key = $1 and status = $2::smallint
order by created_at asc limit 1`, mediaObjectsRows, m.table)
	var resp MediaObjects
	err := m.conn.QueryRowCtx(ctx, &resp, query, objectKey, MediaStatusReady)
	switch err {
	case nil:
		return &resp, nil
	case sqlx.ErrNotFound:
		return nil, ErrNotFound
	default:
		return nil, err
	}
}

func (m *customMediaObjectsModel) MarkReady(ctx context.Context, mediaId int64, objectKey string, digestAlgo int64) (*MediaObjects, error) {
	query := fmt.Sprintf(`update %s
set object_key = $2, digest_algo = $3::smallint, status = $4::smallint, updated_at = now()
where media_id = $1 returning %s`, m.table, mediaObjectsRows)
	var resp MediaObjects
	err := m.conn.QueryRowCtx(ctx, &resp, query, mediaId, objectKey, digestAlgo, MediaStatusReady)
	switch err {
	case nil:
		return &resp, nil
	case sqlx.ErrNotFound:
		return nil, ErrNotFound
	default:
		return nil, err
	}
}
