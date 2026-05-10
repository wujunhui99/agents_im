package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/wujunhui99/agents_im/internal/apperror"
	"github.com/wujunhui99/agents_im/internal/auth/model"
	appconfig "github.com/wujunhui99/agents_im/internal/config"
	"github.com/zeromicro/go-zero/core/stores/postgres"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type PostgresRepository struct {
	conn sqlx.SqlConn
}

type postgresCredentialRow struct {
	AccountID       string       `db:"account_id"`
	Identifier      string       `db:"identifier"`
	Email           string       `db:"email_normalized"`
	EmailVerifiedAt sql.NullTime `db:"email_verified_at"`
	PasswordHash    string       `db:"password_hash"`
	PasswordAlgo    int16        `db:"password_algo"`
	CreatedAt       time.Time    `db:"created_at"`
	UpdatedAt       time.Time    `db:"updated_at"`
}

type postgresActiveSessionRow struct {
	AccountID string    `db:"account_id"`
	SessionID string    `db:"active_session_id"`
	IssuedAt  time.Time `db:"active_session_issued_at"`
	ExpiresAt time.Time `db:"active_session_expires_at"`
	UpdatedAt time.Time `db:"updated_at"`
}

type postgresEmailVerificationRow struct {
	ID           string       `db:"id"`
	Purpose      int16        `db:"purpose"`
	Email        string       `db:"email_normalized"`
	CodeHash     string       `db:"code_hash"`
	CodeHashAlgo int16        `db:"code_hash_algo"`
	ExpiresAt    time.Time    `db:"expires_at"`
	ConsumedAt   sql.NullTime `db:"consumed_at"`
	AttemptCount int          `db:"attempt_count"`
	LastSentAt   time.Time    `db:"last_sent_at"`
	CreatedAt    time.Time    `db:"created_at"`
	UpdatedAt    time.Time    `db:"updated_at"`
}

const pgUniqueViolationCode = "23505"
const pgForeignKeyViolationCode = "23503"

func NewPostgresRepository(dataSource string) (*PostgresRepository, error) {
	dataSource = strings.TrimSpace(dataSource)
	if dataSource == "" {
		return nil, errors.New("postgres datasource is required")
	}
	return NewPostgresRepositoryFromConn(postgres.New(dataSource)), nil
}

func NewPostgresRepositoryFromConn(conn sqlx.SqlConn) *PostgresRepository {
	return &PostgresRepository{conn: conn}
}

func NewRepositoryForStorage(driver string, dataSource string) (Repository, error) {
	storageDriver := appconfig.ResolveStorageDriver(driver)
	switch storageDriver {
	case appconfig.StorageDriverMemory:
		return NewMemoryRepository(), nil
	case appconfig.StorageDriverPostgres:
		return NewPostgresRepository(appconfig.ResolveDataSource(dataSource))
	default:
		return nil, fmt.Errorf("unsupported storage driver %q; use %q only for explicit dev/test memory mode or %q for PostgreSQL", storageDriver, appconfig.StorageDriverMemory, appconfig.StorageDriverPostgres)
	}
}

func (r *PostgresRepository) Create(ctx context.Context, credential model.Credential) (model.Credential, error) {
	passwordAlgo, err := passwordAlgoToDB(credential.HashVersion)
	if err != nil {
		return model.Credential{}, err
	}

	var row postgresCredentialRow
	err = r.conn.QueryRowCtx(ctx, &row, `
insert into auth_credentials (account_id, email_normalized, email_verified_at, password_hash, password_algo)
values ($1, $2, $3, $4, $5)
returning account_id, $6::text as identifier, email_normalized, email_verified_at, password_hash, password_algo, created_at, updated_at
`, credential.UserID, credential.Email, nullableTime(credential.EmailVerifiedAt), credential.PasswordHash, passwordAlgo, credential.Identifier)
	if err != nil {
		if isPgUniqueViolation(err) {
			return model.Credential{}, apperror.AlreadyExists("auth credential already exists")
		}
		if isPgForeignKeyViolation(err) {
			return model.Credential{}, apperror.NotFound("account not found")
		}
		return model.Credential{}, err
	}

	return row.credential()
}

func (r *PostgresRepository) SetActiveSession(ctx context.Context, session model.ActiveSession) error {
	session.UserID = strings.TrimSpace(session.UserID)
	session.SessionID = strings.TrimSpace(session.SessionID)
	if session.UserID == "" || session.SessionID == "" {
		return apperror.InvalidArgument("active session requires user_id and session_id")
	}

	var row postgresActiveSessionRow
	err := r.conn.QueryRowCtx(ctx, &row, `
update auth_credentials
set active_session_id = $2,
    active_session_issued_at = $3,
    active_session_expires_at = $4,
    updated_at = now()
where account_id = $1
returning account_id, active_session_id, active_session_issued_at, active_session_expires_at, updated_at
`, session.UserID, session.SessionID, session.IssuedAt.UTC(), session.ExpiresAt.UTC())
	if err != nil {
		if errors.Is(err, sqlx.ErrNotFound) {
			return apperror.NotFound("auth credential not found")
		}
		return err
	}
	return nil
}

