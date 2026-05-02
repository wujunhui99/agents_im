package svc

import (
	"github.com/wujunhui99/agents_im/internal/config"
	"github.com/wujunhui99/agents_im/internal/logic"
	"github.com/wujunhui99/agents_im/internal/objectstorage"
	"github.com/wujunhui99/agents_im/internal/repository"
)

type ServiceContext struct {
	Auth            config.JWTAuthConfig
	UserLogic       *logic.UserLogic
	FriendsLogic    *logic.FriendsLogic
	GroupsLogic     *logic.GroupsLogic
	MessageLogic    *logic.MessageLogic
	MediaLogic      *logic.MediaLogic
	AgentLogic      *logic.AgentLogic
	AgentAuditLogic *logic.AgentAuditLogic
	Repo            repository.Repository
	GroupsRepo      repository.GroupsRepository
	MessageRepo     repository.MessageRepository
	MediaRepo       repository.MediaRepository
	ObjectStore     objectstorage.ObjectStore
	AgentRepo       repository.AgentRepository
	OutboxRepo      repository.OutboxRepository
	DeliveryRepo    repository.DeliveryAttemptRepository
	AgentAuditRepo  repository.AgentAuditRepository
}

func NewServiceContext(repo repository.Repository) *ServiceContext {
	return NewServiceContextWithAuth(repo, config.DefaultJWTAuthConfig())
}

func NewServiceContextWithAuth(repo repository.Repository, auth config.JWTAuthConfig) *ServiceContext {
	userLogic := logic.NewUserLogic(repo)
	messageRepo := repository.NewMemoryMessageRepository()
	mediaRepo := repository.NewMemoryMediaRepository()
	objectStore := objectstorage.NewMemoryStore()
	mediaLogic := logic.NewMediaLogic(mediaRepo, objectStore, config.DefaultObjectStorageConfig().Bucket)
	agentAuditRepo := repository.NewMemoryAgentAuditRepository()
	return &ServiceContext{
		Auth:            normalizeAuthConfig(auth),
		UserLogic:       userLogic,
		FriendsLogic:    logic.NewFriendsLogic(repo, userLogic),
		MessageLogic:    logic.NewMessageLogicWithMediaValidator(messageRepo, logic.NewUserLogicExistenceChecker(userLogic), nil, mediaLogic),
		MediaLogic:      mediaLogic,
		AgentAuditLogic: logic.NewAgentAuditLogic(agentAuditRepo),
		Repo:            repo,
		MessageRepo:     messageRepo,
		MediaRepo:       mediaRepo,
		ObjectStore:     objectStore,
		OutboxRepo:      outboxRepositoryFromMessageRepo(messageRepo),
		DeliveryRepo:    deliveryAttemptRepositoryFromMessageRepo(messageRepo),
		AgentAuditRepo:  agentAuditRepo,
	}
}

func NewGroupsServiceContext(repo repository.GroupsRepository, userExists logic.UserExistenceChecker) *ServiceContext {
	return NewGroupsServiceContextWithAuth(repo, userExists, config.DefaultJWTAuthConfig())
}

func NewGroupsServiceContextWithAuth(repo repository.GroupsRepository, userExists logic.UserExistenceChecker, auth config.JWTAuthConfig) *ServiceContext {
	return &ServiceContext{
		Auth:        normalizeAuthConfig(auth),
		GroupsLogic: logic.NewGroupsLogic(repo, userExists),
		GroupsRepo:  repo,
	}
}

func NewMessageServiceContext(repo repository.MessageRepository, userExists logic.UserExistenceChecker, groups logic.GroupMemberLister) *ServiceContext {
	return NewMessageServiceContextWithAuth(repo, userExists, groups, config.DefaultJWTAuthConfig())
}

