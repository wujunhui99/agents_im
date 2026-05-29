package eino

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components"
	einomodel "github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	einojsonschema "github.com/eino-contrib/jsonschema"
	"github.com/wujunhui99/agents_im/internal/agentruntime"
	llmdeepseek "github.com/wujunhui99/agents_im/internal/agentruntime/llm/deepseek"
	runtimetools "github.com/wujunhui99/agents_im/internal/agentruntime/tools"
	"github.com/wujunhui99/agents_im/pkg/apperror"
	"github.com/wujunhui99/agents_im/pkg/config"
	"github.com/wujunhui99/agents_im/pkg/idgen"
	"github.com/wujunhui99/agents_im/pkg/llmobs"
	"github.com/wujunhui99/agents_im/pkg/observability"
)

type DeepSeekRuntime struct {
	cfg              config.DeepSeekConfig
	llmobsSink       llmobs.Sink
	llmobsCfg        llmobs.Config
	toolProvider     runtimetools.Provider
	chatModelFactory deepSeekChatModelFactory
}

type DeepSeekRuntimeOption func(*DeepSeekRuntime)

type deepSeekChatModelFactory func(ctx context.Context, cfg config.DeepSeekConfig) (einomodel.ToolCallingChatModel, error)

const defaultMaxDeepSeekRuntimeToolCalls = 8

var deepSeekToolNameUnsafeChars = regexp.MustCompile(`[^a-zA-Z0-9_-]`)

func WithLLMObservability(sink llmobs.Sink, cfg llmobs.Config) DeepSeekRuntimeOption {
	return func(runtime *DeepSeekRuntime) {
		runtime.llmobsSink = sink
		runtime.llmobsCfg = cfg
	}
}

func WithToolProvider(provider runtimetools.Provider) DeepSeekRuntimeOption {
	return func(runtime *DeepSeekRuntime) {
		runtime.toolProvider = provider
	}
}

func WithChatModelFactory(factory func(ctx context.Context, cfg config.DeepSeekConfig) (einomodel.ToolCallingChatModel, error)) DeepSeekRuntimeOption {
	return func(runtime *DeepSeekRuntime) {
		runtime.chatModelFactory = factory
	}
}

func NewDeepSeekRuntime(cfg config.DeepSeekConfig, opts ...DeepSeekRuntimeOption) *DeepSeekRuntime {
	runtime := &DeepSeekRuntime{cfg: cfg, chatModelFactory: llmdeepseek.NewChatModel}
	for _, opt := range opts {
		if opt != nil {
			opt(runtime)
		}
	}
	return runtime
}

func (r *DeepSeekRuntime) Run(ctx context.Context, req agentruntime.RunRequest) (agentruntime.RunResult, error) {
	ctx, span := observability.StartSpan(ctx, "agentruntime.eino.run")
	defer span.End()
	normalized, err := agentruntime.NormalizeRunRequest(req)
	if err != nil {
		observability.RecordSpanError(span, err)
		return agentruntime.RunResult{}, err
	}
	if normalized.TraceID == "" {
		normalized.TraceID = observability.TraceIDFromContext(ctx)
	}
	cfg := config.ResolveDeepSeekConfig(r.cfg)
	if strings.TrimSpace(normalized.Agent.Model.Model) != "" {
		cfg.Model = normalized.Agent.Model.Model
	}
	factory := r.chatModelFactory
	if factory == nil {
		factory = llmdeepseek.NewChatModel
	}
	cm, err := factory(ctx, cfg)
	if err != nil {
		observability.RecordSpanError(span, err)
		return agentruntime.RunResult{}, err
	}

	runID := normalized.RunID
	if runID == "" {
		generated, err := idgen.NewString()
		if err != nil {
			observability.RecordSpanError(span, err)
			return agentruntime.RunResult{}, err
		}
		runID = generated
	}
	normalized.RunID = runID
	resolvedTools, err := r.resolveTools(ctx, normalized, runID)
	if err != nil {
		observability.RecordSpanError(span, err)
		return agentruntime.RunResult{}, err
	}
	if len(resolvedTools) > 0 {
		toolInfos, err := toolInfosFromResolvedTools(resolvedTools)
		if err != nil {
			observability.RecordSpanError(span, err)
			return agentruntime.RunResult{}, err
		}
		cm, err = cm.WithTools(toolInfos)
		if err != nil {
			observability.RecordSpanError(span, err)
			return agentruntime.RunResult{}, fmt.Errorf("bind deepseek tools: %w", err)
		}
	}

	startedAt := time.Now().UTC()
	if r.llmobsSink != nil {
		ctx = callbacks.InitCallbacks(ctx, &callbacks.RunInfo{
			Type:      "DeepSeek",
			Name:      cfg.Model,
			Component: components.ComponentOfChatModel,
		}, llmobs.NewEinoCallbackHandler(r.llmobsSink, llmObsBaseEvent(normalized, cfg), r.llmobsCfg))
	}
	resp, toolCallResults, err := r.generateWithTools(ctx, cm, normalized, runID, resolvedTools)
	finishedAt := time.Now().UTC()
	if err != nil {
		observability.RecordSpanError(span, err)
		return agentruntime.RunResult{}, fmt.Errorf("deepseek generate AI hosting reply: %w", err)
	}
	if resp == nil {
		err := apperror.Internal("deepseek returned empty response")
		observability.RecordSpanError(span, err)
		return agentruntime.RunResult{}, err
	}
	result := agentruntime.RunResult{
		RunID:     runID,
		FinalText: strings.TrimSpace(resp.Content),
		Model: agentruntime.ModelMetadata{
			Provider: "deepseek",
			Model:    cfg.Model,
		},
		StartedAt:  startedAt,
		FinishedAt: finishedAt,
		ToolCalls:  toolCallResults,
	}
	if resp.ResponseMeta != nil {
		result.FinishReason = resp.ResponseMeta.FinishReason
		if resp.ResponseMeta.Usage != nil {
			result.Usage = agentruntime.Usage{
				PromptTokens:     int64(resp.ResponseMeta.Usage.PromptTokens),
				CompletionTokens: int64(resp.ResponseMeta.Usage.CompletionTokens),
				ReasoningTokens:  int64(resp.ResponseMeta.Usage.CompletionTokensDetails.ReasoningTokens),
				CachedTokens:     int64(resp.ResponseMeta.Usage.PromptTokenDetails.CachedTokens),
				TotalTokens:      int64(resp.ResponseMeta.Usage.TotalTokens),
			}
		}
	}
	return result, nil
}

