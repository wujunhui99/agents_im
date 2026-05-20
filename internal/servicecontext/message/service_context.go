package message

import (
	"github.com/wujunhui99/agents_im/internal/agentim"
	einoruntime "github.com/wujunhui99/agents_im/internal/agentruntime/eino"
	"github.com/wujunhui99/agents_im/internal/config"
	"github.com/wujunhui99/agents_im/internal/llmobs"
	"github.com/wujunhui99/agents_im/internal/logic"
	"github.com/wujunhui99/agents_im/internal/repository"
	"github.com/wujunhui99/agents_im/internal/servicecontext/common"
)

type ServiceContext struct {
	common.AuthRuntime
	MessageLogic     *logic.MessageLogic
	AIHostingLogic   *logic.ConversationAIHostingLogic
	MediaLogic       *logic.MediaLogic
	MessageRepo      repository.MessageRepository
	MediaRepo        repository.MediaRepository
	AgentHostingRepo repository.AgentConversationHostingRepository
	AIHostingRepo    repository.ConversationAIHostingRepository
	GroupMembers     logic.GroupMemberLister
	OutboxRepo       repository.OutboxRepository
	AgentAuditLogic  *logic.AgentAuditLogic
	AgentAuditRepo   repository.AgentAuditRepository
	AgentResolver    logic.AgentAccountExistenceChecker
}

func NewServiceContext(repo repository.MessageRepository, userExists logic.UserExistenceChecker, groups logic.GroupMemberLister) *ServiceContext {
	return NewServiceContextWithAuth(repo, userExists, groups, config.DefaultJWTAuthConfig())
}

func NewServiceContextWithAuth(repo repository.MessageRepository, userExists logic.UserExistenceChecker, groups logic.GroupMemberLister, auth config.JWTAuthConfig) *ServiceContext {
	mediaRepo := repository.NewMemoryMediaRepository()
	return NewServiceContextWithMedia(repo, mediaRepo, userExists, groups, auth)
}

func NewServiceContextWithMedia(repo repository.MessageRepository, mediaRepo repository.MediaRepository, userExists logic.UserExistenceChecker, groups logic.GroupMemberLister, auth config.JWTAuthConfig) *ServiceContext {
	mediaLogic := logic.NewMediaLogic(mediaRepo, nil, config.DefaultObjectStorageConfig().Bucket)
	mediaLogic.WithAttachmentAccessChecker(logic.NewMessageMediaAccessChecker(repo))
	agentHostingRepo := repository.NewMemoryAgentConversationHostingRepository()
	aiHostingRepo := repository.NewMemoryConversationAIHostingRepository()
	agentAuditRepo := repository.NewMemoryAgentAuditRepository()
	return &ServiceContext{
		AuthRuntime:      common.NewAuthRuntime(auth),
		MessageLogic:     logic.NewMessageLogicWithMediaValidator(repo, userExists, groups, mediaLogic),
		AIHostingLogic:   logic.NewConversationAIHostingLogic(aiHostingRepo),
		MediaLogic:       mediaLogic,
		MessageRepo:      repo,
		MediaRepo:        mediaRepo,
		AgentHostingRepo: agentHostingRepo,
		AIHostingRepo:    aiHostingRepo,
		GroupMembers:     groups,
		OutboxRepo:       outboxRepositoryFromMessageRepo(repo),
		AgentAuditLogic:  logic.NewAgentAuditLogic(agentAuditRepo),
		AgentAuditRepo:   agentAuditRepo,
	}
}

func ConfigureConversationAIHosting(ctx *ServiceContext, deepSeek config.DeepSeekConfig, obs config.LLMObservabilityConfig) error {
	if ctx == nil || ctx.MessageLogic == nil || ctx.MessageRepo == nil {
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
	if ctx.AgentResolver != nil {
		ctx.AIHostingLogic.WithAgentAccountResolver(ctx.AgentResolver)
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
	llmObsConfig := llmObservabilityConfig(obs)
	llmObsSink, err := llmobs.NewSink(llmObsConfig)
	if err != nil {
		return err
	}
	orchestrator, err := agentim.NewAgentRunOrchestrator(agentim.AgentRunOrchestratorConfig{
		Runtime: einoruntime.NewDeepSeekRuntime(deepSeek, einoruntime.WithLLMObservability(llmObsSink, llmObsConfig)),
		RequestBuilder: agentim.NewConversationAIHostingRuntimeRequestBuilder(agentim.ConversationAIHostingRuntimeRequestBuilderConfig{
			MessageRepository: ctx.MessageRepo,
			HostingRepository: ctx.AIHostingRepo,
			AgentRepository:   agentRepositoryFromResolver(ctx.AgentResolver),
			DeepSeek:          deepSeek,
			MaxRecentMessages: 30,
		}),
		Audit:                ctx.AgentAuditLogic,
		Writer:               writer,
		LLMObservabilitySink: llmObsSink,
	})
	if err != nil {
		return err
	}
	hosting, err := agentim.NewConversationHostingService(agentim.ConversationHostingConfig{
		Repository:           ctx.AgentHostingRepo,
		AIHostingRepository:  ctx.AIHostingRepo,
		Runner:               orchestrator,
		AgentAccountResolver: ctx.AgentResolver,
		GroupMembers:         ctx.GroupMembers,
	})
	if err != nil {
		return err
	}
	ctx.MessageLogic.SetMessageCreatedHook(hosting)
	return nil
}

func agentRepositoryFromResolver(resolver logic.AgentAccountExistenceChecker) repository.AgentRepository {
	if typed, ok := resolver.(interface {
		AgentRepository() repository.AgentRepository
	}); ok {
		return typed.AgentRepository()
	}
	return nil
}

func llmObservabilityConfig(obs config.LLMObservabilityConfig) llmobs.Config {
	return llmobs.Config{
		Enabled:        obs.Enabled,
		Backend:        obs.Backend,
		CaptureOutput:  obs.CaptureOutput,
		MaxOutputBytes: obs.MaxOutputBytes,
		Langfuse: llmobs.LangfuseConfig{
			Host:      obs.Langfuse.Host,
			PublicKey: obs.Langfuse.PublicKey,
			SecretKey: obs.Langfuse.SecretKey,
		},
	}
}

func outboxRepositoryFromMessageRepo(repo repository.MessageRepository) repository.OutboxRepository {
	outboxRepo, _ := repo.(repository.OutboxRepository)
	return outboxRepo
}
