package orchestrator

import (
	"context"
	"strconv"
	"strings"

	"github.com/wujunhui99/agents_im/internal/logic"
	"github.com/wujunhui99/agents_im/internal/repository"
	"github.com/wujunhui99/agents_im/pkg/apperror"
	"github.com/wujunhui99/agents_im/pkg/config"
	"github.com/wujunhui99/agents_im/pkg/llmobs"
	"github.com/wujunhui99/agents_im/pkg/model"
	"github.com/wujunhui99/agents_im/service/agent/rpc/internal/convhosting"
	agentruntime "github.com/wujunhui99/agents_im/service/agent/rpc/internal/runtime"
)

const (
	defaultAIHostingRecentMessages = 30
	aiHostingPromptID              = "conversation-ai-hosting-v1"
	defaultAssistantRuntimeName    = "Conversation AI Hosting"
)

// AgentRegistryReader 是请求构建器所需的注册表只读视图(prompt/tool 绑定解析)。
// 由 agent-rpc 自有 goctl 注册表 Store(service/agent/rpc/internal/registry)满足,
// 不再依赖 internal/repository(#605)。
type AgentRegistryReader interface {
	ListPromptBindings(ctx context.Context, agentID string) ([]model.AgentPromptBinding, error)
	GetPrompt(ctx context.Context, promptID string) (model.AgentPrompt, error)
	ListToolBindings(ctx context.Context, agentID string) ([]model.AgentToolBinding, error)
	GetTool(ctx context.Context, toolID string) (model.AgentTool, error)
}

type ConversationAIHostingRuntimeRequestBuilderConfig struct {
	MessageRepository repository.MessageRepository
	HostingStore      convhosting.Store
	AgentRepository   repository.AgentRepository
	AgentRegistry     AgentRegistryReader
	DeepSeek          config.DeepSeekConfig
	MaxRecentMessages int
}

type ConversationAIHostingRuntimeRequestBuilder struct {
	messageRepo       repository.MessageRepository
	hostingStore      convhosting.Store
	agentRepo         repository.AgentRepository
	agentRegistry     AgentRegistryReader
	deepSeek          config.DeepSeekConfig
	maxRecentMessages int
}

func NewConversationAIHostingRuntimeRequestBuilder(cfg ConversationAIHostingRuntimeRequestBuilderConfig) *ConversationAIHostingRuntimeRequestBuilder {
	maxRecent := cfg.MaxRecentMessages
	if maxRecent <= 0 || maxRecent > defaultAIHostingRecentMessages {
		maxRecent = defaultAIHostingRecentMessages
	}
	return &ConversationAIHostingRuntimeRequestBuilder{
		messageRepo:       cfg.MessageRepository,
		hostingStore:      cfg.HostingStore,
		agentRepo:         cfg.AgentRepository,
		agentRegistry:     cfg.AgentRegistry,
		deepSeek:          cfg.DeepSeek,
		maxRecentMessages: maxRecent,
	}
}