func (r *DeepSeekRuntime) resolveTools(ctx context.Context, req agentruntime.RunRequest, runID string) ([]runtimetools.ResolvedTool, error) {
	if r == nil || r.toolProvider == nil {
		return nil, nil
	}
	toolIDs := make([]string, 0, len(req.Agent.Tools))
	for _, tool := range req.Agent.Tools {
		if tool.ToolID != "" {
			toolIDs = append(toolIDs, tool.ToolID)
		}
	}
	resolved, err := r.toolProvider.ResolveAgentTools(ctx, runtimetools.ResolveAgentToolsRequest{
		AgentID:         req.Agent.AgentID,
		ToolIDs:         toolIDs,
		RequireAdapters: true,
		RunID:           runID,
		TraceID:         req.TraceID,
		RequestID:       req.RequestID,
	})
	if err != nil {
		return nil, err
	}
	return resolved, nil
}

func (r *DeepSeekRuntime) generateWithTools(
	ctx context.Context,
	cm einomodel.ToolCallingChatModel,
	req agentruntime.RunRequest,
	runID string,
	resolvedTools []runtimetools.ResolvedTool,
) (*schema.Message, []agentruntime.ToolCallResult, error) {
	messages := runtimeMessages(req)
	toolByName := make(map[string]runtimetools.ResolvedTool, len(resolvedTools))
	for _, tool := range resolvedTools {
		toolByName[deepSeekToolFunctionName(tool.Spec)] = tool
	}
	maxToolCalls := req.Agent.Policy.MaxToolCalls
	if maxToolCalls <= 0 {
		maxToolCalls = defaultMaxDeepSeekRuntimeToolCalls
	}
	toolCallResults := make([]agentruntime.ToolCallResult, 0)
	executedToolCalls := 0

	for {
		generateCtx, generateSpan := observability.StartSpan(ctx, "llm.generate")
		resp, err := cm.Generate(generateCtx, messages)
		if err != nil {
			observability.RecordSpanError(generateSpan, err)
			generateSpan.End()
			return nil, toolCallResults, err
		}
		if resp == nil {
			err := apperror.Internal("deepseek returned empty response")
			observability.RecordSpanError(generateSpan, err)
			generateSpan.End()
			return nil, toolCallResults, err
		}
		generateSpan.End()
		ctx = generateCtx
		if len(resp.ToolCalls) == 0 {
			return resp, toolCallResults, nil
		}
		messages = append(messages, resp)
		for _, call := range resp.ToolCalls {
			if executedToolCalls >= maxToolCalls {
				return nil, toolCallResults, apperror.InvalidArgument("max tool calls exceeded")
			}
			executedToolCalls++
			toolMessage, callResult, err := executeRuntimeToolCall(ctx, req, runID, toolByName, call)
			toolCallResults = append(toolCallResults, callResult)
			if err != nil {
				return nil, toolCallResults, err
			}
			messages = append(messages, toolMessage)
		}
	}
}

