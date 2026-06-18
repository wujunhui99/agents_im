package repository

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/wujunhui99/agents_im/pkg/model"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

func TestPostgresAccountSchemaUsesAccountsAndProfiles(t *testing.T) {
	migrationDir := filepath.Join("..", "..", "db", "migrations")
	initRaw, err := os.ReadFile(filepath.Join(migrationDir, "001_init_postgres.sql"))
	if err != nil {
		t.Fatal(err)
	}
	initMigration := string(initRaw)
	entries, err := os.ReadDir(migrationDir)
	if err != nil {
		t.Fatal(err)
	}
	var builder strings.Builder
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(migrationDir, entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		builder.Write(raw)
		builder.WriteString("\n")
	}
	migration := builder.String()

	for _, required := range []string{
		"create table if not exists accounts",
		"create table if not exists profiles",
		"account_id text primary key",
		"account_type smallint",
		"email_normalized text not null default ''",
		"email_verified_at timestamptz",
		"accounts_email_normalized_uniq",
		"drop index if exists auth_credentials_email_normalized_uniq",
		"drop column if exists email_normalized",
		"create table if not exists auth_credentials",
		"password_algo smallint",
		"birth_date date",
		"account_id text not null",
		"owner_account_id text",
	} {
		if !strings.Contains(migration, required) {
			t.Fatalf("migration missing %q", required)
		}
	}
	for _, forbidden := range []string{
		"create table if not exists users",
		"'usr_'",
		"age integer",
		"profiles_age_check",
		"references accounts(account_id)",
		"auth_credentials_user_id_account_fk",
		"friendships_user_id_account_fk",
		"media_objects_owner_account_fk",
	} {
		if strings.Contains(initMigration, forbidden) {
			t.Fatalf("initial account migration must not contain legacy account storage %q", forbidden)
		}
	}
	if strings.Contains(migration, "auth_credentials (account_id, email_normalized") {
		t.Fatal("auth_credentials inserts must not store email; email belongs to accounts")
	}

	avatarMigrationPath := filepath.Join("..", "..", "db", "migrations", "007_profile_avatar_url.sql")
	avatarRaw, err := os.ReadFile(avatarMigrationPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(avatarRaw), "add column if not exists avatar_url text not null default ''") {
		t.Fatal("avatar URL migration must add profiles.avatar_url")
	}
}

