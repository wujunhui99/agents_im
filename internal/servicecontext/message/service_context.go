package message

import (
	"context"

	"github.com/wujunhui99/agents_im/internal/agentim"
	einoruntime "github.com/wujunhui99/agents_im/internal/agentruntime/eino"
	runtimetools "github.com/wujunhui99/agents_im/internal/agentruntime/tools"
	"github.com/wujunhui99/agents_im/internal/logic"
	"github.com/wujunhui99/agents_im/internal/mediavalidate"
	"github.com/wujunhui99/agents_im/internal/repository"
	"github.com/wujunhui99/agents_im/internal/servicecontext/common"
	"github.com/wujunhui99/agents_im/pkg/apperror"
	"github.com/wujunhui99/agents_im/pkg/config"
	"github.com/wujunhui99/agents_im/pkg/llmobs"
	"github.com/wujunhui99/agents_im/pkg/pythonexec"
)

type ServiceContext struct {
	common.AuthRuntime
	MessageLogic      *logic.MessageLogic
	AgentMessageHook  logic.MessageCreatedHook
	AIHostingLogic    *logic.ConversationAIHostingLogic
	FeedbackLogic     *logic.FeedbackLogic
	MessageRepo       repository.MessageRepository
	MediaRepo         repository.MediaRepository
	FeedbackRepo      repository.FeedbackRepository
	AgentHostingRepo  repository.AgentConversationHostingRepository
	AIHostingRepo     repository.ConversationAIHostingRepository
	GroupMembers      logic.GroupMemberLister
	AgentAuditLogic   *logic.AgentAuditLogic
	AgentAuditRepo    repository.AgentAuditRepository
	AgentResolver     logic.AgentAccountExistenceChecker
	AccountRepo       repository.Repository
	AgentRepo         repository.AgentRepository
	AgentRegistryRepo repository.AgentRegistryRepository
	PythonExecutor    pythonexec.Executor
	// AgentResponseSender 覆盖 AI 写回通道（默认 MessageLogic 直写 PG）。
	// Kafka 模式（03 §9 B2）由 msg-rpc 注入「经自身 SendMessage 走 Kafka」的实现。
	AgentResponseSender agentim.MessageSender
}

type ConversationAIHostingRuntimeOptions struct {
	DeepSeek         config.DeepSeekConfig
	LLMObservability config.LLMObservabilityConfig
	AgentRegistry    repository.AgentRegistryRepository
	PythonExecutor   pythonexec.Executor
	AgentCreate      runtimetools.AgentCreateHandler
}

func NewServiceContext(repo repository.MessageRepository, userExists logic.UserExistenceChecker, groups logic.GroupMemberLister) *ServiceContext {
	return NewServiceContextWithAuth(repo, userExists, groups, config.DefaultJWTAuthConfig())
}

func NewServiceContextWithAuth(repo repository.MessageRepository, userExists logic.UserExistenceChecker, groups logic.GroupMemberLister, auth config.JWTAuthConfig) *ServiceContext {
	mediaRepo := repository.NewMemoryMediaRepository()
	feedbackRepo := repository.NewMemoryFeedbackRepository()
	return NewServiceContextWithFeedback(repo, mediaRepo, feedbackRepo, userExists, groups, auth)
}

func NewServiceContextWithMedia(repo repository.MessageRepository, mediaRepo repository.MediaRepository, userExists logic.UserExistenceChecker, groups logic.GroupMemberLister, auth config.JWTAuthConfig) *ServiceContext {
	return NewServiceContextWithFeedback(repo, mediaRepo, repository.NewMemoryFeedbackRepository(), userExists, groups, auth)
}

