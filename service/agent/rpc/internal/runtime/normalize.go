package runtime

import (
	"strings"
	"time"

	"github.com/wujunhui99/agents_im/pkg/apperror"
)

const maxIdentifierLength = 256

func NormalizeRunRequest(input RunRequest) (RunRequest, error) {
	var err error
	input.RunID = normalizeOptional(input.RunID)
	input.RequestID, err = normalizeRequiredID(input.RequestID, "request_id")
	if err != nil {
		return RunRequest{}, err
	}
	input.EventID = normalizeOptional(input.EventID)
	input.OperationID = normalizeOptional(input.OperationID)
	input.TraceID = normalizeOptional(input.TraceID)
	input.TriggerType, err = normalizeTriggerType(input.TriggerType)
	if err != nil {
		return RunRequest{}, err
	}
	input.AgentUserID, err = normalizeRequiredID(input.AgentUserID, "agent_user_id")
	if err != nil {
		return RunRequest{}, err
	}
	input.RequestingUserID, err = normalizeRequiredID(input.RequestingUserID, "requesting_user_id")
	if err != nil {
		return RunRequest{}, err
	}
	input.ConversationID, err = normalizeRequiredID(input.ConversationID, "conversation_id")
	if err != nil {
		return RunRequest{}, err
	}
	input.ConversationType, err = normalizeConversationType(input.ConversationType)
	if err != nil {
		return RunRequest{}, err
	}
	input.TriggerMessageID = normalizeOptional(input.TriggerMessageID)
	input.PromptText, err = normalizeRequiredText(input.PromptText, "prompt_text")
	if err != nil {
		return RunRequest{}, err
	}
	input.ReplyToMessageID = normalizeOptional(input.ReplyToMessageID)
	input.SourceAgentRunID = normalizeOptional(input.SourceAgentRunID)
	input.SourceAgentUserID = normalizeOptional(input.SourceAgentUserID)
	input.SourceMessageID = normalizeOptional(input.SourceMessageID)
	input.SourceContentType = normalizeOptional(input.SourceContentType)
	if input.TriggerSeq < 0 {
		return RunRequest{}, apperror.InvalidArgument("trigger_seq cannot be negative")
	}
	if input.SourceMessageSeq < 0 {
		return RunRequest{}, apperror.InvalidArgument("source_message_seq cannot be negative")
	}
	input.TargetAgentUserIDs = normalizeIDSlice(input.TargetAgentUserIDs)
	input.Agent, err = NormalizeAgentConfig(input.Agent)
	if err != nil {
		return RunRequest{}, err
	}
	if input.Agent.AgentUserID != input.AgentUserID {
		return RunRequest{}, apperror.InvalidArgument("agent_user_id must match agent config")
	}
	input.Conversation, err = normalizeConversationMessages(input.Conversation)
	if err != nil {
		return RunRequest{}, err
	}
	input.Metadata = normalizeStringMap(input.Metadata)
	return input, nil
}

func (r RunRequest) Normalize() (RunRequest, error) {
	return NormalizeRunRequest(r)
}

func (r RunRequest) Validate() error {
	_, err := NormalizeRunRequest(r)
	return err
}

func NormalizeAgentConfig(input AgentConfig) (AgentConfig, error) {
	var err error
	input.AgentID, err = normalizeRequiredID(input.AgentID, "agent_id")
	if err != nil {
		return AgentConfig{}, err
	}
	input.AgentUserID, err = normalizeRequiredID(input.AgentUserID, "agent_user_id")
	if err != nil {
		return AgentConfig{}, err
	}
	input.Name = normalizeOptional(input.Name)
	input.Description = strings.TrimSpace(input.Description)
	input.Status, err = normalizeAgentStatus(input.Status)
	if err != nil {
		return AgentConfig{}, err
	}
	input.Prompt, err = NormalizePromptRef(input.Prompt)
	if err != nil {
		return AgentConfig{}, err
	}
	input.Model, err = NormalizeModelConfig(input.Model)
	if err != nil {
		return AgentConfig{}, err
	}
	input.Tools, err = normalizeToolRefs(input.Tools)
	if err != nil {
		return AgentConfig{}, err
	}
	input.Skills, err = normalizeSkillRefs(input.Skills)
	if err != nil {
		return AgentConfig{}, err
	}
	if err := input.Policy.Validate(); err != nil {
		return AgentConfig{}, err
	}
	return input, nil
}

func (c AgentConfig) Normalize() (AgentConfig, error) {
	return NormalizeAgentConfig(c)
}

func (c AgentConfig) Validate() error {
	_, err := NormalizeAgentConfig(c)
	return err
}

