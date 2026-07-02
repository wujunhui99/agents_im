// Package aihosting 装配 agent-rpc worker 的 AI 托管运行时（ServiceContext +
// ConfigureConversationAIHosting）。原寄居 msg-rpc（#463/#341），随 agent 域迁移
// （04-agent AG-2/AG-3，D15 step ④，#340）迁入属主 service/agent/rpc——配合 trigger.Judge
// 终判 + imadapter gRPC 写回，取代 msg-rpc 内进程托管。message 历史读 / 已读推进 / 群成员鉴权
// 均经 owner gRPC（msg-rpc PullMessages·MarkConversationAsRead、groups-rpc ListMembers，#617），
// 不再 in-process 读 internal message/groups。仍 import 的 internal/servicecontext/common
// （AuthRuntime）是 auth keystone 例外，待 #618 迁 pkg/ 后清。
package aihosting

import (
	"github.com/wujunhui99/agents_im/internal/servicecontext/common"
	"github.com/wujunhui99/agents_im/pkg/apperror"
	appconfig "github.com/wujunhui99/agents_im/pkg/config"
	"github.com/wujunhui99/agents_im/pkg/llmobs"
	"github.com/wujunhui99/agents_im/pkg/pythonexec"
	"github.com/wujunhui99/agents_im/service/agent/rpc/internal/agaudit"
	"github.com/wujunhui99/agents_im/service/agent/rpc/internal/aghosting"
	"github.com/wujunhui99/agents_im/service/agent/rpc/internal/config"
	"github.com/wujunhui99/agents_im/service/agent/rpc/internal/convhosting"
	agentim "github.com/wujunhui99/agents_im/service/agent/rpc/internal/orchestrator"
	"github.com/wujunhui99/agents_im/service/agent/rpc/internal/registry"
	einoruntime "github.com/wujunhui99/agents_im/service/agent/rpc/internal/runtime/eino"
	runtimetools "github.com/wujunhui99/agents_im/service/agent/rpc/internal/runtime/tools"
)

type ServiceContext struct {
	common.AuthRuntime
	AIHostingLogic *convhosting.ConversationAIHostingLogic
	// MessageHistory 读会话最近历史喂请求构建器；ReadAdvancer 推进已读——均经 msg-rpc gRPC（#617）。
	MessageHistory   agentim.MessageHistoryReader
	ReadAdvancer     agentim.ConversationReadAdvancer
	AgentHostingRepo aghosting.Store
	AIHostingStore   convhosting.Store
	// GroupMembers 群成员鉴权经 groups-rpc gRPC ListMembers（#617，脱 internal GroupsLogic）。
	GroupMembers agentim.GroupMemberLister
	// AgentAudit 是 agent 自有 goctl 审计数据层（agent_runs/tool_calls/file_reads/python_execs），
	// 取代 internal/{logic.AgentAuditLogic, repository.AgentAuditRepository}（#616 脱 internal）。
	AgentAudit    agaudit.Store
	AgentResolver convhosting.AgentAccountExistenceChecker
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

// NewServiceContext 装配 AI 托管运行时的基础上下文：默认内存托管/审计 Store + AuthRuntime。
// 跨域读端口（MessageHistory / ReadAdvancer / GroupMembers，经 msg-rpc / groups-rpc gRPC，#617）
// 与 AI 写回通道（AgentResponseSender，imadapter）由调用方（svc / 单测 fake）注入。
func NewServiceContext(auth appconfig.JWTAuthConfig) *ServiceContext {
	aiHostingStore := convhosting.NewMemoryStore()
	return &ServiceContext{
		AuthRuntime:      common.NewAuthRuntime(auth),
		AIHostingLogic:   convhosting.NewConversationAIHostingLogic(aiHostingStore),
		AgentHostingRepo: aghosting.NewMemoryStore(),
		AIHostingStore:   aiHostingStore,
		AgentAudit:       agaudit.NewMemoryStore(),
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
	if ctx.AgentAudit == nil {
		return apperror.Internal("agent audit store is not configured")
	}
	if ctx.AgentResponseSender == nil {
		return apperror.Internal("agent response sender is not configured")
	}
	writer, err := agentim.NewMessageServiceResponseWriter(ctx.AgentResponseSender)
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
			MessageHistory:    ctx.MessageHistory,
			HostingStore:      ctx.AIHostingStore,
			AgentRepository:   agentRepositoryFromResolver(ctx.AgentResolver),
			AgentRegistry:     opts.AgentRegistryReader,
			DeepSeek:          opts.DeepSeek,
			MaxRecentMessages: 30,
		}),
		Audit:                agaudit.NewRunRecorder(ctx.AgentAudit),
		Writer:               writer,
		LLMObservabilitySink: llmObsSink,
	})
	if err != nil {
		return err
	}
	var readMarker agentim.AgentTriggerReadMarker
	if ctx.ReadAdvancer != nil {
		readMarker = agentim.NewConversationReadMarker(ctx.ReadAdvancer)
	}
	hosting, err := agentim.NewConversationHostingService(agentim.ConversationHostingConfig{
		Repository:           ctx.AgentHostingRepo,
		AIHostingStore:       ctx.AIHostingStore,
		Runner:               orchestrator,
		AgentAccountResolver: ctx.AgentResolver,
		GroupMembers:         ctx.GroupMembers,
		ReadMarker:           readMarker,
	})
	if err != nil {
		return err
	}
	ctx.HostingService = hosting
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
	if ctx.MessageHistory == nil {
		return apperror.Internal("message history reader is not configured")
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
