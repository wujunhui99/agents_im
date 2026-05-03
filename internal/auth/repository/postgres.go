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
	var row postgresCredentialRow
	err := r.conn.QueryRowCtx(ctx, &row, `
insert into auth_credentials (account_id, password_hash, password_algo)
values ($1, $2, $3)
returning account_id, $4::text as identifier, password_hash, password_algo, created_at, updated_at
`, credential.UserID, credential.PasswordHash, passwordAlgoToDB(credential.HashVersion), credential.Identifier)
	if err != nil {
		if isPgUniqueViolation(err) {
			return model.Credential{}, apperror.AlreadyExists("auth credential already exists")
		}
		if isPgForeignKeyViolation(err) {
			return model.Credential{}, apperror.NotFound("account not found")
		}
		return model.Credential{}, err
	}

	return row.credential(), nil
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

	return row.credential(), nil
}

func (r postgresCredentialRow) credential() model.Credential {
	return model.Credential{
		Identifier:   r.Identifier,
		UserID:       r.AccountID,
		PasswordHash: r.PasswordHash,
		HashVersion:  passwordAlgoFromDB(r.PasswordAlgo),
		CreatedAt:    r.CreatedAt,
		UpdatedAt:    r.UpdatedAt,
	}
}

func isPgUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolationCode
}

func isPgForeignKeyViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == pgForeignKeyViolationCode
}

func passwordAlgoToDB(version string) int16 {
	return 1
}

func passwordAlgoFromDB(algo int16) string {
	return "sha256-iter-v1"
}
