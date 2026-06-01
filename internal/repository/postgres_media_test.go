package repository

import (
	"context"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/wujunhui99/agents_im/common/share/model"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

func TestPostgresMediaCreateMediaObjectCastsMetadataParameters(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	repo := NewPostgresMediaRepositoryFromConn(sqlx.NewSqlConnFromDB(db))
	createdAt := time.Date(2026, 5, 4, 15, 0, 0, 0, time.UTC)
	updatedAt := createdAt.Add(time.Second)

	mock.ExpectQuery(regexp.QuoteMeta(`
insert into media_objects (
  media_id, owner_account_id, bucket, object_key, original_filename, content_type,
  size_bytes, purpose, status, metadata
)
values ($1, $2, $3, $4, $5, $6, $7, $8::smallint, $9::smallint, jsonb_build_object('sha256', $10::text, 'width', $11::integer, 'height', $12::integer))
returning media_id, owner_account_id, bucket, object_key, original_filename, content_type,
          size_bytes, purpose, status, created_at, updated_at
`)).WithArgs(
		"med_regression", "309536626675863552", "agents-im-media", "users/309536626675863552/media/med_regression/hermes.jpeg",
		"hermes.jpeg", "image/jpeg", int64(11518), int16(2), int16(1), "", int32(200), int32(200),
	).WillReturnRows(sqlmock.NewRows([]string{
		"media_id", "owner_account_id", "bucket", "object_key", "original_filename", "content_type", "size_bytes", "purpose", "status", "created_at", "updated_at",
	}).AddRow(
		"med_regression", "309536626675863552", "agents-im-media", "users/309536626675863552/media/med_regression/hermes.jpeg",
		"hermes.jpeg", "image/jpeg", int64(11518), int16(2), int16(1), createdAt, updatedAt,
	))

	media, err := repo.CreateMediaObject(context.Background(), model.MediaObject{
		MediaID:          "med_regression",
		OwnerUserID:      "309536626675863552",
		Bucket:           "agents-im-media",
		ObjectKey:        "users/309536626675863552/media/med_regression/hermes.jpeg",
		OriginalFilename: "hermes.jpeg",
		ContentType:      "image/jpeg",
		SizeBytes:        11518,
		Purpose:          model.MediaPurposeMessageImage,
		Status:           model.MediaStatusPending,
		SHA256:           "",
		Width:            200,
		Height:           200,
	})
	if err != nil {
		t.Fatalf("CreateMediaObject: %v", err)
	}
	if media.MediaID != "med_regression" || media.Purpose != model.MediaPurposeMessageImage || media.Status != model.MediaStatusPending {
		t.Fatalf("unexpected media row: %+v", media)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
