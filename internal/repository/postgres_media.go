package repository

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"github.com/wujunhui99/agents_im/internal/apperror"
	"github.com/wujunhui99/agents_im/internal/model"
	"github.com/zeromicro/go-zero/core/stores/postgres"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type PostgresMediaRepository struct {
	conn sqlx.SqlConn
}

type postgresMediaRow struct {
	MediaID          string        `db:"media_id"`
	OwnerUserID      string        `db:"owner_user_id"`
	Bucket           string        `db:"bucket"`
	ObjectKey        string        `db:"object_key"`
	SHA256           string        `db:"sha256"`
	ContentType      string        `db:"content_type"`
	SizeBytes        int64         `db:"size_bytes"`
	Width            sql.NullInt32 `db:"width"`
	Height           sql.NullInt32 `db:"height"`
	OriginalFilename string        `db:"original_filename"`
	Purpose          string        `db:"purpose"`
	Status           string        `db:"status"`
	CreatedAt        time.Time     `db:"created_at"`
	UpdatedAt        time.Time     `db:"updated_at"`
}

func NewPostgresMediaRepository(dataSource string) (*PostgresMediaRepository, error) {
	dataSource = strings.TrimSpace(dataSource)
	if dataSource == "" {
		return nil, sql.ErrConnDone
	}
	return NewPostgresMediaRepositoryFromConn(postgres.New(dataSource)), nil
}

func NewPostgresMediaRepositoryFromConn(conn sqlx.SqlConn) *PostgresMediaRepository {
	return &PostgresMediaRepository{conn: conn}
}

func (r *PostgresMediaRepository) CreateMediaObject(ctx context.Context, media model.MediaObject) (model.MediaObject, error) {
	var row postgresMediaRow
	err := r.conn.QueryRowCtx(ctx, &row, `
insert into media_objects (
  media_id, owner_user_id, bucket, object_key, sha256, content_type, size_bytes,
  width, height, original_filename, purpose, status
)
values ($1, $2, $3, $4, $5, $6, $7, nullif($8, 0), nullif($9, 0), $10, $11, $12)
returning media_id, owner_user_id, bucket, object_key, sha256, content_type, size_bytes,
          width, height, original_filename, purpose, status, created_at, updated_at
`, media.MediaID, media.OwnerUserID, media.Bucket, media.ObjectKey, media.SHA256,
		media.ContentType, media.SizeBytes, media.Width, media.Height, media.OriginalFilename,
		media.Purpose, media.Status)
	if err != nil {
		if isPostgresUniqueViolation(err) {
			return model.MediaObject{}, apperror.AlreadyExists("media object already exists")
		}
		if isPostgresCheckViolation(err) {
			return model.MediaObject{}, apperror.InvalidArgument("invalid media object")
		}
		if isPostgresForeignKeyViolation(err) {
			return model.MediaObject{}, apperror.NotFound("owner account not found")
		}
		return model.MediaObject{}, err
	}
	return row.mediaObject(), nil
}

func (r *PostgresMediaRepository) GetMediaObject(ctx context.Context, mediaID string) (model.MediaObject, error) {
	var row postgresMediaRow
	err := r.conn.QueryRowCtx(ctx, &row, `
select media_id, owner_user_id, bucket, object_key, sha256, content_type, size_bytes,
       width, height, original_filename, purpose, status, created_at, updated_at
from media_objects
where media_id = $1
`, mediaID)
	if err != nil {
		if isNotFound(err) {
			return model.MediaObject{}, apperror.NotFound("media object not found")
		}
		return model.MediaObject{}, err
	}
	return row.mediaObject(), nil
}

func (r *PostgresMediaRepository) UpdateMediaStatus(ctx context.Context, mediaID string, status model.MediaStatus) (model.MediaObject, error) {
	var row postgresMediaRow
	err := r.conn.QueryRowCtx(ctx, &row, `
update media_objects
set status = $2, updated_at = now()
where media_id = $1
returning media_id, owner_user_id, bucket, object_key, sha256, content_type, size_bytes,
          width, height, original_filename, purpose, status, created_at, updated_at
`, mediaID, status)
	if err != nil {
		if isNotFound(err) {
			return model.MediaObject{}, apperror.NotFound("media object not found")
		}
		if isPostgresCheckViolation(err) {
			return model.MediaObject{}, apperror.InvalidArgument("invalid media status")
		}
		return model.MediaObject{}, err
	}
	return row.mediaObject(), nil
}

func (r postgresMediaRow) mediaObject() model.MediaObject {
	media := model.MediaObject{
		MediaID:          r.MediaID,
		OwnerUserID:      r.OwnerUserID,
		Bucket:           r.Bucket,
		ObjectKey:        r.ObjectKey,
		SHA256:           r.SHA256,
		ContentType:      r.ContentType,
		SizeBytes:        r.SizeBytes,
		OriginalFilename: r.OriginalFilename,
		Purpose:          model.MediaPurpose(r.Purpose),
		Status:           model.MediaStatus(r.Status),
		CreatedAt:        r.CreatedAt,
		UpdatedAt:        r.UpdatedAt,
	}
	if r.Width.Valid {
		media.Width = r.Width.Int32
	}
	if r.Height.Valid {
		media.Height = r.Height.Int32
	}
	return media
}
