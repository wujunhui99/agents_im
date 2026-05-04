package repository

import (
	"context"
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
	AccountID    string    `db:"account_id"`
	Identifier   string    `db:"identifier"`
	PasswordHash string    `db:"password_hash"`
	PasswordAlgo int16     `db:"password_algo"`
	CreatedAt    time.Time `db:"created_at"`
	UpdatedAt    time.Time `db:"updated_at"`
}

type postgresActiveSessionRow struct {
	AccountID string    `db:"account_id"`
	SessionID string    `db:"active_session_id"`
	IssuedAt  time.Time `db:"active_session_issued_at"`
	ExpiresAt time.Time `db:"active_session_expires_at"`
	UpdatedAt time.Time `db:"updated_at"`
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

func NewRepositoryForStorage(driver string, dataSource string) (CredentialRepository, error) {
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
insert into auth_credentials (account_id, password_hash, password_algo)
values ($1, $2, $3)
returning account_id, $4::text as identifier, password_hash, password_algo, created_at, updated_at
`, credential.UserID, credential.PasswordHash, passwordAlgo, credential.Identifier)
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
select c.account_id, a.identifier, c.password_hash, c.password_algo, c.created_at, c.updated_at
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

func (r postgresCredentialRow) credential() (model.Credential, error) {
	hashVersion, err := passwordAlgoFromDB(r.PasswordAlgo)
	if err != nil {
		return model.Credential{}, err
	}

	return model.Credential{
		Identifier:   r.Identifier,
		UserID:       r.AccountID,
		PasswordHash: r.PasswordHash,
		HashVersion:  hashVersion,
		CreatedAt:    r.CreatedAt,
		UpdatedAt:    r.UpdatedAt,
	}, nil
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