func NewMessageServiceContextWithAuth(repo repository.MessageRepository, userExists logic.UserExistenceChecker, groups logic.GroupMemberLister, auth config.JWTAuthConfig) *ServiceContext {
	mediaRepo := repository.NewMemoryMediaRepository()
	mediaLogic := logic.NewMediaLogic(mediaRepo, nil, config.DefaultObjectStorageConfig().Bucket)
	return &ServiceContext{
		Auth:         normalizeAuthConfig(auth),
		MessageLogic: logic.NewMessageLogicWithMediaValidator(repo, userExists, groups, mediaLogic),
		MessageRepo:  repo,
		MediaLogic:   mediaLogic,
		MediaRepo:    mediaRepo,
		OutboxRepo:   outboxRepositoryFromMessageRepo(repo),
		DeliveryRepo: deliveryAttemptRepositoryFromMessageRepo(repo),
	}
}

func NewUserServiceContextWithMedia(repo repository.Repository, mediaRepo repository.MediaRepository, objectStore objectstorage.ObjectStore, bucket string, auth config.JWTAuthConfig) *ServiceContext {
	ctx := NewServiceContextWithAuth(repo, auth)
	if mediaRepo != nil {
		ctx.MediaRepo = mediaRepo
	}
	if objectStore != nil {
		ctx.ObjectStore = objectStore
	}
	ctx.MediaLogic = logic.NewMediaLogic(ctx.MediaRepo, ctx.ObjectStore, bucket)
	if ctx.MessageLogic != nil {
		ctx.MessageLogic = ctx.MessageLogic.WithMediaValidator(ctx.MediaLogic)
	}
	return ctx
}

func NewMessageServiceContextWithMedia(repo repository.MessageRepository, mediaRepo repository.MediaRepository, userExists logic.UserExistenceChecker, groups logic.GroupMemberLister, auth config.JWTAuthConfig) *ServiceContext {
	mediaLogic := logic.NewMediaLogic(mediaRepo, nil, config.DefaultObjectStorageConfig().Bucket)
	return &ServiceContext{
		Auth:         normalizeAuthConfig(auth),
		MessageLogic: logic.NewMessageLogicWithMediaValidator(repo, userExists, groups, mediaLogic),
		MessageRepo:  repo,
		MediaLogic:   mediaLogic,
		MediaRepo:    mediaRepo,
		OutboxRepo:   outboxRepositoryFromMessageRepo(repo),
		DeliveryRepo: deliveryAttemptRepositoryFromMessageRepo(repo),
	}
}

func NewAgentServiceContext(repo repository.AgentRepository, accountTypeChecker logic.UserAccountTypeChecker) *ServiceContext {
	return NewAgentServiceContextWithAuth(repo, accountTypeChecker, config.DefaultJWTAuthConfig())
}

func NewAgentServiceContextWithAuth(repo repository.AgentRepository, accountTypeChecker logic.UserAccountTypeChecker, auth config.JWTAuthConfig) *ServiceContext {
	return &ServiceContext{
		Auth:       normalizeAuthConfig(auth),
		AgentLogic: logic.NewAgentLogic(repo, accountTypeChecker),
		AgentRepo:  repo,
	}
}

func NewAgentAuditServiceContext(repo repository.AgentAuditRepository) *ServiceContext {
	return &ServiceContext{
		AgentAuditLogic: logic.NewAgentAuditLogic(repo),
		AgentAuditRepo:  repo,
	}
}

func normalizeAuthConfig(auth config.JWTAuthConfig) config.JWTAuthConfig {
	defaults := config.DefaultJWTAuthConfig()
	if auth.AccessSecret == "" {
		auth.AccessSecret = defaults.AccessSecret
	}
	if auth.AccessExpire <= 0 {
		auth.AccessExpire = defaults.AccessExpire
	}
	return auth
}

func outboxRepositoryFromMessageRepo(repo repository.MessageRepository) repository.OutboxRepository {
	outboxRepo, _ := repo.(repository.OutboxRepository)
	return outboxRepo
}

func deliveryAttemptRepositoryFromMessageRepo(repo repository.MessageRepository) repository.DeliveryAttemptRepository {
	deliveryRepo, _ := repo.(repository.DeliveryAttemptRepository)
	return deliveryRepo
}