func executeRuntimeToolCall(
	ctx context.Context,
	req agentruntime.RunRequest,
	runID string,
	toolByName map[string]runtimetools.ResolvedTool,
	call schema.ToolCall,
) (*schema.Message, agentruntime.ToolCallResult, error) {
	ctx, span := observability.StartSpan(ctx, "agentruntime.tool_call")
	defer span.End()
	toolName := strings.TrimSpace(call.Function.Name)
	result := agentruntime.ToolCallResult{
		ToolCallID: strings.TrimSpace(call.ID),
		ToolName:   toolName,
		Status:     "failed",
	}
	startedAt := time.Now()
	if toolName == "" {
		err := apperror.InvalidArgument("tool call function name is required")
		result.ErrorCode = string(apperror.From(err).Code)
		result.ErrorMessage = err.Error()
		result.DurationMs = time.Since(startedAt).Milliseconds()
		observability.RecordSpanError(span, err)
		return nil, result, err
	}
	resolved, ok := toolByName[toolName]
	if !ok || resolved.Adapter == nil {
		err := apperror.Forbidden("tool call is not approved for this agent")
		result.ErrorCode = string(apperror.From(err).Code)
		result.ErrorMessage = err.Error()
		result.DurationMs = time.Since(startedAt).Milliseconds()
		observability.RecordSpanError(span, err)
		return nil, result, err
	}
	result.ToolID = resolved.Spec.ToolID
	result.ToolName = resolved.Spec.Name

	toolResult, err := resolved.Adapter.Invoke(ctx, runtimetools.ToolCall{
		RunID:            runID,
		AgentID:          req.Agent.AgentID,
		RequestingUserID: req.RequestingUserID,
		ConversationID:   req.ConversationID,
		ToolID:           resolved.Spec.ToolID,
		ToolName:         resolved.Spec.Name,
		InputJSON:        json.RawMessage(strings.TrimSpace(call.Function.Arguments)),
		TraceID:          req.TraceID,
		RequestID:        req.RequestID,
	})
	result.DurationMs = time.Since(startedAt).Milliseconds()
	if err != nil {
		appErr := apperror.From(err)
		result.ErrorCode = string(appErr.Code)
		result.ErrorMessage = err.Error()
		observability.RecordSpanError(span, err)
		if isRecoverableToolInputError(appErr) {
			content, marshalErr := json.Marshal(map[string]string{
				"error_code":    string(appErr.Code),
				"error_message": appErr.Message,
			})
			if marshalErr != nil {
				return nil, result, marshalErr
			}
			return schema.ToolMessage(string(content), call.ID, schema.WithToolName(toolName)), result, nil
		}
		return nil, result, err
	}
	result.Status = "succeeded"
	content := strings.TrimSpace(toolResult.Content)
	if content == "" && len(toolResult.OutputJSON) > 0 {
		content = string(toolResult.OutputJSON)
	}
	if content == "" {
		content = "{}"
	}
	return schema.ToolMessage(content, call.ID, schema.WithToolName(toolName)), result, nil
}

func isRecoverableToolInputError(err *apperror.Error) bool {
	if err == nil {
		return false
	}
	switch err.Code {
	case apperror.CodeInvalidArgument, apperror.CodeForbidden, apperror.CodeNotFound:
		return true
	default:
		return false
	}
}

func toolInfosFromResolvedTools(resolvedTools []runtimetools.ResolvedTool) ([]*schema.ToolInfo, error) {
	infos := make([]*schema.ToolInfo, 0, len(resolvedTools))
	for _, resolved := range resolvedTools {
		info, err := toolInfoFromSpec(resolved.Spec)
		if err != nil {
			return nil, err
		}
		infos = append(infos, info)
	}
	return infos, nil
}

func toolInfoFromSpec(spec runtimetools.ToolSpec) (*schema.ToolInfo, error) {
	name := deepSeekToolFunctionName(spec)
	if name == "" {
		return nil, apperror.InvalidArgument("tool name is required")
	}
	rawSchema := strings.TrimSpace(spec.InputSchemaJSON)
	if rawSchema == "" {
		rawSchema = `{"type":"object"}`
	}
	var inputSchema einojsonschema.Schema
	if err := json.Unmarshal([]byte(rawSchema), &inputSchema); err != nil {
		return nil, apperror.InvalidArgument("tool input_schema_json must be valid JSON Schema: " + err.Error())
	}
	return &schema.ToolInfo{
		Name:        name,
		Desc:        strings.TrimSpace(spec.Description),
		ParamsOneOf: schema.NewParamsOneOfByJSONSchema(&inputSchema),
	}, nil
}

