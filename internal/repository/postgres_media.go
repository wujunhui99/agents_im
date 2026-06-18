package repository

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"github.com/wujunhui99/agents_im/pkg/model"
	"github.com/wujunhui99/agents_im/pkg/apperror"
	"github.com/zeromicro/go-zero/core/stores/postgres"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type PostgresMediaRepository struct {
	conn sqlx.SqlConn
}

type postgresMediaRow struct {
	MediaID          string    `db:"media_id"`
	OwnerUserID      string    `db:"owner_account_id"`
	Bucket           string    `db:"bucket"`
	ObjectKey        string    `db:"object_key"`
	ContentType      string    `db:"content_type"`
	SizeBytes        int64     `db:"size_bytes"`
	OriginalFilename string    `db:"original_filename"`
	Purpose          int16     `db:"purpose"`
	Status           int16     `db:"status"`
	CreatedAt        time.Time `db:"created_at"`
	UpdatedAt        time.Time `db:"updated_at"`
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
  media_id, owner_account_id, bucket, object_key, original_filename, content_type,
  size_bytes, purpose, status, metadata
)
values ($1, $2, $3, $4, $5, $6, $7, $8::smallint, $9::smallint, jsonb_build_object('sha256', $10::text, 'width', $11::integer, 'height', $12::integer))
returning media_id, owner_account_id, bucket, object_key, original_filename, content_type,
          size_bytes, purpose, status, created_at, updated_at
`, media.MediaID, media.OwnerUserID, media.Bucket, media.ObjectKey, media.OriginalFilename,
		media.ContentType, media.SizeBytes, mediaPurposeToDB(media.Purpose), mediaStatusToDB(media.Status),
		media.SHA256, media.Width, media.Height)
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
select media_id, owner_account_id, bucket, object_key, original_filename, content_type, size_bytes,
       purpose, status, created_at, updated_at
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
returning media_id, owner_account_id, bucket, object_key, original_filename, content_type, size_bytes,
          purpose, status, created_at, updated_at
`, mediaID, mediaStatusToDB(status))
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
		ContentType:      r.ContentType,
		SizeBytes:        r.SizeBytes,
		OriginalFilename: r.OriginalFilename,
		Purpose:          mediaPurposeFromDB(r.Purpose),
		Status:           mediaStatusFromDB(r.Status),
		CreatedAt:        r.CreatedAt,
		UpdatedAt:        r.UpdatedAt,
	}
	return media
}
