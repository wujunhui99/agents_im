package repository

import (
	"context"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/wujunhui99/agents_im/internal/auth/model"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

func TestPostgresCredentialCreateStoresSameAccountID(t *testing.T) {
	repo, mock, cleanup := newMockCredentialRepository(t)
	defer cleanup()

	now := time.Date(2026, 5, 2, 10, 0, 0, 0, time.UTC)
	accountID := "740000000000000003"
	mock.ExpectQuery(`(?s)insert\s+into\s+auth_credentials\s+\(identifier,\s+user_id,\s+password_hash,\s+salt,\s+hash_version\)`).
		WithArgs("pg_alice", accountID, "hash", "salt", "v1").
		WillReturnRows(sqlmock.NewRows([]string{
			"identifier",
			"user_id",
			"password_hash",
			"salt",
			"hash_version",
			"created_at",
			"updated_at",
		}).AddRow("pg_alice", accountID, "hash", "salt", "v1", now, now))

	got, err := repo.Create(context.Background(), model.Credential{
		Identifier:   "pg_alice",
		UserID:       accountID,
		PasswordHash: "hash",
		Salt:         "salt",
		HashVersion:  "v1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.UserID != accountID {
		t.Fatalf("credential user_id/account id = %q, want %q", got.UserID, accountID)
	}
	if !regexp.MustCompile(`^[0-9]+$`).MatchString(got.UserID) {
		t.Fatalf("credential account id = %q, want unprefixed numeric string", got.UserID)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func newMockCredentialRepository(t *testing.T) (*PostgresRepository, sqlmock.Sqlmock, func()) {
	t.Helper()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	cleanup := func() {
		_ = db.Close()
	}
	return NewPostgresRepositoryFromConn(sqlx.NewSqlConnFromDB(db)), mock, cleanup
}
