package repository

import (
	"errors"
	"fmt"
	"strings"

	appconfig "github.com/wujunhui99/agents_im/internal/config"
	"github.com/zeromicro/go-zero/core/stores/postgres"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type PostgresRepository struct {
	conn sqlx.SqlConn
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

func MustRepositoryForStorage(driver string, dataSource string) Repository {
	repo, err := NewRepositoryForStorage(driver, dataSource)
	if err != nil {
		panic(err)
	}
	return repo
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

func MustGroupsRepositoryForStorage(driver string, dataSource string) GroupsRepository {
	repo, err := NewGroupsRepositoryForStorage(driver, dataSource)
	if err != nil {
		panic(err)
	}
	return repo
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

func MustMessageRepositoryForStorage(driver string, dataSource string) MessageRepository {
	repo, err := NewMessageRepositoryForStorage(driver, dataSource)
	if err != nil {
		panic(err)
	}
	return repo
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

func MustOutboxRepositoryForStorage(driver string, dataSource string) OutboxRepository {
	repo, err := NewOutboxRepositoryForStorage(driver, dataSource)
	if err != nil {
		panic(err)
	}
	return repo
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

func MustAgentRepositoryForStorage(driver string, dataSource string) AgentRepository {
	repo, err := NewAgentRepositoryForStorage(driver, dataSource)
	if err != nil {
		panic(err)
	}
	return repo
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

func MustAgentRegistryRepositoryForStorage(driver string, dataSource string) AgentRegistryRepository {
	repo, err := NewAgentRegistryRepositoryForStorage(driver, dataSource)
	if err != nil {
		panic(err)
	}
	return repo
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