func NormalizePromptRef(input PromptRef) (PromptRef, error) {
	var err error
	input.PromptID, err = normalizeRequiredID(input.PromptID, "prompt_id")
	if err != nil {
		return PromptRef{}, err
	}
	input.Name = normalizeOptional(input.Name)
	input.Description = strings.TrimSpace(input.Description)
	input.Content, err = normalizeRequiredText(input.Content, "prompt_content")
	if err != nil {
		return PromptRef{}, err
	}
	input.Version = normalizeOptional(input.Version)
	input.VariablesSchemaJSON = strings.TrimSpace(input.VariablesSchemaJSON)
	return input, nil
}

func (p PromptRef) Normalize() (PromptRef, error) {
	return NormalizePromptRef(p)
}

func (p PromptRef) Validate() error {
	_, err := NormalizePromptRef(p)
	return err
}

func NormalizeModelConfig(input ModelConfig) (ModelConfig, error) {
	var err error
	input.Provider, err = normalizeRequiredID(input.Provider, "model_provider")
	if err != nil {
		return ModelConfig{}, err
	}
	input.Model, err = normalizeRequiredID(input.Model, "model")
	if err != nil {
		return ModelConfig{}, err
	}
	input.ModelVersion = normalizeOptional(input.ModelVersion)
	input.BaseURL = normalizeOptional(input.BaseURL)
	input.CredentialRef = normalizeOptional(input.CredentialRef)
	if input.Temperature != nil && (*input.Temperature < 0 || *input.Temperature > 2) {
		return ModelConfig{}, apperror.InvalidArgument("temperature must be between 0 and 2")
	}
	if input.MaxTokens < 0 {
		return ModelConfig{}, apperror.InvalidArgument("max_tokens cannot be negative")
	}
	input.Metadata = normalizeStringMap(input.Metadata)
	return input, nil
}

func (c ModelConfig) Normalize() (ModelConfig, error) {
	return NormalizeModelConfig(c)
}

func (c ModelConfig) Validate() error {
	_, err := NormalizeModelConfig(c)
	return err
}

func (p RuntimePolicy) Validate() error {
	if p.MaxToolCalls < 0 {
		return apperror.InvalidArgument("max_tool_calls cannot be negative")
	}
	if p.MaxRunDuration < 0 {
		return apperror.InvalidArgument("max_run_duration cannot be negative")
	}
	if p.MaxRecursiveDepth < 0 {
		return apperror.InvalidArgument("max_recursive_depth cannot be negative")
	}
	return nil
}

func NormalizeRunResult(input RunResult) (RunResult, error) {
	var err error
	input.RunID, err = normalizeRequiredID(input.RunID, "run_id")
	if err != nil {
		return RunResult{}, err
	}
	input.FinalText, err = normalizeRequiredText(input.FinalText, "final_text")
	if err != nil {
		return RunResult{}, err
	}
	input.Model = normalizeModelMetadata(input.Model)
	if err := input.Usage.Validate(); err != nil {
		return RunResult{}, err
	}
	input.FinishReason = normalizeOptional(input.FinishReason)
	input.ToolCalls, err = normalizeToolCallResults(input.ToolCalls)
	if err != nil {
		return RunResult{}, err
	}
	input.Metadata = normalizeStringMap(input.Metadata)
	input.StartedAt = utcOrZero(input.StartedAt)
	input.FinishedAt = utcOrZero(input.FinishedAt)
	return input, nil
}

func (r RunResult) Normalize() (RunResult, error) {
	return NormalizeRunResult(r)
}

func (r RunResult) Validate() error {
	_, err := NormalizeRunResult(r)
	return err
}

func (u Usage) Validate() error {
	if u.PromptTokens < 0 {
		return apperror.InvalidArgument("prompt_tokens cannot be negative")
	}
	if u.CompletionTokens < 0 {
		return apperror.InvalidArgument("completion_tokens cannot be negative")
	}
	if u.ReasoningTokens < 0 {
		return apperror.InvalidArgument("reasoning_tokens cannot be negative")
	}
	if u.CachedTokens < 0 {
		return apperror.InvalidArgument("cached_tokens cannot be negative")
	}
	if u.TotalTokens < 0 {
		return apperror.InvalidArgument("total_tokens cannot be negative")
	}
	return nil
}

func normalizeToolRefs(input []ToolRef) ([]ToolRef, error) {
	if len(input) == 0 {
		return nil, nil
	}
	result := make([]ToolRef, 0, len(input))
	for i, tool := range input {
		normalized, err := normalizeToolRef(tool)
		if err != nil {
			return nil, apperror.InvalidArgument("tools[" + itoa(i) + "]: " + err.Error())
		}
		result = append(result, normalized)
	}
	return result, nil
}

