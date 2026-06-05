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
		// conversation_id/storage_provider/expires_at/时间戳）。metadata 传 JSON 文本（如
		// {"sha256":..,"width":..,"height":..}），purpose/status 传 vars.go 整型常量。
		CreateMediaObject(ctx context.Context, data *MediaObjects) (*MediaObjects, error)
		// UpdateStatus 改 status（并刷新 updated_at）并返回更新后的行；无此 media_id 返回 ErrNotFound。
		UpdateStatus(ctx context.Context, mediaId string, status int64) (*MediaObjects, error)
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
	// 只写业务列，conversation_id/storage_provider/expires_at 交给 DB 默认（见 001_init）。
	query := fmt.Sprintf(`insert into %s
  (media_id, owner_account_id, bucket, object_key, original_filename, content_type,
   size_bytes, purpose, status, metadata)
values ($1, $2, $3, $4, $5, $6, $7, $8::smallint, $9::smallint, $10::jsonb)
returning %s`, m.table, mediaObjectsRows)
	var resp MediaObjects
	err := m.conn.QueryRowCtx(ctx, &resp, query,
		data.MediaId, data.OwnerAccountId, data.Bucket, data.ObjectKey, data.OriginalFilename,
		data.ContentType, data.SizeBytes, data.Purpose, data.Status, data.Metadata)
	switch err {
	case nil:
		return &resp, nil
	case sqlx.ErrNotFound:
		return nil, ErrNotFound
	default:
		return nil, err
	}
}

func (m *customMediaObjectsModel) UpdateStatus(ctx context.Context, mediaId string, status int64) (*MediaObjects, error) {
	query := fmt.Sprintf(`update %s set status = $2, updated_at = now() where media_id = $1 returning %s`, m.table, mediaObjectsRows)
	var resp MediaObjects
	err := m.conn.QueryRowCtx(ctx, &resp, query, mediaId, status)
	switch err {
	case nil:
		return &resp, nil
	case sqlx.ErrNotFound:
		return nil, ErrNotFound
	default:
		return nil, err
	}
}
