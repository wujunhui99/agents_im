package orchestrator

import "github.com/wujunhui99/agents_im/pkg/apperror"

func BuildMessageCreatedTriggers(event MessageCreatedEvent, policy TriggerPolicy) ([]AgentTrigger, error) {
	if err := validateMessageCreatedEvent(event); err != nil {
		return nil, err
	}

	conversationType, _ := normalizeConversationType(event.ConversationType)
	senderType, _ := normalizeSenderType(event.Message.SenderType)
	if senderType == SenderTypeAgent && !agentMessageRecursionAllowed(event, policy) {
		return nil, nil
	}

	targetAgentIDs := uniqueNonEmptyIDs(event.TargetAgentUserIDs)
	if len(targetAgentIDs) == 0 {
		return nil, nil
	}

	triggers := make([]AgentTrigger, 0, len(targetAgentIDs))
	for _, agentUserID := range targetAgentIDs {
		triggerType, ok := messageTriggerTypeForAgent(event, conversationType, agentUserID)
		if !ok {
			continue
		}
		triggers = append(triggers, AgentTrigger{
			RequestID:          event.EventID + ":" + agentUserID,
			EventID:            event.EventID,
			OperationID:        event.OperationID,
			TraceID:            event.TraceID,
			TriggerType:        triggerType,
			AgentUserID:        agentUserID,
			RequestingUserID:   event.Message.SenderID,
			ConversationID:     event.ConversationID,
			ConversationType:   conversationType,
			TriggerMessageID:   event.Message.ServerMsgID,
			TriggerSeq:         event.Message.Seq,
			PromptText:         event.Message.Text,
			ReplyToMessageID:   event.Message.ServerMsgID,
			RecursiveTrigger:   senderType == SenderTypeAgent,
			SourceAgentRunID:   event.Message.AgentMetadata.AgentRunID,
			SourceAgentUserID:  sourceAgentUserID(event.Message),
			SourceMessageID:    event.Message.ServerMsgID,
			SourceMessageSeq:   event.Message.Seq,
			SourceMessageText:  event.Message.Text,
			SourceContentType:  event.Message.ContentType,
			TargetAgentUserIDs: append([]string(nil), targetAgentIDs...),
		})
	}
	return triggers, nil
}

func validateMessageCreatedEvent(event MessageCreatedEvent) error {
	if _, err := normalizeRequired(event.EventID, "event_id"); err != nil {
		return err
	}
	if _, err := normalizeRequired(event.ConversationID, "conversation_id"); err != nil {
		return err
	}
	if _, err := normalizeConversationType(event.ConversationType); err != nil {
		return err
	}
	if _, err := normalizeRequired(event.Message.ServerMsgID, "server_msg_id"); err != nil {
		return err
	}
	if event.Message.Seq <= 0 {
		return apperror.InvalidArgument("seq must be greater than 0")
	}
	if _, err := normalizeRequired(event.Message.SenderID, "sender_id"); err != nil {
		return err
	}
	if _, err := normalizeSenderType(event.Message.SenderType); err != nil {
		return err
	}
	contentType, err := normalizeRequired(event.Message.ContentType, "content_type")
	if err != nil {
		return err
	}
	if contentType != ContentTypeText {
		return apperror.InvalidArgument("content_type must be text")
	}
	return nil
}

func agentMessageRecursionAllowed(event MessageCreatedEvent, policy TriggerPolicy) bool {
	return policy.AllowAgentMessageRecursion && event.Message.AgentMetadata.AllowRecursiveTrigger
}

func messageTriggerTypeForAgent(event MessageCreatedEvent, conversationType string, agentUserID string) (string, bool) {
	switch conversationType {
	case ConversationTypeSingle:
		if event.Message.ReceiverID != agentUserID {
			return "", false
		}
		return TriggerTypeUserPrivateMessage, true
	case ConversationTypeGroup:
		if event.Message.GroupID == "" || !containsID(event.Message.AtUserIDs, agentUserID) {
			return "", false
		}
		return TriggerTypeGroupMention, true
	default:
		return "", false
	}
}

func sourceAgentUserID(message MessageEnvelope) string {
	if message.SenderType == SenderTypeAgent {
		return message.SenderID
	}
	return ""
}
