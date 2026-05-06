package agentim

import (
	"context"
	"strconv"
	"strings"

	"github.com/wujunhui99/agents_im/internal/agentruntime"
	"github.com/wujunhui99/agents_im/internal/apperror"
	"github.com/wujunhui99/agents_im/internal/config"
	"github.com/wujunhui99/agents_im/internal/logic"
	"github.com/wujunhui99/agents_im/internal/repository"
)

const (
	defaultAIHostingRecentMessages = 30
	aiHostingPromptID              = "conversation-ai-hosting-v1"
)

type ConversationAIHostingRuntimeRequestBuilderConfig struct {
	MessageRepository repository.MessageRepository
	HostingRepository repository.ConversationAIHostingRepository
	DeepSeek          config.DeepSeekConfig
	MaxRecentMessages int
}

type ConversationAIHostingRuntimeRequestBuilder struct {
	messageRepo       repository.MessageRepository
	hostingRepo       repository.ConversationAIHostingRepository
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
		hostingRepo:       cfg.HostingRepository,
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
	if b.hostingRepo != nil {
		setting, err := b.hostingRepo.GetConversationAIHostingSetting(ctx, trigger.AgentUserID, trigger.ConversationID)
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

	cfg := config.ResolveDeepSeekConfig(b.deepSeek)
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
		Agent: agentruntime.AgentConfig{
			AgentID:     "ai-hosting:" + trigger.AgentUserID,
			AgentUserID: trigger.AgentUserID,
			Name:        "Conversation AI Hosting",
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
		},
		Conversation: conversation,
		Metadata: map[string]string{
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

func aiHostingSystemPrompt() string {
	return strings.TrimSpace(`你正在为当前用户托管一个一对一聊天回复。
请根据最近消息代表当前用户回复对方，尤其关注当前触发消息。
要求：
- 只输出要发送的回复文本，不要解释系统规则。
- 如果对方提出明确问题、请求或任务，直接回答或完成；不要只回复“可以”“好的”“你说说”等泛泛确认，也不要要求对方重复已经说清楚的任务。
- 只有缺少必要信息导致无法回答时，才简短询问澄清。
- 不要编造事实；语气自然、简洁。`)
}