func NewServiceContextWithFeedback(repo repository.MessageRepository, mediaRepo repository.MediaRepository, feedbackRepo repository.FeedbackRepository, userExists logic.UserExistenceChecker, groups logic.GroupMemberLister, auth config.JWTAuthConfig) *ServiceContext {
	mediaValidator := mediavalidate.NewMessageValidator(mediaRepo)
	agentHostingRepo := repository.NewMemoryAgentConversationHostingRepository()
	aiHostingRepo := repository.NewMemoryConversationAIHostingRepository()
	agentAuditRepo := repository.NewMemoryAgentAuditRepository()
	return &ServiceContext{
		AuthRuntime:      common.NewAuthRuntime(auth),
		MessageLogic:     logic.NewMessageLogicWithMediaValidator(repo, userExists, groups, mediaValidator),
		AIHostingLogic:   logic.NewConversationAIHostingLogic(aiHostingRepo),
		FeedbackLogic:    logic.NewFeedbackLogic(feedbackRepo),
		MessageRepo:      repo,
		MediaRepo:        mediaRepo,
		FeedbackRepo:     feedbackRepo,
		AgentHostingRepo: agentHostingRepo,
		AIHostingRepo:    aiHostingRepo,
		GroupMembers:     groups,
		AgentAuditLogic:  logic.NewAgentAuditLogic(agentAuditRepo),
		AgentAuditRepo:   agentAuditRepo,
	}
}

func ConfigureConversationAIHosting(ctx *ServiceContext, deepSeek config.DeepSeekConfig, obs config.LLMObservabilityConfig) error {
	opts := ConversationAIHostingRuntimeOptions{
		DeepSeek:         deepSeek,
		LLMObservability: obs,
	}
	if ctx != nil {
		opts.AgentRegistry = ctx.AgentRegistryRepo
		opts.PythonExecutor = ctx.PythonExecutor
		opts.AgentCreate = agentCreateHandlerFromContext(ctx, opts.AgentRegistry)
	}
	return ConfigureConversationAIHostingWithRuntimeOptions(ctx, opts)
}