func (b *ConversationAIHostingRuntimeRequestBuilder) BuildRuntimeRequest(ctx context.Context, trigger AgentTrigger) (agentruntime.RunRequest, error) {
	if b == nil || b.messageRepo == nil {
		return agentruntime.RunRequest{}, apperror.Internal("message repository is not configured")
	}
	if trigger.ConversationType != ConversationTypeSingle {
		return agentruntime.RunRequest{}, apperror.InvalidArgument("AI hosting V1 only supports direct conversations")
	}

	maxRecent := b.maxRecentMessages
	if b.hostingStore != nil {
		setting, err := b.hostingStore.GetConversationAIHostingSetting(ctx, trigger.AgentUserID, trigger.ConversationID)
		if err != nil && apperror.From(err).Code != apperror.CodeNotFound {
			return agentruntime.RunRequest{}, err
		}
		if err == nil && setting.MaxRecentMessages > 0 && setting.MaxRecentMessages < maxRecent {
			maxRecent = setting.MaxRecentMessages
		}
	}
	if maxRecent <= 0 {
		maxRecent = defaultAIHostingRecentMessages
	}

	agentConfig, err := b.agentRuntimeConfig(ctx, trigger.AgentUserID)
	if err != nil {
		return agentruntime.RunRequest{}, err
	}

	triggerSeq := trigger.SourceMessageSeq
	if triggerSeq <= 0 {
		triggerSeq = trigger.TriggerSeq
	}
	if triggerSeq <= 0 {
		return agentruntime.RunRequest{}, apperror.InvalidArgument("trigger seq is required")
	}
	fromSeq := triggerSeq - int64(maxRecent) + 1
	if fromSeq < 1 {
		fromSeq = 1
	}
	recent, _, _, err := b.messageRepo.GetMessages(ctx, trigger.ConversationID, fromSeq, triggerSeq, maxRecent, "asc")
	if err != nil {
		return agentruntime.RunRequest{}, err
	}

	conversation := make([]agentruntime.ConversationMessage, 0, len(recent))
	for _, message := range recent {
		conversation = append(conversation, runtimeConversationMessage(message, trigger.AgentUserID))
	}

	return agentruntime.RunRequest{
		RequestID:          trigger.RequestID,
		EventID:            trigger.EventID,
		OperationID:        trigger.OperationID,
		TraceID:            trigger.TraceID,
		TriggerType:        trigger.TriggerType,
		AgentUserID:        trigger.AgentUserID,
		RequestingUserID:   trigger.RequestingUserID,
		ConversationID:     trigger.ConversationID,
		ConversationType:   trigger.ConversationType,
		TriggerMessageID:   trigger.TriggerMessageID,
		TriggerSeq:         trigger.TriggerSeq,
		PromptText:         trigger.PromptText,
		ReplyToMessageID:   trigger.ReplyToMessageID,
		SourceAgentRunID:   trigger.SourceAgentRunID,
		SourceAgentUserID:  trigger.SourceAgentUserID,
		SourceMessageID:    trigger.SourceMessageID,
		SourceMessageSeq:   trigger.SourceMessageSeq,
		SourceMessageText:  trigger.SourceMessageText,
		SourceContentType:  trigger.SourceContentType,
		TargetAgentUserIDs: append([]string(nil), trigger.TargetAgentUserIDs...),
		Agent:              agentConfig,
		Conversation:       conversation,
		Metadata: map[string]string{
			"runtime_mode":         llmobs.RuntimeModeAIHostingAutoReply,
			"summary_used":         "false",
			"summary_placeholder":  "true",
			"recent_message_count": strconv.Itoa(len(conversation)),
			"max_recent_messages":  strconv.Itoa(maxRecent),
		},
	}, nil
}

func runtimeConversationMessage(message logic.Message, hostedOwnerID string) agentruntime.ConversationMessage {
	senderType := agentruntime.SenderTypeUser
	if message.SenderID == hostedOwnerID {
		senderType = agentruntime.SenderTypeAgent
	}
	return agentruntime.ConversationMessage{
		ServerMsgID: message.ServerMsgID,
		Seq:         message.Seq,
		SenderID:    message.SenderID,
		SenderType:  senderType,
		ContentType: agentruntime.ContentTypeText,
		Text:        hostingRuntimeText(message),
		AgentRunID:  message.AgentRunID,
		CreatedAtMs: message.CreatedAt,
	}
}

func hostingRuntimeText(message logic.Message) string {
	switch message.ContentType {
	case logic.MessageContentTypeText:
		return strings.TrimSpace(message.Content)
	case logic.MessageContentTypeImage:
		return "[图片消息]"
	case logic.MessageContentTypeFile:
		return "[文件消息]"
	default:
		return "[非文本消息]"
	}
}