func normalizeToolRef(input ToolRef) (ToolRef, error) {
	var err error
	input.ToolID, err = normalizeRequiredID(input.ToolID, "tool_id")
	if err != nil {
		return ToolRef{}, err
	}
	input.Name, err = normalizeRequiredID(input.Name, "tool_name")
	if err != nil {
		return ToolRef{}, err
	}
	input.Description = strings.TrimSpace(input.Description)
	input.ToolType, err = normalizeToolType(input.ToolType)
	if err != nil {
		return ToolRef{}, err
	}
	input.MCPServerID = normalizeOptional(input.MCPServerID)
	input.MCPToolName = normalizeOptional(input.MCPToolName)
	input.LocalHandlerKey = normalizeOptional(input.LocalHandlerKey)
	input.BuiltinKey = normalizeOptional(input.BuiltinKey)
	input.InputSchemaJSON = strings.TrimSpace(input.InputSchemaJSON)
	input.OutputSchemaJSON = strings.TrimSpace(input.OutputSchemaJSON)
	input.PermissionLevel = normalizeOptional(input.PermissionLevel)

	switch input.ToolType {
	case ToolTypeMCP:
		if input.MCPServerID == "" {
			return ToolRef{}, apperror.InvalidArgument("mcp tools require mcp_server_id")
		}
		if input.MCPToolName == "" {
			return ToolRef{}, apperror.InvalidArgument("mcp tools require mcp_tool_name")
		}
		if input.LocalHandlerKey != "" || input.BuiltinKey != "" {
			return ToolRef{}, apperror.InvalidArgument("mcp tools cannot include local handler or builtin keys")
		}
	case ToolTypeLocal:
		if input.LocalHandlerKey == "" {
			return ToolRef{}, apperror.InvalidArgument("local tools require local_handler_key")
		}
		if input.MCPServerID != "" || input.MCPToolName != "" || input.BuiltinKey != "" {
			return ToolRef{}, apperror.InvalidArgument("local tools can only use local handler metadata")
		}
	case ToolTypeBuiltin:
		if input.BuiltinKey == "" {
			return ToolRef{}, apperror.InvalidArgument("builtin tools require builtin_key")
		}
		if input.MCPServerID != "" || input.MCPToolName != "" || input.LocalHandlerKey != "" {
			return ToolRef{}, apperror.InvalidArgument("builtin tools can only use builtin metadata")
		}
	}
	return input, nil
}

func normalizeSkillRefs(input []SkillRef) ([]SkillRef, error) {
	if len(input) == 0 {
		return nil, nil
	}
	result := make([]SkillRef, 0, len(input))
	for i, skill := range input {
		normalized, err := normalizeSkillRef(skill)
		if err != nil {
			return nil, apperror.InvalidArgument("skills[" + itoa(i) + "]: " + err.Error())
		}
		result = append(result, normalized)
	}
	return result, nil
}

func normalizeSkillRef(input SkillRef) (SkillRef, error) {
	var err error
	input.SkillID, err = normalizeRequiredID(input.SkillID, "skill_id")
	if err != nil {
		return SkillRef{}, err
	}
	input.Name = normalizeOptional(input.Name)
	input.Description = strings.TrimSpace(input.Description)
	input.Version = normalizeOptional(input.Version)
	input.ObjectKey = normalizeOptional(input.ObjectKey)
	input.SHA256 = normalizeOptional(input.SHA256)
	input.ContentType = normalizeOptional(input.ContentType)
	if input.SizeBytes < 0 {
		return SkillRef{}, apperror.InvalidArgument("size_bytes cannot be negative")
	}
	return input, nil
}

func normalizeConversationMessages(input []ConversationMessage) ([]ConversationMessage, error) {
	if len(input) == 0 {
		return nil, nil
	}
	result := make([]ConversationMessage, 0, len(input))
	for i, message := range input {
		normalized, err := normalizeConversationMessage(message)
		if err != nil {
			return nil, apperror.InvalidArgument("conversation[" + itoa(i) + "]: " + err.Error())
		}
		result = append(result, normalized)
	}
	return result, nil
}

func normalizeConversationMessage(input ConversationMessage) (ConversationMessage, error) {
	var err error
	input.ServerMsgID = normalizeOptional(input.ServerMsgID)
	input.SenderID, err = normalizeRequiredID(input.SenderID, "sender_id")
	if err != nil {
		return ConversationMessage{}, err
	}
	input.SenderType, err = normalizeSenderType(input.SenderType)
	if err != nil {
		return ConversationMessage{}, err
	}
	input.ContentType, err = normalizeContentType(input.ContentType)
	if err != nil {
		return ConversationMessage{}, err
	}
	if strings.TrimSpace(input.Text) == "" {
		return ConversationMessage{}, apperror.InvalidArgument("text is required")
	}
	input.AgentRunID = normalizeOptional(input.AgentRunID)
	if input.Seq < 0 {
		return ConversationMessage{}, apperror.InvalidArgument("seq cannot be negative")
	}
	if input.CreatedAtMs < 0 {
		return ConversationMessage{}, apperror.InvalidArgument("created_at_ms cannot be negative")
	}
	return input, nil
}

