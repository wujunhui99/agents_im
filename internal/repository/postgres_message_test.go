package repository

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

func TestPostgresUserCanAccessMediaUsesVisibleMessagePredicate(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	repo := NewPostgresMessageRepositoryFromConn(sqlx.NewSqlConnFromDB(db))

	mock.ExpectQuery(`(?s)select exists \(.+m\.seq\s+>\s+s\.visible_start_seq.+m\.content_type in \(\$3, \$4\).+m\.content ->> 'mediaId' = \$2.+\)`).
		WithArgs("usr_receiver", "med_image", MessageContentTypeImage, MessageContentTypeFile).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))

	allowed, err := repo.UserCanAccessMedia(context.Background(), "usr_receiver", "med_image")
	if err != nil {
		t.Fatalf("UserCanAccessMedia: %v", err)
	}
	if !allowed {
		t.Fatal("UserCanAccessMedia allowed = false, want true")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
