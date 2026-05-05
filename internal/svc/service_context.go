package svc

import (
	"github.com/wujunhui99/agents_im/internal/agentim"
	einoruntime "github.com/wujunhui99/agents_im/internal/agentruntime/eino"
	authrepo "github.com/wujunhui99/agents_im/internal/auth/repository"
	"github.com/wujunhui99/agents_im/internal/config"
	"github.com/wujunhui99/agents_im/internal/logic"
	"github.com/wujunhui99/agents_im/internal/objectstorage"
	"github.com/wujunhui99/agents_im/internal/repository"
)

type ServiceContext struct {
	Auth             config.JWTAuthConfig
	AuthSessions     authrepo.ActiveSessionRepository
	AccountLogic     *logic.AccountLogic
	UserLogic        *logic.UserLogic
	FriendsLogic     *logic.FriendsLogic
	GroupsLogic      *logic.GroupsLogic
	MessageLogic     *logic.MessageLogic
	AIHostingLogic   *logic.ConversationAIHostingLogic
	MediaLogic       *logic.MediaLogic
	AgentLogic       *logic.AgentLogic
	AgentAuditLogic  *logic.AgentAuditLogic
	Repo             repository.Repository
	GroupsRepo       repository.GroupsRepository
	MessageRepo      repository.MessageRepository
	MediaRepo        repository.MediaRepository
	ObjectStore      objectstorage.ObjectStore
	AgentRepo        repository.AgentRepository
	AgentHostingRepo repository.AgentConversationHostingRepository
	AIHostingRepo    repository.ConversationAIHostingRepository
	OutboxRepo       repository.OutboxRepository
	AgentAuditRepo   repository.AgentAuditRepository
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
	mediaLogic.WithAttachmentAccessChecker(logic.NewMessageMediaAccessChecker(messageRepo))
	agentAuditRepo := repository.NewMemoryAgentAuditRepository()
	agentHostingRepo := repository.NewMemoryAgentConversationHostingRepository()
	aiHostingRepo := repository.NewMemoryConversationAIHostingRepository()
	return &ServiceContext{
		Auth:             normalizeAuthConfig(auth),
		AccountLogic:     userLogic,
		UserLogic:        userLogic,
		FriendsLogic:     logic.NewFriendsLogic(repo, userLogic),
		MessageLogic:     logic.NewMessageLogicWithMediaValidator(messageRepo, logic.NewUserLogicExistenceChecker(userLogic), nil, mediaLogic),
		AIHostingLogic:   logic.NewConversationAIHostingLogic(aiHostingRepo),
		MediaLogic:       mediaLogic,
		AgentAuditLogic:  logic.NewAgentAuditLogic(agentAuditRepo),
		Repo:             repo,
		MessageRepo:      messageRepo,
		MediaRepo:        mediaRepo,
		ObjectStore:      objectStore,
		OutboxRepo:       outboxRepositoryFromMessageRepo(messageRepo),
		AgentHostingRepo: agentHostingRepo,
		AIHostingRepo:    aiHostingRepo,
		AgentAuditRepo:   agentAuditRepo,
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
	mediaLogic.WithAttachmentAccessChecker(logic.NewMessageMediaAccessChecker(repo))
	agentHostingRepo := repository.NewMemoryAgentConversationHostingRepository()
	aiHostingRepo := repository.NewMemoryConversationAIHostingRepository()
	agentAuditRepo := repository.NewMemoryAgentAuditRepository()
	return &ServiceContext{
		Auth:             normalizeAuthConfig(auth),
		MessageLogic:     logic.NewMessageLogicWithMediaValidator(repo, userExists, groups, mediaLogic),
		AIHostingLogic:   logic.NewConversationAIHostingLogic(aiHostingRepo),
		MessageRepo:      repo,
		MediaLogic:       mediaLogic,
		MediaRepo:        mediaRepo,
		AgentHostingRepo: agentHostingRepo,
		AIHostingRepo:    aiHostingRepo,
		OutboxRepo:       outboxRepositoryFromMessageRepo(repo),
		AgentAuditLogic:  logic.NewAgentAuditLogic(agentAuditRepo),
		AgentAuditRepo:   agentAuditRepo,
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
	ConfigureMediaAttachmentAccess(ctx, ctx.MessageRepo)
	if ctx.MessageLogic != nil {
		ctx.MessageLogic = ctx.MessageLogic.WithMediaValidator(ctx.MediaLogic)
	}
	return ctx
}

func NewMessageServiceContextWithMedia(repo repository.MessageRepository, mediaRepo repository.MediaRepository, userExists logic.UserExistenceChecker, groups logic.GroupMemberLister, auth config.JWTAuthConfig) *ServiceContext {
	mediaLogic := logic.NewMediaLogic(mediaRepo, nil, config.DefaultObjectStorageConfig().Bucket)
	mediaLogic.WithAttachmentAccessChecker(logic.NewMessageMediaAccessChecker(repo))
	agentHostingRepo := repository.NewMemoryAgentConversationHostingRepository()
	aiHostingRepo := repository.NewMemoryConversationAIHostingRepository()
	agentAuditRepo := repository.NewMemoryAgentAuditRepository()
	return &ServiceContext{
		Auth:             normalizeAuthConfig(auth),
		MessageLogic:     logic.NewMessageLogicWithMediaValidator(repo, userExists, groups, mediaLogic),
		AIHostingLogic:   logic.NewConversationAIHostingLogic(aiHostingRepo),
		MessageRepo:      repo,
		MediaLogic:       mediaLogic,
		MediaRepo:        mediaRepo,
		AgentHostingRepo: agentHostingRepo,
		AIHostingRepo:    aiHostingRepo,
		OutboxRepo:       outboxRepositoryFromMessageRepo(repo),
		AgentAuditLogic:  logic.NewAgentAuditLogic(agentAuditRepo),
		AgentAuditRepo:   agentAuditRepo,
	}
}

func ConfigureMediaAttachmentAccess(ctx *ServiceContext, messageRepo repository.MessageRepository) {
	if ctx == nil || ctx.MediaLogic == nil || messageRepo == nil {
		return
	}
	ctx.MessageRepo = messageRepo
	ctx.MediaLogic.WithAttachmentAccessChecker(logic.NewMessageMediaAccessChecker(messageRepo))
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

func NewAgentConversationHostingServiceContext(repo repository.AgentConversationHostingRepository) *ServiceContext {
	return &ServiceContext{
		AgentHostingRepo: repo,
	}
}

func ConfigureConversationAIHosting(ctx *ServiceContext, deepSeek config.DeepSeekConfig) error {
	if ctx == nil {
		return nil
	}
	if ctx.MessageLogic == nil {
		return nil
	}
	if ctx.MessageRepo == nil {
		return nil
	}
	if ctx.AgentHostingRepo == nil {
		ctx.AgentHostingRepo = repository.NewMemoryAgentConversationHostingRepository()
	}
	if ctx.AIHostingRepo == nil {
		ctx.AIHostingRepo = repository.NewMemoryConversationAIHostingRepository()
	}
	if ctx.AIHostingLogic == nil {
		ctx.AIHostingLogic = logic.NewConversationAIHostingLogic(ctx.AIHostingRepo)
	}
	if ctx.AgentAuditRepo == nil {
		ctx.AgentAuditRepo = repository.NewMemoryAgentAuditRepository()
	}
	if ctx.AgentAuditLogic == nil {
		ctx.AgentAuditLogic = logic.NewAgentAuditLogic(ctx.AgentAuditRepo)
	}
	writer, err := agentim.NewMessageServiceResponseWriter(ctx.MessageLogic)
	if err != nil {
		return err
	}
	orchestrator, err := agentim.NewAgentRunOrchestrator(agentim.AgentRunOrchestratorConfig{
		Runtime: einoruntime.NewDeepSeekRuntime(deepSeek),
		RequestBuilder: agentim.NewConversationAIHostingRuntimeRequestBuilder(agentim.ConversationAIHostingRuntimeRequestBuilderConfig{
			MessageRepository: ctx.MessageRepo,
			HostingRepository: ctx.AIHostingRepo,
			DeepSeek:          deepSeek,
			MaxRecentMessages: 30,
		}),
		Audit:  ctx.AgentAuditLogic,
		Writer: writer,
	})
	if err != nil {
		return err
	}
	hosting, err := agentim.NewConversationHostingService(agentim.ConversationHostingConfig{
		Repository:          ctx.AgentHostingRepo,
		AIHostingRepository: ctx.AIHostingRepo,
		Runner:              orchestrator,
	})
	if err != nil {
		return err
	}
	ctx.MessageLogic.SetMessageCreatedHook(hosting)
	return nil
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