func (r *PostgresRepository) GetActiveSession(ctx context.Context, userID string) (model.ActiveSession, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return model.ActiveSession{}, apperror.InvalidArgument("user_id is required")
	}

	var row postgresActiveSessionRow
	err := r.conn.QueryRowCtx(ctx, &row, `
select account_id, active_session_id, active_session_issued_at, active_session_expires_at, updated_at
from auth_credentials
where account_id = $1 and active_session_id <> ''
`, userID)
	if err != nil {
		if errors.Is(err, sqlx.ErrNotFound) {
			return model.ActiveSession{}, apperror.NotFound("active session not found")
		}
		return model.ActiveSession{}, err
	}
	return model.ActiveSession{
		UserID:    row.AccountID,
		SessionID: row.SessionID,
		IssuedAt:  row.IssuedAt,
		ExpiresAt: row.ExpiresAt,
		UpdatedAt: row.UpdatedAt,
	}, nil
}

func (r *PostgresRepository) GetByIdentifier(ctx context.Context, identifier string) (model.Credential, error) {
	var row postgresCredentialRow
	err := r.conn.QueryRowCtx(ctx, &row, `
select c.account_id, a.identifier, c.email_normalized, c.email_verified_at, c.password_hash, c.password_algo, c.created_at, c.updated_at
from auth_credentials c
join accounts a on a.account_id = c.account_id
where a.identifier = $1
`, identifier)
	if err != nil {
		if errors.Is(err, sqlx.ErrNotFound) {
			return model.Credential{}, apperror.NotFound("auth credential not found")
		}
		return model.Credential{}, err
	}

	return row.credential()
}

func (r *PostgresRepository) GetByEmail(ctx context.Context, email string) (model.Credential, error) {
	var row postgresCredentialRow
	err := r.conn.QueryRowCtx(ctx, &row, `
select c.account_id, a.identifier, c.email_normalized, c.email_verified_at, c.password_hash, c.password_algo, c.created_at, c.updated_at
from auth_credentials c
join accounts a on a.account_id = c.account_id
where c.email_normalized = $1 and c.email_normalized <> ''
`, email)
	if err != nil {
		if errors.Is(err, sqlx.ErrNotFound) {
			return model.Credential{}, apperror.NotFound("auth credential not found")
		}
		return model.Credential{}, err
	}

	return row.credential()
}

func (r *PostgresRepository) CreateEmailVerification(ctx context.Context, token model.EmailVerificationToken) (model.EmailVerificationToken, error) {
	purpose, err := emailVerificationPurposeToDB(token.Purpose)
	if err != nil {
		return model.EmailVerificationToken{}, err
	}
	codeHashAlgo, err := passwordAlgoToDB(token.CodeHashVersion)
	if err != nil {
		return model.EmailVerificationToken{}, err
	}
	var row postgresEmailVerificationRow
	err = r.conn.TransactCtx(ctx, func(ctx context.Context, session sqlx.Session) error {
		now := time.Now().UTC()
		if !token.CreatedAt.IsZero() {
			now = token.CreatedAt.UTC()
		}
		if _, err := session.ExecCtx(ctx, `
update auth_email_verification_tokens
set consumed_at = $3, updated_at = $3
where purpose = $1 and email_normalized = $2 and consumed_at is null
`, purpose, token.Email, now); err != nil {
			return err
		}
		return session.QueryRowCtx(ctx, &row, `
insert into auth_email_verification_tokens (
  id, purpose, email_normalized, code_hash, code_hash_algo, expires_at, attempt_count, last_sent_at
) values ($1, $2, $3, $4, $5, $6, $7, $8)
returning id, purpose, email_normalized, code_hash, code_hash_algo, expires_at, consumed_at, attempt_count, last_sent_at, created_at, updated_at
`, token.ID, purpose, token.Email, token.CodeHash, codeHashAlgo, token.ExpiresAt.UTC(), token.AttemptCount, token.LastSentAt.UTC())
	})
	if err != nil {
		if isPgUniqueViolation(err) {
			return model.EmailVerificationToken{}, apperror.AlreadyExists("email verification token already exists")
		}
		return model.EmailVerificationToken{}, err
	}
	return row.emailVerificationToken()
}

func (r *PostgresRepository) LatestEmailVerification(ctx context.Context, purpose string, email string) (model.EmailVerificationToken, error) {
	purposeValue, err := emailVerificationPurposeToDB(purpose)
	if err != nil {
		return model.EmailVerificationToken{}, err
	}
	var row postgresEmailVerificationRow
	err = r.conn.QueryRowCtx(ctx, &row, `
select id, purpose, email_normalized, code_hash, code_hash_algo, expires_at, consumed_at, attempt_count, last_sent_at, created_at, updated_at
from auth_email_verification_tokens
where purpose = $1 and email_normalized = $2
order by created_at desc, id desc
limit 1
`, purposeValue, email)
	if err != nil {
		if errors.Is(err, sqlx.ErrNotFound) {
			return model.EmailVerificationToken{}, apperror.NotFound("email verification token not found")
		}
		return model.EmailVerificationToken{}, err
	}
	return row.emailVerificationToken()
}

