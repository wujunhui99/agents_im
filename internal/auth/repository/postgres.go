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
	Identifier   string    `db:"identifier"`
	UserID       string    `db:"user_id"`
	PasswordHash string    `db:"password_hash"`
	Salt         string    `db:"salt"`
	HashVersion  string    `db:"hash_version"`
	CreatedAt    time.Time `db:"created_at"`
	UpdatedAt    time.Time `db:"updated_at"`
}

const pgUniqueViolationCode = "23505"

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
	var row postgresCredentialRow
	err := r.conn.QueryRowCtx(ctx, &row, `
insert into auth_credentials (identifier, user_id, password_hash, salt, hash_version)
values ($1, $2, $3, $4, $5)
returning identifier, user_id, password_hash, salt, hash_version, created_at, updated_at
`, credential.Identifier, credential.UserID, credential.PasswordHash, credential.Salt, credential.HashVersion)
	if err != nil {
		if isPgUniqueViolation(err) {
			return model.Credential{}, apperror.AlreadyExists("auth credential already exists")
		}
		return model.Credential{}, err
	}

	return row.credential(), nil
}

func (r *PostgresRepository) GetByIdentifier(ctx context.Context, identifier string) (model.Credential, error) {
	var row postgresCredentialRow
	err := r.conn.QueryRowCtx(ctx, &row, `
select identifier, user_id, password_hash, salt, hash_version, created_at, updated_at
from auth_credentials
where identifier = $1
`, identifier)
	if err != nil {
		if errors.Is(err, sqlx.ErrNotFound) {
			return model.Credential{}, apperror.NotFound("auth credential not found")
		}
		return model.Credential{}, err
	}

	return row.credential(), nil
}

func (r postgresCredentialRow) credential() model.Credential {
	return model.Credential{
		Identifier:   r.Identifier,
		UserID:       r.UserID,
		PasswordHash: r.PasswordHash,
		Salt:         r.Salt,
		HashVersion:  r.HashVersion,
		CreatedAt:    r.CreatedAt,
		UpdatedAt:    r.UpdatedAt,
	}
}

func isPgUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolationCode
}
