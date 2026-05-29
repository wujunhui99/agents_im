package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"

	appconfig "github.com/wujunhui99/agents_im/pkg/config"
	"github.com/zeromicro/go-zero/core/stores/postgres"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type PostgresRepository struct {
	conn sqlx.SqlConn
	inTx bool
}

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

func newPostgresRepositoryFromSession(session sqlx.Session) *PostgresRepository {
	return &PostgresRepository{conn: sqlx.NewSqlConnFromSession(session), inTx: true}
}

func (r *PostgresRepository) TransactRepository(ctx context.Context, fn func(repo *PostgresRepository) error) error {
	if r.inTx {
		return fn(r)
	}
	return r.conn.TransactCtx(ctx, func(ctx context.Context, session sqlx.Session) error {
		return fn(newPostgresRepositoryFromSession(session))
	})
}

func (r *PostgresRepository) withTx(ctx context.Context, fn func(ctx context.Context, session sqlx.Session) error) error {
	if r.inTx {
		return fn(ctx, r.conn)
	}
	return r.conn.TransactCtx(ctx, fn)
}

func NewRepositoryForStorage(driver string, dataSource string) (Repository, error) {
	storageDriver, err := repositoryStorageDriver(driver)
	if err != nil {
		return nil, err
	}
	if storageDriver == appconfig.StorageDriverMemory {
		return NewMemoryRepository(), nil
	}
	return NewPostgresRepository(appconfig.ResolveDataSource(dataSource))
}

func NewGroupsRepositoryForStorage(driver string, dataSource string) (GroupsRepository, error) {
	storageDriver, err := repositoryStorageDriver(driver)
	if err != nil {
		return nil, err
	}
	if storageDriver == appconfig.StorageDriverMemory {
		return NewMemoryGroupsRepository(), nil
	}
	return NewPostgresGroupsRepository(appconfig.ResolveDataSource(dataSource))
}

func NewMessageRepositoryForStorage(driver string, dataSource string) (MessageRepository, error) {
	storageDriver, err := repositoryStorageDriver(driver)
	if err != nil {
		return nil, err
	}
	if storageDriver == appconfig.StorageDriverMemory {
		return NewMemoryMessageRepository(), nil
	}
	return NewPostgresMessageRepository(appconfig.ResolveDataSource(dataSource))
}

func NewMediaRepositoryForStorage(driver string, dataSource string) (MediaRepository, error) {
	storageDriver, err := repositoryStorageDriver(driver)
	if err != nil {
		return nil, err
	}
	if storageDriver == appconfig.StorageDriverMemory {
		return NewMemoryMediaRepository(), nil
	}
	return NewPostgresMediaRepository(appconfig.ResolveDataSource(dataSource))
}

func NewOutboxRepositoryForStorage(driver string, dataSource string) (OutboxRepository, error) {
	storageDriver, err := repositoryStorageDriver(driver)
	if err != nil {
		return nil, err
	}
	if storageDriver == appconfig.StorageDriverMemory {
		return NewMemoryMessageRepository(), nil
	}
	return NewPostgresMessageRepository(appconfig.ResolveDataSource(dataSource))
}

func NewAgentRepositoryForStorage(driver string, dataSource string) (AgentRepository, error) {
	storageDriver, err := repositoryStorageDriver(driver)
	if err != nil {
		return nil, err
	}
	if storageDriver == appconfig.StorageDriverMemory {
		return NewMemoryAgentRepository(), nil
	}
	return NewPostgresRepository(appconfig.ResolveDataSource(dataSource))
}

func NewAgentAuditRepositoryForStorage(driver string, dataSource string) (AgentAuditRepository, error) {
	storageDriver, err := repositoryStorageDriver(driver)
	if err != nil {
		return nil, err
	}
	if storageDriver == appconfig.StorageDriverMemory {
		return NewMemoryAgentAuditRepository(), nil
	}
	return NewPostgresAgentAuditRepository(appconfig.ResolveDataSource(dataSource))
}

func NewTaskReportRepositoryForStorage(driver string, dataSource string) (TaskReportRepository, error) {
	storageDriver, err := repositoryStorageDriver(driver)
	if err != nil {
		return nil, err
	}
	if storageDriver == appconfig.StorageDriverMemory {
		return NewMemoryTaskReportRepository(), nil
	}
	return NewPostgresTaskReportRepository(appconfig.ResolveDataSource(dataSource))
}

func NewAgentRegistryRepositoryForStorage(driver string, dataSource string) (AgentRegistryRepository, error) {
	storageDriver, err := repositoryStorageDriver(driver)
	if err != nil {
		return nil, err
	}
	if storageDriver == appconfig.StorageDriverMemory {
		return NewMemoryAgentRegistryRepository(), nil
	}
	return NewPostgresAgentRegistryRepository(appconfig.ResolveDataSource(dataSource))
}

func NewAgentConversationHostingRepositoryForStorage(driver string, dataSource string) (AgentConversationHostingRepository, error) {
	storageDriver, err := repositoryStorageDriver(driver)
	if err != nil {
		return nil, err
	}
	if storageDriver == appconfig.StorageDriverMemory {
		return NewMemoryAgentConversationHostingRepository(), nil
	}
	return NewPostgresAgentConversationHostingRepository(appconfig.ResolveDataSource(dataSource))
}

func NewConversationAIHostingRepositoryForStorage(driver string, dataSource string) (ConversationAIHostingRepository, error) {
	storageDriver, err := repositoryStorageDriver(driver)
	if err != nil {
		return nil, err
	}
	if storageDriver == appconfig.StorageDriverMemory {
		return NewMemoryConversationAIHostingRepository(), nil
	}
	return NewPostgresConversationAIHostingRepository(appconfig.ResolveDataSource(dataSource))
}

func repositoryStorageDriver(driver string) (string, error) {
	storageDriver := appconfig.ResolveStorageDriver(driver)
	switch storageDriver {
	case appconfig.StorageDriverMemory, appconfig.StorageDriverPostgres:
		return storageDriver, nil
	default:
		return "", fmt.Errorf("unsupported storage driver %q; use %q only for explicit dev/test memory mode or %q for PostgreSQL", storageDriver, appconfig.StorageDriverMemory, appconfig.StorageDriverPostgres)
	}
}