func (r *PostgresRepository) IncrementEmailVerificationAttempts(ctx context.Context, id string, now time.Time) (model.EmailVerificationToken, error) {
	var row postgresEmailVerificationRow
	err := r.conn.QueryRowCtx(ctx, &row, `
update auth_email_verification_tokens
set attempt_count = attempt_count + 1,
    updated_at = $2
where id = $1
returning id, purpose, email_normalized, code_hash, code_hash_algo, expires_at, consumed_at, attempt_count, last_sent_at, created_at, updated_at
`, id, now.UTC())
	if err != nil {
		if errors.Is(err, sqlx.ErrNotFound) {
			return model.EmailVerificationToken{}, apperror.NotFound("email verification token not found")
		}
		return model.EmailVerificationToken{}, err
	}
	return row.emailVerificationToken()
}

func (r *PostgresRepository) ConsumeEmailVerification(ctx context.Context, id string, now time.Time) (model.EmailVerificationToken, error) {
	var row postgresEmailVerificationRow
	err := r.conn.QueryRowCtx(ctx, &row, `
update auth_email_verification_tokens
set consumed_at = $2,
    attempt_count = attempt_count + 1,
    updated_at = $2
where id = $1 and consumed_at is null and expires_at > $2
returning id, purpose, email_normalized, code_hash, code_hash_algo, expires_at, consumed_at, attempt_count, last_sent_at, created_at, updated_at
`, id, now.UTC())
	if err != nil {
		if errors.Is(err, sqlx.ErrNotFound) {
			return model.EmailVerificationToken{}, apperror.InvalidArgument("email verification code is invalid or expired")
		}
		return model.EmailVerificationToken{}, err
	}
	return row.emailVerificationToken()
}

func (r postgresCredentialRow) credential() (model.Credential, error) {
	hashVersion, err := passwordAlgoFromDB(r.PasswordAlgo)
	if err != nil {
		return model.Credential{}, err
	}

	credential := model.Credential{
		Identifier:   r.Identifier,
		UserID:       r.AccountID,
		Email:        r.Email,
		PasswordHash: r.PasswordHash,
		HashVersion:  hashVersion,
		CreatedAt:    r.CreatedAt,
		UpdatedAt:    r.UpdatedAt,
	}
	if r.EmailVerifiedAt.Valid {
		credential.EmailVerifiedAt = r.EmailVerifiedAt.Time
	}
	return credential, nil
}

func (r postgresEmailVerificationRow) emailVerificationToken() (model.EmailVerificationToken, error) {
	purpose, err := emailVerificationPurposeFromDB(r.Purpose)
	if err != nil {
		return model.EmailVerificationToken{}, err
	}
	hashVersion, err := passwordAlgoFromDB(r.CodeHashAlgo)
	if err != nil {
		return model.EmailVerificationToken{}, err
	}
	token := model.EmailVerificationToken{
		ID:              r.ID,
		Purpose:         purpose,
		Email:           r.Email,
		CodeHash:        r.CodeHash,
		CodeHashVersion: hashVersion,
		ExpiresAt:       r.ExpiresAt,
		AttemptCount:    r.AttemptCount,
		LastSentAt:      r.LastSentAt,
		CreatedAt:       r.CreatedAt,
		UpdatedAt:       r.UpdatedAt,
	}
	if r.ConsumedAt.Valid {
		token.ConsumedAt = r.ConsumedAt.Time
	}
	return token, nil
}

func isPgUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolationCode
}

func isPgForeignKeyViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == pgForeignKeyViolationCode
}

func passwordAlgoToDB(version string) (int16, error) {
	switch version {
	case model.PasswordHashVersionBcrypt:
		return 1, nil
	default:
		return 0, apperror.InvalidArgument("unsupported password hash version")
	}
}

func passwordAlgoFromDB(algo int16) (string, error) {
	switch algo {
	case 1:
		return model.PasswordHashVersionBcrypt, nil
	case 2:
		return model.PasswordHashVersionLegacySHA256, nil
	default:
		return "", apperror.Internal("unsupported password algorithm")
	}
}

func emailVerificationPurposeToDB(purpose string) (int16, error) {
	switch purpose {
	case model.EmailVerificationPurposeRegister:
		return 1, nil
	default:
		return 0, apperror.InvalidArgument("unsupported email verification purpose")
	}
}

func emailVerificationPurposeFromDB(purpose int16) (string, error) {
	switch purpose {
	case 1:
		return model.EmailVerificationPurposeRegister, nil
	default:
		return "", apperror.Internal("unsupported email verification purpose")
	}
}

func nullableTime(t time.Time) sql.NullTime {
	if t.IsZero() {
		return sql.NullTime{}
	}
	return sql.NullTime{Time: t.UTC(), Valid: true}
}