func TestPostgresCreateAccountWritesAccountsAndProfiles(t *testing.T) {
	repo, mock, cleanup := newMockPostgresRepository(t)
	defer cleanup()

	now := time.Date(2026, 5, 2, 10, 0, 0, 0, time.UTC)
	accountID := "740000000000000001"

	mock.ExpectBegin()
	mock.ExpectExec(`(?s)insert\s+into\s+accounts\s+\(account_id,\s+identifier,\s+account_type,\s+email_normalized,\s+email_verified_at\)`).
		WithArgs(accountID, "pg_alice", int16(1), "alice@example.com", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`(?s)insert\s+into\s+profiles\s+\(account_id,\s+display_name,\s+name,\s+gender,\s+birth_date,\s+region,\s+avatar_media_id,\s+avatar_url\)`).
		WithArgs(accountID, "Alice", "Alice", int16(0), "", "", "", "").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(`(?s)from\s+accounts\s+a\s+join\s+profiles\s+p\s+on\s+p\.account_id\s+=\s+a\.account_id`).
		WithArgs(accountID).
		WillReturnRows(postgresUserRows().AddRow(accountID, "pg_alice", int16(1), "alice@example.com", now, now, now, "Alice", "Alice", int16(0), "", "", "", "", now, now))
	mock.ExpectCommit()

	got, err := repo.Create(context.Background(), model.User{
		UserID:          accountID,
		Identifier:      "pg_alice",
		Email:           "alice@example.com",
		EmailVerifiedAt: now,
		DisplayName:     "Alice",
		Name:            "Alice",
		Gender:          "unknown",
		AccountType:     model.AccountTypeUser,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !regexp.MustCompile(`^[0-9]+$`).MatchString(got.UserID) {
		t.Fatalf("created account id = %q, want unprefixed numeric string", got.UserID)
	}
	if got.UserID != accountID || got.Identifier != "pg_alice" || got.Email != "alice@example.com" || got.DisplayName != "Alice" || got.AccountType != model.AccountTypeUser {
		t.Fatalf("created account mismatch: %+v", got)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestPostgresUpdateProfileWritesProfilesTable(t *testing.T) {
	repo, mock, cleanup := newMockPostgresRepository(t)
	defer cleanup()

	now := time.Date(2026, 5, 2, 10, 0, 0, 0, time.UTC)
	accountID := "740000000000000002"
	displayName := "Alice Updated"
	region := "Hangzhou"

	mock.ExpectQuery(`(?s)update\s+profiles\s+.*set\s+display_name\s+=\s+\$1,\s+name\s+=\s+\$2,\s+region\s+=\s+\$3`).
		WithArgs(displayName, displayName, region, accountID).
		WillReturnRows(sqlmock.NewRows([]string{"account_id"}).AddRow(accountID))
	mock.ExpectQuery(`(?s)from\s+accounts\s+a\s+join\s+profiles\s+p\s+on\s+p\.account_id\s+=\s+a\.account_id`).
		WithArgs(accountID).
		WillReturnRows(postgresUserRows().AddRow(accountID, "pg_alice", int16(1), "alice@example.com", now, now, now, displayName, displayName, int16(0), "", region, "", "", now, now))

	got, err := repo.UpdateProfile(context.Background(), accountID, ProfilePatch{
		DisplayName: &displayName,
		Name:        &displayName,
		Region:      &region,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.UserID != accountID || got.DisplayName != displayName || got.Region != region {
		t.Fatalf("updated profile mismatch: %+v", got)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestPostgresUpdateAvatarPersistsDurableAvatarURL(t *testing.T) {
	repo, mock, cleanup := newMockPostgresRepository(t)
	defer cleanup()

	now := time.Date(2026, 5, 6, 10, 0, 0, 0, time.UTC)
	accountID := "740000000000000003"
	avatarMediaID := "med_avatar_1"
	avatarURL := "/media/avatars/med_avatar_1"

	mock.ExpectQuery(`(?s)update\s+profiles\s+.*set\s+avatar_media_id\s+=\s+\$2,\s+avatar_url\s+=\s+\$3`).
		WithArgs(accountID, avatarMediaID, avatarURL).
		WillReturnRows(sqlmock.NewRows([]string{"account_id"}).AddRow(accountID))
	mock.ExpectQuery(`(?s)from\s+accounts\s+a\s+join\s+profiles\s+p\s+on\s+p\.account_id\s+=\s+a\.account_id`).
		WithArgs(accountID).
		WillReturnRows(postgresUserRows().AddRow(accountID, "pg_alice", int16(1), "alice@example.com", now, now, now, "Alice", "Alice", int16(0), "", "", avatarMediaID, avatarURL, now, now))

	got, err := repo.UpdateAvatar(context.Background(), accountID, avatarMediaID, avatarURL)
	if err != nil {
		t.Fatal(err)
	}
	if got.AvatarMediaID != avatarMediaID {
		t.Fatalf("avatar media id = %q, want %q", got.AvatarMediaID, avatarMediaID)
	}
	if got.AvatarURL != avatarURL {
		t.Fatalf("avatar url = %q, want %q", got.AvatarURL, avatarURL)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func newMockPostgresRepository(t *testing.T) (*PostgresRepository, sqlmock.Sqlmock, func()) {
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

func postgresUserRows() *sqlmock.Rows {
	return sqlmock.NewRows([]string{
		"account_id",
		"identifier",
		"account_type",
		"email_normalized",
		"email_verified_at",
		"account_created_at",
		"account_updated_at",
		"display_name",
		"name",
		"gender",
		"birth_date",
		"region",
		"avatar_media_id",
		"avatar_url",
		"profile_created_at",
		"profile_updated_at",
	})
}
