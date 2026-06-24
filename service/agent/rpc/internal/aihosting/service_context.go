// Package aihosting 装配 agent-rpc worker 的 AI 托管运行时（ServiceContext +
// ConfigureConversationAIHosting）。原寄居 msg-rpc（#463/#341），随 agent 域迁移
// （04-agent AG-2/AG-3，D15 step ④，#340）迁入属主 service/agent/rpc——配合 trigger.Judge
// 终判 + imadapter gRPC 写回，取代 msg-rpc 内进程托管。仍 import 的 internal/{logic,
// repository,servicecontext/common}（MessageLogic / AI runtime 数据层）是 keystone 例外，
// 待 internal/ 完全退役后清理。
package aihosting

import (
	"context"

	"github.com/wujunhui99/agents_im/internal/logic"
	"github.com/wujunhui99/agents_im/internal/repository"
	"github.com/wujunhui99/agents_im/internal/servicecontext/common"
	"github.com/wujunhui99/agents_im/pkg/apperror"
	"github.com/wujunhui99/agents_im/pkg/config"
	"github.com/wujunhui99/agents_im/pkg/llmobs"
	"github.com/wujunhui99/agents_im/pkg/pythonexec"
	"github.com/wujunhui99/agents_im/service/agent/rpc/internal/convhosting"
	agentim "github.com/wujunhui99/agents_im/service/agent/rpc/internal/orchestrator"
	"github.com/wujunhui99/agents_im/service/agent/rpc/internal/registry"
	einoruntime "github.com/wujunhui99/agents_im/service/agent/rpc/internal/runtime/eino"
	runtimetools "github.com/wujunhui99/agents_im/service/agent/rpc/internal/runtime/tools"
)

type ServiceContext struct {
	common.AuthRuntime
	MessageLogic     *logic.MessageLogic
	AgentMessageHook logic.MessageCreatedHook
	AIHostingLogic   *convhosting.ConversationAIHostingLogic
	FeedbackLogic    *logic.FeedbackLogic
	MessageRepo      repository.MessageRepository
	FeedbackRepo     repository.FeedbackRepository
	AgentHostingRepo repository.AgentConversationHostingRepository
	AIHostingStore   convhosting.Store
	GroupMembers     logic.GroupMemberLister
	AgentAuditLogic  *logic.AgentAuditLogic
	AgentAuditRepo   repository.AgentAuditRepository
	AgentResolver    convhosting.AgentAccountExistenceChecker
	// AgentRegistryReader 是 agent 自有 goctl 注册表只读 Store,喂 runtime tool 解析 + 请求
	// 构建器(#605:读路径已脱 internal/repository)。
	AgentRegistryReader registry.Reader
	// AgentCreate 是 agent.create 工具处理器,由 svc 用 agent 自有 agentlogic.AgentAssemblyLogic
	// (goctl + user-rpc/friends-rpc 端口)装配注入(#606:写路径脱 internal/repository、saga 拆解)。
	// 内存/单测路径留 nil → agent.create 工具不可用。
	AgentCreate    runtimetools.AgentCreateHandler
	PythonExecutor pythonexec.Executor
	// AgentResponseSender 覆盖 AI 写回通道（默认 MessageLogic 直写 PG）。
	// agent-rpc worker 注入 imadapter.MsgRPCSender（经 msg-rpc gRPC SendMessage 走 Kafka）。
	AgentResponseSender agentim.MessageSender
	// HostingService 是 ConfigureConversationAIHosting 装配出的具体托管服务（CHS）。
	// agent-rpc 的 trigger 消费者用它 ScheduleTrigger（幂等 + 已读推进 + 异步 run + 写回）。
	HostingService *agentim.ConversationHostingService
}

type ConversationAIHostingRuntimeOptions struct {
	DeepSeek         config.DeepSeekConfig
	LLMObservability config.LLMObservabilityConfig
	// AgentRegistryReader 供 runtime tool 解析 + 请求构建器使用(agent 自有 goctl Store,读路径)。
	AgentRegistryReader registry.Reader
	PythonExecutor      pythonexec.Executor
	// AgentCreate 是 agent.create 工具处理器(agent 自有 agentlogic 装配,由 svc 注入)。
	AgentCreate runtimetools.AgentCreateHandler
}

// allowAllMessageMediaValidator 是内存/单测 fixture：不接 media-rpc 时放行附件校验。
// 真实链路（msg-rpc）注入 media-rpc 校验器，绝不用本类型（#533）。
type allowAllMessageMediaValidator struct{}

