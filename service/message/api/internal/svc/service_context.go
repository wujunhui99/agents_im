package svc

import (
	"context"
	"fmt"
	"log"

	"github.com/wujunhui99/agents_im/internal/agent/pythonexec"
	"github.com/wujunhui99/agents_im/internal/agentim"
	einoruntime "github.com/wujunhui99/agents_im/internal/agentruntime/eino"
	runtimetools "github.com/wujunhui99/agents_im/internal/agentruntime/tools"
	"github.com/wujunhui99/agents_im/internal/apperror"
	authrepo "github.com/wujunhui99/agents_im/internal/auth/repository"
	appconfig "github.com/wujunhui99/agents_im/internal/config"
	"github.com/wujunhui99/agents_im/internal/llmobs"
	"github.com/wujunhui99/agents_im/internal/logic"
	"github.com/wujunhui99/agents_im/internal/repository"
	adminsvc "github.com/wujunhui99/agents_im/internal/servicecontext/admin"
	"github.com/wujunhui99/agents_im/internal/servicecontext/common"
	"github.com/wujunhui99/agents_im/service/message/api/internal/config"
)

type ServiceContext struct {
	Config config.Config
	common.AuthRuntime
	MessageLogic      *logic.MessageLogic
	AgentMessageHook  logic.MessageCreatedHook
	AIHostingLogic    *logic.ConversationAIHostingLogic
	MediaLogic        *logic.MediaLogic
	FeedbackLogic     *logic.FeedbackLogic
	MessageRepo       repository.MessageRepository
	MediaRepo         repository.MediaRepository
	FeedbackRepo      repository.FeedbackRepository
	AgentHostingRepo  repository.AgentConversationHostingRepository
	AIHostingRepo     repository.ConversationAIHostingRepository
	GroupMembers      logic.GroupMemberLister
	OutboxRepo        repository.OutboxRepository
	AgentAuditLogic   *logic.AgentAuditLogic
	AgentAuditRepo    repository.AgentAuditRepository
	AgentResolver     logic.AgentAccountExistenceChecker
	AccountRepo       repository.Repository
	AgentRepo         repository.AgentRepository
	AgentRegistryRepo repository.AgentRegistryRepository
	TaskReportRepo    repository.TaskReportRepository
	PythonExecutor    pythonexec.Executor
}

type ConversationAIHostingRuntimeOptions struct {
	DeepSeek         appconfig.DeepSeekConfig
	LLMObservability appconfig.LLMObservabilityConfig
	AgentRegistry    repository.AgentRegistryRepository
	PythonExecutor   pythonexec.Executor
	AgentCreate      runtimetools.AgentCreateHandler
}

func NewServiceContext(c config.Config) (*ServiceContext, error) {
	groupsRepo, err := repository.NewGroupsRepositoryForStorage(c.StorageDriver, c.DataSource)
	if err != nil {
		return nil, fmt.Errorf("build groups repository: %w", err)
	}
	accountRepo, err := repository.NewRepositoryForStorage(c.StorageDriver, c.DataSource)
	if err != nil {
		return nil, fmt.Errorf("build account repository: %w", err)
	}
	messageRepo, err := repository.NewMessageRepositoryForStorage(c.StorageDriver, c.DataSource)
	if err != nil {
		return nil, fmt.Errorf("build message repository: %w", err)
	}
	mediaRepo, err := repository.NewMediaRepositoryForStorage(c.StorageDriver, c.DataSource)
	if err != nil {
		return nil, fmt.Errorf("build media repository: %w", err)
	}
	agentHostingRepo, err := repository.NewAgentConversationHostingRepositoryForStorage(c.StorageDriver, c.DataSource)
	if err != nil {
		return nil, fmt.Errorf("build agent hosting repository: %w", err)
	}
	agentRepo, err := repository.NewAgentRepositoryForStorage(c.StorageDriver, c.DataSource)
	if err != nil {
		return nil, fmt.Errorf("build agent repository: %w", err)
	}
	agentRegistryRepo, err := repository.NewAgentRegistryRepositoryForStorage(c.StorageDriver, c.DataSource)
	if err != nil {
		return nil, fmt.Errorf("build agent registry repository: %w", err)
	}
	aiHostingRepo, err := repository.NewConversationAIHostingRepositoryForStorage(c.StorageDriver, c.DataSource)
	if err != nil {
		return nil, fmt.Errorf("build AI hosting repository: %w", err)
	}
	agentAuditRepo, err := repository.NewAgentAuditRepositoryForStorage(c.StorageDriver, c.DataSource)
	if err != nil {
		return nil, fmt.Errorf("build agent audit repository: %w", err)
	}
	feedbackRepo, err := repository.NewFeedbackRepositoryForStorage(c.StorageDriver, c.DataSource)
	if err != nil {
		return nil, fmt.Errorf("build feedback repository: %w", err)
	}
	taskReportRepo, err := repository.NewTaskReportRepositoryForStorage(c.StorageDriver, c.DataSource)
	if err != nil {
		return nil, fmt.Errorf("build task report repository: %w", err)
	}
	var pythonExecutorClient pythonexec.KubernetesSandboxClient
	if c.PythonExecutor.Backend == appconfig.PythonExecutorBackendK8S {
		pythonExecutorClient, err = pythonexec.NewInClusterKubernetesSandboxClient()
		if err != nil {
			return nil, fmt.Errorf("build python executor kubernetes client: %w", err)
		}
	}
	pythonExecutor, err := pythonexec.NewExecutorFromConfig(c.PythonExecutor, pythonExecutorClient)
	if err != nil {
		return nil, fmt.Errorf("build python executor: %w", err)
	}

	groupsLogic := logic.NewGroupsLogic(groupsRepo, nil)
	ctx := NewServiceContextWithFeedback(messageRepo, mediaRepo, feedbackRepo, nil, groupsLogic, c.Auth)
	ctx.Config = c
	ctx.AgentHostingRepo = agentHostingRepo
	ctx.AIHostingRepo = aiHostingRepo
	ctx.AgentResolver = agentim.NewAgentRepositoryAccountResolver(agentRepo)
	ctx.AccountRepo = accountRepo
	ctx.AgentRepo = agentRepo
	ctx.AIHostingLogic = logic.NewConversationAIHostingLogic(aiHostingRepo).WithAgentAccountResolver(ctx.AgentResolver)
	ctx.AgentAuditRepo = agentAuditRepo
	ctx.AgentAuditLogic = logic.NewAgentAuditLogic(agentAuditRepo)
	ctx.AgentRegistryRepo = agentRegistryRepo
	ctx.TaskReportRepo = taskReportRepo
	ctx.PythonExecutor = pythonExecutor
	if err := ConfigureConversationAIHosting(ctx, c.DeepSeek, c.LLMObservability); err != nil {
		return nil, fmt.Errorf("configure AI conversation hosting: %w", err)
	}
	if appconfig.ResolveStorageDriver(c.StorageDriver) == appconfig.StorageDriverPostgres {
		authRepo, err := authrepo.NewRepositoryForStorage(c.StorageDriver, c.DataSource)
		if err != nil {
			return nil, fmt.Errorf("build auth repository: %w", err)
		}
		ctx.AuthSessions = authRepo
	} else {
		log.Printf("active session shared validation disabled for storage driver %q; use postgres for single-device enforcement across services", appconfig.ResolveStorageDriver(c.StorageDriver))
	}
	return ctx, nil
}