func ConfigureConversationAIHostingWithRuntimeOptions(ctx *ServiceContext, opts ConversationAIHostingRuntimeOptions) error {
	if err := validateConversationAIHostingDependencies(ctx); err != nil {
		return err
	}
	if ctx.AgentHostingRepo == nil {
		return apperror.Internal("agent conversation hosting repository is not configured")
	}
	if ctx.AIHostingRepo == nil {
		return apperror.Internal("conversation AI hosting repository is not configured")
	}
	if ctx.AIHostingLogic == nil {
		ctx.AIHostingLogic = logic.NewConversationAIHostingLogic(ctx.AIHostingRepo)
	}
	if ctx.AgentResolver != nil {
		ctx.AIHostingLogic.WithAgentAccountResolver(ctx.AgentResolver)
	}
	if ctx.AgentAuditRepo == nil {
		return apperror.Internal("agent audit repository is not configured")
	}
	if ctx.AgentAuditLogic == nil {
		ctx.AgentAuditLogic = logic.NewAgentAuditLogic(ctx.AgentAuditRepo)
	}
	var responseSender agentim.MessageSender = ctx.MessageLogic
	if ctx.AgentResponseSender != nil {
		responseSender = ctx.AgentResponseSender
	}
	writer, err := agentim.NewMessageServiceResponseWriter(responseSender)
	if err != nil {
		return err
	}
	llmObsConfig := llmObservabilityConfig(opts.LLMObservability)
	llmObsSink, err := llmobs.NewSink(llmObsConfig)
	if err != nil {
		return err
	}
	// OB-12: keep remote (Langfuse) export off the agent run path — enqueue in the
	// foreground and export in a background worker, dropping on backpressure.
	if _, ok := llmObsSink.(*llmobs.LangfuseSink); ok {
		llmObsSink = llmobs.NewAsyncSink(llmObsSink, llmObsConfig.Backend, 0)
	}
	toolProvider, err := newConversationAIHostingToolProviderWithAgentCreate(opts.AgentRegistry, opts.PythonExecutor, opts.AgentCreate)
	if err != nil {
		return err
	}
	runtimeOptions := []einoruntime.DeepSeekRuntimeOption{
		einoruntime.WithLLMObservability(llmObsSink, llmObsConfig),
	}
	if toolProvider != nil {
		runtimeOptions = append(runtimeOptions, einoruntime.WithToolProvider(toolProvider))
	}
	orchestrator, err := agentim.NewAgentRunOrchestrator(agentim.AgentRunOrchestratorConfig{
		Runtime: einoruntime.NewDeepSeekRuntime(opts.DeepSeek, runtimeOptions...),
		RequestBuilder: agentim.NewConversationAIHostingRuntimeRequestBuilder(agentim.ConversationAIHostingRuntimeRequestBuilderConfig{
			MessageRepository: ctx.MessageRepo,
			HostingRepository: ctx.AIHostingRepo,
			AgentRepository:   agentRepositoryFromResolver(ctx.AgentResolver),
			AgentRegistry:     opts.AgentRegistry,
			DeepSeek:          opts.DeepSeek,
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
		ReadMarker:           agentim.NewMessageRepositoryReadMarker(ctx.MessageRepo),
	})
	if err != nil {
		return err
	}
	ctx.AgentMessageHook = hosting
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

func validateConversationAIHostingDependencies(ctx *ServiceContext) error {
	if ctx == nil {
		return apperror.Internal("message service context is not configured")
	}
	if ctx.MessageLogic == nil {
		return apperror.Internal("message logic is not configured")
	}
	if ctx.MessageRepo == nil {
		return apperror.Internal("message repository is not configured")
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

func newConversationAIHostingToolProvider(registryRepo repository.AgentRegistryRepository, executor pythonexec.Executor) (runtimetools.Provider, error) {
	return newConversationAIHostingToolProviderWithAgentCreate(registryRepo, executor, nil)
}

func newConversationAIHostingToolProviderWithAgentCreate(registryRepo repository.AgentRegistryRepository, executor pythonexec.Executor, agentCreate runtimetools.AgentCreateHandler) (runtimetools.Provider, error) {
	if registryRepo == nil {
		return nil, nil
	}
	pythonCatalog := runtimetools.NewDefaultLocalAdapterCatalog(executor)
	catalog := runtimetools.AdapterCatalogFunc(func(spec runtimetools.ToolSpec) (runtimetools.ToolAdapter, bool, error) {
		if runtimetools.IsAgentCreateToolSpec(spec) {
			if agentCreate == nil {
				return nil, false, nil
			}
			adapter, err := runtimetools.NewAgentCreateAdapter(spec, agentCreate)
			if err != nil {
				return nil, false, err
			}
			return adapter, true, nil
		}
		return pythonCatalog.LookupToolAdapter(spec)
	})
	return runtimetools.NewResolver(
		registryRepo,
		runtimetools.WithAdapterCatalog(catalog),
	)
}

func agentCreateHandlerFromContext(ctx *ServiceContext, registry repository.AgentRegistryRepository) runtimetools.AgentCreateHandler {
	if ctx == nil || ctx.AccountRepo == nil || ctx.AgentRepo == nil || registry == nil {
		return nil
	}
	assembly := logic.NewAgentAssemblyLogic(logic.AgentAssemblyDependencies{
		Accounts:    ctx.AccountRepo,
		Friendships: ctx.AccountRepo,
		Agents:      ctx.AgentRepo,
		Registry:    registry,
	})
	return runtimetools.AgentCreateHandlerFunc(func(ctx context.Context, req runtimetools.AgentCreateRequest) (runtimetools.AgentCreateResponse, error) {
		created, err := assembly.CreateAgentFromTool(ctx, logic.AgentCreateToolRequest{
			CreatorAgentID:   req.CreatorAgentID,
			RequestingUserID: req.RequestingUserID,
			Identifier:       req.Identifier,
			Name:             req.Name,
			Description:      req.Description,
			SystemPrompt:     req.SystemPrompt,
			ToolNames:        req.ToolNames,
		})
		if err != nil {
			return runtimetools.AgentCreateResponse{}, err
		}
		return runtimetools.AgentCreateResponse{
			AgentID:      created.AgentID,
			AccountID:    created.AccountID,
			Identifier:   created.Identifier,
			Name:         created.Name,
			Description:  created.Description,
			PromptID:     created.PromptID,
			ToolNames:    created.ToolNames,
			FriendUserID: created.FriendUserID,
		}, nil
	})
}