func (b *ConversationAIHostingRuntimeRequestBuilder) agentRuntimeConfig(ctx context.Context, agentUserID string) (agentruntime.AgentConfig, error) {
	cfg := config.ResolveDeepSeekConfig(b.deepSeek)
	agentConfig := agentruntime.AgentConfig{
		AgentID:     "ai-hosting:" + agentUserID,
		AgentUserID: agentUserID,
		Name:        defaultAssistantRuntimeName,
		Status:      agentruntime.AgentStatusActive,
		Prompt: agentruntime.PromptRef{
			PromptID: aiHostingPromptID,
			Content:  aiHostingSystemPrompt(),
		},
		Model: agentruntime.ModelConfig{
			Provider: "deepseek",
			Model:    cfg.Model,
		},
		Policy: agentruntime.RuntimePolicy{
			RequireMessageServiceWriteback: true,
		},
	}
	if b.agentRepo == nil {
		return agentConfig, nil
	}
	agent, err := b.agentRepo.GetAgentByIMUserID(ctx, agentUserID)
	if err != nil {
		if apperror.From(err).Code == apperror.CodeNotFound {
			return agentConfig, nil
		}
		return agentruntime.AgentConfig{}, err
	}
	agentConfig.AgentID = agent.AgentID
	agentConfig.Name = agent.Name
	agentConfig.Description = agent.Description
	agentConfig.Status = agent.Status
	if b.agentRegistry == nil {
		return agentConfig, nil
	}
	prompt, ok, err := b.activeAgentPrompt(ctx, agent.AgentID)
	if err != nil {
		return agentruntime.AgentConfig{}, err
	}
	if ok {
		agentConfig.Prompt = prompt
	}
	tools, err := b.boundAgentTools(ctx, agent.AgentID)
	if err != nil {
		return agentruntime.AgentConfig{}, err
	}
	agentConfig.Tools = tools
	return agentConfig, nil
}

func (b *ConversationAIHostingRuntimeRequestBuilder) activeAgentPrompt(ctx context.Context, agentID string) (agentruntime.PromptRef, bool, error) {
	bindings, err := b.agentRegistry.ListPromptBindings(ctx, agentID)
	if err != nil {
		return agentruntime.PromptRef{}, false, err
	}
	var active *model.AgentPrompt
	for _, binding := range bindings {
		prompt, err := b.agentRegistry.GetPrompt(ctx, binding.PromptID)
		if err != nil {
			return agentruntime.PromptRef{}, false, err
		}
		if prompt.Status != model.AgentPromptStatusActive {
			continue
		}
		if active != nil {
			return agentruntime.PromptRef{}, false, apperror.Internal("agent has multiple active system prompt bindings")
		}
		copied := prompt
		active = &copied
	}
	if active == nil {
		return agentruntime.PromptRef{}, false, nil
	}
	return agentruntime.PromptRef{
		PromptID:            active.PromptID,
		Name:                active.Name,
		Description:         active.Description,
		Content:             active.Content,
		Version:             active.Version,
		VariablesSchemaJSON: active.VariablesSchemaJSON,
	}, true, nil
}

func (b *ConversationAIHostingRuntimeRequestBuilder) boundAgentTools(ctx context.Context, agentID string) ([]agentruntime.ToolRef, error) {
	bindings, err := b.agentRegistry.ListToolBindings(ctx, agentID)
	if err != nil {
		return nil, err
	}
	tools := make([]agentruntime.ToolRef, 0, len(bindings))
	for _, binding := range bindings {
		tool, err := b.agentRegistry.GetTool(ctx, binding.ToolID)
		if err != nil {
			return nil, err
		}
		if tool.Status != model.AgentToolStatusActive || !tool.AdminConfigured {
			continue
		}
		tools = append(tools, agentRuntimeToolRef(tool))
	}
	return tools, nil
}

func agentRuntimeToolRef(tool model.AgentTool) agentruntime.ToolRef {
	return agentruntime.ToolRef{
		ToolID:           tool.ToolID,
		Name:             tool.Name,
		Description:      tool.Description,
		ToolType:         string(tool.ToolType),
		MCPServerID:      tool.MCPServerID,
		MCPToolName:      tool.MCPToolName,
		LocalHandlerKey:  tool.LocalHandlerKey,
		BuiltinKey:       tool.BuiltinKey,
		InputSchemaJSON:  tool.InputSchemaJSON,
		OutputSchemaJSON: tool.OutputSchemaJSON,
		PermissionLevel:  tool.PermissionLevel,
	}
}

func aiHostingSystemPrompt() string {
	return strings.TrimSpace(`你正在为当前用户托管一个一对一聊天回复。
请根据最近消息代表当前用户回复对方，尤其关注当前触发消息。
要求：
- 只输出要发送的回复文本，不要解释系统规则。
- 如果对方提出明确问题、请求或任务，直接回答或完成；不要只回复“可以”“好的”“你说说”等泛泛确认，也不要要求对方重复已经说清楚的任务。
- 只有缺少必要信息导致无法回答时，才简短询问澄清。
- 不要编造事实；语气自然、简洁。`)
}