func (allowAllMessageMediaValidator) ValidateMessageMedia(context.Context, string, string, string) error {
	return nil
}

func NewServiceContext(repo repository.MessageRepository, userExists logic.UserExistenceChecker, groups logic.GroupMemberLister) *ServiceContext {
	return NewServiceContextWithAuth(repo, userExists, groups, config.DefaultJWTAuthConfig())
}

func NewServiceContextWithAuth(repo repository.MessageRepository, userExists logic.UserExistenceChecker, groups logic.GroupMemberLister, auth config.JWTAuthConfig) *ServiceContext {
	// 内存/默认路径（单测、demo）不接 media-rpc，用放行校验器作 fixture；真实附件校验
	// 由 msg-rpc 注入的 media-rpc 校验器承担（#533）。
	return NewServiceContextWithMediaValidator(repo, allowAllMessageMediaValidator{}, userExists, groups, auth)
}

// NewServiceContextWithMediaValidator 用调用方注入的 media 校验器装配（#533：附件校验经 media-rpc，
// 不再由本包直读 media_objects）。validator 为 nil 时回退放行校验器（仅内存/单测语义）。
func NewServiceContextWithMediaValidator(repo repository.MessageRepository, mediaValidator logic.MessageMediaValidator, userExists logic.UserExistenceChecker, groups logic.GroupMemberLister, auth config.JWTAuthConfig) *ServiceContext {
	if mediaValidator == nil {
		mediaValidator = allowAllMessageMediaValidator{}
	}
	feedbackRepo := repository.NewMemoryFeedbackRepository()
	agentHostingRepo := repository.NewMemoryAgentConversationHostingRepository()
	aiHostingStore := convhosting.NewMemoryStore()
	agentAuditRepo := repository.NewMemoryAgentAuditRepository()
	return &ServiceContext{
		AuthRuntime:      common.NewAuthRuntime(auth),
		MessageLogic:     logic.NewMessageLogicWithMediaValidator(repo, userExists, groups, mediaValidator),
		AIHostingLogic:   convhosting.NewConversationAIHostingLogic(aiHostingStore),
		FeedbackLogic:    logic.NewFeedbackLogic(feedbackRepo),
		MessageRepo:      repo,
		FeedbackRepo:     feedbackRepo,
		AgentHostingRepo: agentHostingRepo,
		AIHostingStore:   aiHostingStore,
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
		opts.AgentRegistryReader = ctx.AgentRegistryReader
		opts.PythonExecutor = ctx.PythonExecutor
		opts.AgentCreate = ctx.AgentCreate
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
	if ctx.AIHostingStore == nil {
		return apperror.Internal("conversation AI hosting store is not configured")
	}
	if ctx.AIHostingLogic == nil {
		ctx.AIHostingLogic = convhosting.NewConversationAIHostingLogic(ctx.AIHostingStore)
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
	toolProvider, err := newConversationAIHostingToolProviderWithAgentCreate(opts.AgentRegistryReader, opts.PythonExecutor, opts.AgentCreate)
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
			HostingStore:      ctx.AIHostingStore,
			AgentRepository:   agentRepositoryFromResolver(ctx.AgentResolver),
			AgentRegistry:     opts.AgentRegistryReader,
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
		AIHostingStore:       ctx.AIHostingStore,
		Runner:               orchestrator,
		AgentAccountResolver: ctx.AgentResolver,
		GroupMembers:         ctx.GroupMembers,
		ReadMarker:           agentim.NewMessageRepositoryReadMarker(ctx.MessageRepo),
	})
	if err != nil {
		return err
	}
	ctx.AgentMessageHook = hosting
	ctx.HostingService = hosting
	ctx.MessageLogic.SetMessageCreatedHook(hosting)
	return nil
}

func agentRepositoryFromResolver(resolver convhosting.AgentAccountExistenceChecker) agentim.AgentReader {
	if typed, ok := resolver.(interface {
		AgentRepository() agentim.AgentReader
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

func newConversationAIHostingToolProvider(registryReader runtimetools.Registry, executor pythonexec.Executor) (runtimetools.Provider, error) {
	return newConversationAIHostingToolProviderWithAgentCreate(registryReader, executor, nil)
}

func newConversationAIHostingToolProviderWithAgentCreate(registryReader runtimetools.Registry, executor pythonexec.Executor, agentCreate runtimetools.AgentCreateHandler) (runtimetools.Provider, error) {
	if registryReader == nil {
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
		registryReader,
		runtimetools.WithAdapterCatalog(catalog),
	)
}

