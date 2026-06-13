package repository

import (
	"context"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/wujunhui99/agents_im/service/auth/core/model"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

func TestPostgresCredentialCreateStoresSameAccountID(t *testing.T) {
	repo, mock, cleanup := newMockCredentialRepository(t)
	defer cleanup()

	now := time.Date(2026, 5, 2, 10, 0, 0, 0, time.UTC)
	accountID := "740000000000000003"
	mock.ExpectQuery(`(?s)insert\s+into\s+auth_credentials\s+\(account_id,\s+password_hash,\s+password_algo\)`).
		WithArgs(accountID, "hash", int16(1), "pg_alice", "alice@example.com", sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{
			"account_id",
			"identifier",
			"email_normalized",
			"email_verified_at",
			"password_hash",
			"password_algo",
			"created_at",
			"updated_at",
		}).AddRow(accountID, "pg_alice", "", nil, "hash", int16(1), now, now))

	got, err := repo.Create(context.Background(), model.Credential{
		Identifier:      "pg_alice",
		UserID:          accountID,
		Email:           "alice@example.com",
		EmailVerifiedAt: now,
		PasswordHash:    "hash",
		HashVersion:     "bcrypt-v1",
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
	if got.HashVersion != "bcrypt-v1" {
		t.Fatalf("credential hash version = %q, want bcrypt-v1", got.HashVersion)
	}
	if got.Salt != "" {
		t.Fatalf("credential salt = %q, want empty because Postgres bcrypt credentials do not store a separate salt", got.Salt)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestPostgresCredentialGetMapsPasswordAlgoOneToBcrypt(t *testing.T) {
	repo, mock, cleanup := newMockCredentialRepository(t)
	defer cleanup()

	now := time.Date(2026, 5, 4, 10, 0, 0, 0, time.UTC)
	accountID := "740000000000000004"
	mock.ExpectQuery(`(?s)select\s+c\.account_id,\s+a\.identifier,\s+a\.email_normalized,\s+a\.email_verified_at,\s+c\.password_hash,\s+c\.password_algo`).
		WithArgs("pg_bcrypt").
		WillReturnRows(sqlmock.NewRows([]string{
			"account_id",
			"identifier",
			"email_normalized",
			"email_verified_at",
			"password_hash",
			"password_algo",
			"created_at",
			"updated_at",
		}).AddRow(accountID, "pg_bcrypt", "", nil, "$2a$10$abcdefghijklmnopqrstuuI3qFoq8ZIRl4p8Q5fCq3dLtWq8B0Qpu", int16(1), now, now))

	got, err := repo.GetByIdentifier(context.Background(), "pg_bcrypt")
	if err != nil {
		t.Fatal(err)
	}
	if got.HashVersion != "bcrypt-v1" {
		t.Fatalf("credential hash version = %q, want bcrypt-v1", got.HashVersion)
	}
	if got.Salt != "" {
		t.Fatalf("credential salt = %q, want empty for bcrypt credentials", got.Salt)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestPostgresCredentialGetByEmailReadsEmailFromAccounts(t *testing.T) {
	repo, mock, cleanup := newMockCredentialRepository(t)
	defer cleanup()

	now := time.Date(2026, 5, 4, 11, 0, 0, 0, time.UTC)
	accountID := "740000000000000006"
	mock.ExpectQuery(`(?s)from\s+auth_credentials\s+c\s+join\s+accounts\s+a\s+on\s+a\.account_id\s+=\s+c\.account_id\s+where\s+a\.email_normalized\s+=\s+\$1`).
		WithArgs("alice@example.com").
		WillReturnRows(sqlmock.NewRows([]string{
			"account_id",
			"identifier",
			"email_normalized",
			"email_verified_at",
			"password_hash",
			"password_algo",
			"created_at",
			"updated_at",
		}).AddRow(accountID, "pg_email", "alice@example.com", now, "hash", int16(1), now, now))

	got, err := repo.GetByEmail(context.Background(), "alice@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if got.UserID != accountID || got.Identifier != "pg_email" || got.Email != "alice@example.com" || got.EmailVerifiedAt.IsZero() {
		t.Fatalf("unexpected credential by email: %+v", got)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestPostgresCredentialActiveSessionLifecycle(t *testing.T) {
	repo, mock, cleanup := newMockCredentialRepository(t)
	defer cleanup()

	issuedAt := time.Date(2026, 5, 4, 12, 0, 0, 0, time.UTC)
	expiresAt := issuedAt.Add(time.Hour)
	updatedAt := issuedAt.Add(time.Second)
	accountID := "740000000000000005"
	sessionID := "sid_postgres_test"

	mock.ExpectQuery(`(?s)update\s+auth_credentials\s+set\s+active_session_id\s+=\s+\$2`).
		WithArgs(accountID, sessionID, issuedAt, expiresAt).
		WillReturnRows(sqlmock.NewRows([]string{
			"account_id",
			"active_session_id",
			"active_session_issued_at",
			"active_session_expires_at",
			"updated_at",
		}).AddRow(accountID, sessionID, issuedAt, expiresAt, updatedAt))

	if err := repo.SetActiveSession(context.Background(), model.ActiveSession{
		UserID:    accountID,
		SessionID: sessionID,
		IssuedAt:  issuedAt,
		ExpiresAt: expiresAt,
	}); err != nil {
		t.Fatalf("set active session: %v", err)
	}

	mock.ExpectQuery(`(?s)select\s+account_id,\s+active_session_id,\s+active_session_issued_at,\s+active_session_expires_at,\s+updated_at\s+from\s+auth_credentials`).
		WithArgs(accountID).
		WillReturnRows(sqlmock.NewRows([]string{
			"account_id",
			"active_session_id",
			"active_session_issued_at",
			"active_session_expires_at",
			"updated_at",
		}).AddRow(accountID, sessionID, issuedAt, expiresAt, updatedAt))

	got, err := repo.GetActiveSession(context.Background(), accountID)
	if err != nil {
		t.Fatalf("get active session: %v", err)
	}
	if got.UserID != accountID || got.SessionID != sessionID || !got.IssuedAt.Equal(issuedAt) || !got.ExpiresAt.Equal(expiresAt) {
		t.Fatalf("unexpected active session: %+v", got)
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