func deepSeekToolFunctionName(spec runtimetools.ToolSpec) string {
	name := strings.TrimSpace(spec.Name)
	if name == "" && spec.Local != nil {
		name = strings.TrimSpace(spec.Local.HandlerKey)
	}
	name = deepSeekToolNameUnsafeChars.ReplaceAllString(name, "_")
	name = strings.Trim(name, "_")
	return name
}

func llmObsBaseEvent(req agentruntime.RunRequest, cfg config.DeepSeekConfig) llmobs.Event {
	runtimeMode := strings.TrimSpace(req.Metadata["runtime_mode"])
	if runtimeMode == "" {
		runtimeMode = strings.TrimSpace(req.TriggerType)
	}
	return llmobs.Event{
		TraceID:              strings.TrimSpace(req.TraceID),
		RequestID:            strings.TrimSpace(req.RequestID),
		AgentRunID:           strings.TrimSpace(req.RunID),
		ConversationID:       strings.TrimSpace(req.ConversationID),
		TriggerServerMsgID:   strings.TrimSpace(req.TriggerMessageID),
		HostedOwnerAccountID: strings.TrimSpace(req.AgentUserID),
		SenderAccountID:      strings.TrimSpace(req.RequestingUserID),
		AgentAccountID:       strings.TrimSpace(req.AgentUserID),
		ModelProvider:        strings.TrimSpace(req.Agent.Model.Provider),
		ModelName:            strings.TrimSpace(cfg.Model),
		PromptVersion:        strings.TrimSpace(req.Agent.Prompt.Version),
		PromptHash:           llmobs.PromptHash(req.Agent.Prompt.Content),
		RuntimeMode:          runtimeMode,
		Generation: llmobs.Generation{
			BoundedRecentMessageCount: len(req.Conversation),
			TriggerInContext:          llmObsTriggerInContext(req),
		},
	}
}

func llmObsTriggerInContext(req agentruntime.RunRequest) bool {
	triggerMessageID := strings.TrimSpace(req.TriggerMessageID)
	for _, message := range req.Conversation {
		if triggerMessageID != "" && strings.TrimSpace(message.ServerMsgID) == triggerMessageID {
			return true
		}
		if req.TriggerSeq > 0 && message.Seq == req.TriggerSeq {
			return true
		}
	}
	return false
}

func runtimeMessages(req agentruntime.RunRequest) []*schema.Message {
	messages := []*schema.Message{schema.SystemMessage(req.Agent.Prompt.Content)}
	for _, message := range req.Conversation {
		text := strings.TrimSpace(message.Text)
		if text == "" {
			continue
		}
		if isCurrentTriggerMessage(req, message) {
			continue
		}
		if message.SenderType == agentruntime.SenderTypeAgent {
			messages = append(messages, schema.AssistantMessage(text, nil))
			continue
		}
		messages = append(messages, schema.UserMessage(text))
	}
	if current := runtimeCurrentTaskMessage(req); current != "" {
		messages = append(messages, schema.UserMessage(current))
	}
	return messages
}

func isCurrentTriggerMessage(req agentruntime.RunRequest, message agentruntime.ConversationMessage) bool {
	triggerMessageID := strings.TrimSpace(req.TriggerMessageID)
	if triggerMessageID != "" && strings.TrimSpace(message.ServerMsgID) == triggerMessageID {
		return true
	}
	if req.TriggerSeq <= 0 || message.Seq != req.TriggerSeq {
		return false
	}
	text := strings.TrimSpace(message.Text)
	if source := strings.TrimSpace(req.SourceMessageText); source != "" && text == source {
		return true
	}
	return text != "" && text == strings.TrimSpace(req.PromptText)
}

func runtimeCurrentTaskMessage(req agentruntime.RunRequest) string {
	promptText := strings.TrimSpace(req.PromptText)
	if promptText == "" {
		return ""
	}
	return strings.TrimSpace(`当前需要回复的对方消息：
` + promptText + `

请直接生成要发送给对方的回复。如果这条消息提出明确问题、请求或任务，直接回答或完成；只有缺少必要信息导致无法完成时才简短询问澄清。不要只回复“可以”“好的”“你说说”等泛泛确认。`)
}