func normalizeToolCallResults(input []ToolCallResult) ([]ToolCallResult, error) {
	if len(input) == 0 {
		return nil, nil
	}
	result := make([]ToolCallResult, 0, len(input))
	for i, call := range input {
		call.ToolCallID = normalizeOptional(call.ToolCallID)
		call.ToolID = normalizeOptional(call.ToolID)
		call.ToolName = normalizeOptional(call.ToolName)
		call.Status = normalizeOptional(call.Status)
		call.ErrorCode = normalizeOptional(call.ErrorCode)
		call.ErrorMessage = strings.TrimSpace(call.ErrorMessage)
		if call.DurationMs < 0 {
			return nil, apperror.InvalidArgument("tool_calls[" + itoa(i) + "]: duration_ms cannot be negative")
		}
		call.Metadata = normalizeStringMap(call.Metadata)
		result = append(result, call)
	}
	return result, nil
}

func normalizeRequiredID(value string, field string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", apperror.InvalidArgument(field + " is required")
	}
	if len([]rune(value)) > maxIdentifierLength {
		return "", apperror.InvalidArgument(field + " must be 256 characters or fewer")
	}
	return value, nil
}

func normalizeRequiredText(value string, field string) (string, error) {
	if strings.TrimSpace(value) == "" {
		return "", apperror.InvalidArgument(field + " is required")
	}
	return value, nil
}

func normalizeOptional(value string) string {
	return strings.TrimSpace(value)
}

func normalizeConversationType(value string) (string, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	switch value {
	case ConversationTypeSingle, ConversationTypeGroup:
		return value, nil
	default:
		return "", apperror.InvalidArgument("conversation_type must be single or group")
	}
}

func normalizeSenderType(value string) (string, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	switch value {
	case SenderTypeUser, SenderTypeAgent:
		return value, nil
	default:
		return "", apperror.InvalidArgument("sender_type must be user or agent")
	}
}

func normalizeContentType(value string) (string, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	switch value {
	case ContentTypeText:
		return value, nil
	default:
		return "", apperror.InvalidArgument("content_type must be text")
	}
}

func normalizeTriggerType(value string) (string, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	switch value {
	case TriggerTypeUserPrivateMessage, TriggerTypeGroupMention, TriggerTypeAdminManualRun:
		return value, nil
	default:
		return "", apperror.InvalidArgument("trigger_type must be user_private_message, group_mention, or admin_manual_run")
	}
}

func normalizeAgentStatus(value string) (string, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return "", nil
	}
	switch value {
	case AgentStatusDraft, AgentStatusActive, AgentStatusDisabled, AgentStatusArchived:
		return value, nil
	default:
		return "", apperror.InvalidArgument("agent status must be draft, active, disabled, or archived")
	}
}

func normalizeToolType(value string) (string, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	switch value {
	case ToolTypeMCP, ToolTypeLocal, ToolTypeBuiltin:
		return value, nil
	default:
		return "", apperror.InvalidArgument("tool_type must be mcp, local, or builtin")
	}
}

func normalizeIDSlice(input []string) []string {
	if len(input) == 0 {
		return nil
	}
	result := make([]string, 0, len(input))
	seen := make(map[string]struct{}, len(input))
	for _, value := range input {
		id := strings.TrimSpace(value)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		result = append(result, id)
	}
	return result
}

func normalizeStringMap(input map[string]string) map[string]string {
	if len(input) == 0 {
		return nil
	}
	result := make(map[string]string, len(input))
	for key, value := range input {
		normalizedKey := strings.TrimSpace(key)
		if normalizedKey == "" {
			continue
		}
		result[normalizedKey] = strings.TrimSpace(value)
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func normalizeModelMetadata(input ModelMetadata) ModelMetadata {
	input.Provider = normalizeOptional(input.Provider)
	input.Model = normalizeOptional(input.Model)
	input.ModelVersion = normalizeOptional(input.ModelVersion)
	input.ResponseID = normalizeOptional(input.ResponseID)
	input.Metadata = normalizeStringMap(input.Metadata)
	return input
}

func utcOrZero(value time.Time) time.Time {
	if value.IsZero() {
		return time.Time{}
	}
	return value.UTC()
}

func itoa(value int) string {
	if value == 0 {
		return "0"
	}
	var digits [20]byte
	i := len(digits)
	n := value
	for n > 0 {
		i--
		digits[i] = byte('0' + n%10)
		n /= 10
	}
	return string(digits[i:])
}
