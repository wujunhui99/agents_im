package repository

import (
	"errors"
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
	if appconfig.ResolveStorageDriver(driver) != appconfig.StorageDriverPostgres {
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
	if appconfig.ResolveStorageDriver(driver) != appconfig.StorageDriverPostgres {
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
	if appconfig.ResolveStorageDriver(driver) != appconfig.StorageDriverPostgres {
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