func NewAdminServiceContext(ctx *ServiceContext) (*adminsvc.ServiceContext, error) {
	if ctx == nil {
		return nil, apperror.Internal("message api service context is not configured")
	}
	accounts, ok := ctx.AccountRepo.(repository.AdminAccountRepository)
	if !ok {
		return nil, fmt.Errorf("account repository has unexpected type %T", ctx.AccountRepo)
	}
	friends, ok := ctx.AccountRepo.(repository.FriendshipRepository)
	if !ok {
		return nil, fmt.Errorf("friendship repository has unexpected type %T", ctx.AccountRepo)
	}
	messages, ok := ctx.MessageRepo.(repository.AdminMessageRepository)
	if !ok {
		return nil, fmt.Errorf("message repository has unexpected type %T", ctx.MessageRepo)
	}
	agentAudits, ok := ctx.AgentAuditRepo.(repository.AdminAgentAuditRepository)
	if !ok {
		return nil, fmt.Errorf("agent audit repository has unexpected type %T", ctx.AgentAuditRepo)
	}
	adminCtx := adminsvc.NewServiceContextWithAuth(adminsvc.Dependencies{
		Accounts:           accounts,
		Friends:            friends,
		Messages:           messages,
		AgentAudits:        agentAudits,
		Feedback:           ctx.FeedbackRepo,
		TaskReports:        ctx.TaskReportRepo,
		MessageCreatedHook: ctx.AgentMessageHook,
	}, ctx.Config.Auth)
	adminCtx.AuthSessions = ctx.AuthSessions
	return adminCtx, nil
}

func NewServiceContextWithFeedback(repo repository.MessageRepository, mediaRepo repository.MediaRepository, feedbackRepo repository.FeedbackRepository, userExists logic.UserExistenceChecker, groups logic.GroupMemberLister, auth appconfig.JWTAuthConfig) *ServiceContext {
	mediaLogic := logic.NewMediaLogic(mediaRepo, nil, appconfig.DefaultObjectStorageConfig().Bucket)
	mediaLogic.WithAttachmentAccessChecker(logic.NewMessageMediaAccessChecker(repo))
	agentHostingRepo := repository.NewMemoryAgentConversationHostingRepository()
	aiHostingRepo := repository.NewMemoryConversationAIHostingRepository()
	agentAuditRepo := repository.NewMemoryAgentAuditRepository()
	return &ServiceContext{
		AuthRuntime:      common.NewAuthRuntime(auth),
		MessageLogic:     logic.NewMessageLogicWithMediaValidator(repo, userExists, groups, mediaLogic),
		AIHostingLogic:   logic.NewConversationAIHostingLogic(aiHostingRepo),
		MediaLogic:       mediaLogic,
		FeedbackLogic:    logic.NewFeedbackLogic(feedbackRepo),
		MessageRepo:      repo,
		MediaRepo:        mediaRepo,
		FeedbackRepo:     feedbackRepo,
		AgentHostingRepo: agentHostingRepo,
		AIHostingRepo:    aiHostingRepo,
		GroupMembers:     groups,
		OutboxRepo:       outboxRepositoryFromMessageRepo(repo),
		AgentAuditLogic:  logic.NewAgentAuditLogic(agentAuditRepo),
		AgentAuditRepo:   agentAuditRepo,
	}
}

func ConfigureConversationAIHosting(ctx *ServiceContext, deepSeek appconfig.DeepSeekConfig, obs appconfig.LLMObservabilityConfig) error {
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
	writer, err := agentim.NewMessageServiceResponseWriter(ctx.MessageLogic)
	if err != nil {
		return err
	}
	llmObsConfig := llmObservabilityConfig(opts.LLMObservability)
	llmObsSink, err := llmobs.NewSink(llmObsConfig)
	if err != nil {
		return err
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

func llmObservabilityConfig(obs appconfig.LLMObservabilityConfig) llmobs.Config {
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
