package llmobs

import (
	"context"
	"time"

	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components/model"
)

type callbackStartedAtKey struct{}

func NewEinoCallbackHandler(sink Sink, base Event, capture Config) callbacks.Handler {
	return callbacks.NewHandlerBuilder().
		OnStartFn(func(ctx context.Context, _ *callbacks.RunInfo, input callbacks.CallbackInput) context.Context {
			startedAt := time.Now().UTC()
			event := cloneEvent(base)
			event.Type = EventTypeGeneration
			event.Status = StatusStarted
			event.StartedAt = startedAt
			if modelInput := model.ConvCallbackInput(input); modelInput != nil && event.Generation.BoundedRecentMessageCount == 0 {
				event.Generation.BoundedRecentMessageCount = len(modelInput.Messages)
			}
			observe(ctx, sink, event)
			return context.WithValue(ctx, callbackStartedAtKey{}, startedAt)
		}).
		OnEndFn(func(ctx context.Context, _ *callbacks.RunInfo, output callbacks.CallbackOutput) context.Context {
			event := cloneEvent(base)
			event.Type = EventTypeGeneration
			event.Status = StatusSucceeded
			event.FinishedAt = time.Now().UTC()
			if startedAt, ok := ctx.Value(callbackStartedAtKey{}).(time.Time); ok && !startedAt.IsZero() {
				event.StartedAt = startedAt
				event.LatencyMs = event.FinishedAt.Sub(startedAt).Milliseconds()
				event.Generation.LatencyMs = event.LatencyMs
			}
			if modelOutput := model.ConvCallbackOutput(output); modelOutput != nil {
				if modelOutput.TokenUsage != nil {
					event.Generation.PromptTokens = int64(modelOutput.TokenUsage.PromptTokens)
					event.Generation.CompletionTokens = int64(modelOutput.TokenUsage.CompletionTokens)
					event.Generation.ReasoningTokens = int64(modelOutput.TokenUsage.CompletionTokensDetails.ReasoningTokens)
					event.Generation.CachedTokens = int64(modelOutput.TokenUsage.PromptTokenDetails.CachedTokens)
					event.Generation.TotalTokens = int64(modelOutput.TokenUsage.TotalTokens)
				}
				if modelOutput.Message != nil {
					if capture.CaptureOutput {
						event.Generation.FinalOutput = captureText(modelOutput.Message.Content, capture.MaxOutputBytes)
					}
					event.Generation.FinalOutputSummary = TextSummary(modelOutput.Message.Content)
					if modelOutput.Message.ResponseMeta != nil {
						event.Generation.FinishReason = modelOutput.Message.ResponseMeta.FinishReason
					}
				}
			}
			observe(ctx, sink, event)
			return ctx
		}).
		OnErrorFn(func(ctx context.Context, _ *callbacks.RunInfo, err error) context.Context {
			event := cloneEvent(base)
			event.Type = EventTypeGeneration
			event.Status = StatusFailed
			event.FinishedAt = time.Now().UTC()
			if startedAt, ok := ctx.Value(callbackStartedAtKey{}).(time.Time); ok && !startedAt.IsZero() {
				event.StartedAt = startedAt
				event.LatencyMs = event.FinishedAt.Sub(startedAt).Milliseconds()
				event.Generation.LatencyMs = event.LatencyMs
			}
			event.ErrorClass, event.ErrorMessage = ErrorFields(err)
			observe(ctx, sink, event)
			return ctx
		}).
		Build()
}

func observe(ctx context.Context, sink Sink, event Event) {
	if sink == nil {
		return
	}
	_, _ = sink.Observe(ctx, event)
}

func captureText(value string, maxBytes int) string {
	value = RedactPlainText(value)
	if maxBytes <= 0 || len([]byte(value)) <= maxBytes {
		return value
	}
	for len([]byte(value)) > maxBytes {
		value = value[:len(value)-1]
	}
	return value
}
