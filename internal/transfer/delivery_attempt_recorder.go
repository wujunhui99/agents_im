package transfer

import (
	"context"
	"strings"

	"github.com/wujunhui99/agents_im/internal/observability"
)

// MetricsDeliveryAttemptRecorder records live-delivery outcomes as metrics only.
// V2 intentionally removed the delivery_attempts table: durable delivery state is
// recovered from messages/outbox plus per-conversation seq pull, not per-user rows.
type MetricsDeliveryAttemptRecorder struct{}

func NewMetricsDeliveryAttemptRecorder() *MetricsDeliveryAttemptRecorder {
	return &MetricsDeliveryAttemptRecorder{}
}

func NewRepositoryDeliveryAttemptRecorder(_ any) *MetricsDeliveryAttemptRecorder {
	return NewMetricsDeliveryAttemptRecorder()
}

func (r *MetricsDeliveryAttemptRecorder) RecordDeliveryResults(ctx context.Context, envelope Envelope, result ProcessResult) error {
	if r == nil || len(result.RecipientResults) == 0 {
		return nil
	}
	for _, recipient := range result.RecipientResults {
		status := strings.TrimSpace(string(recipient.Status))
		if status == "" {
			status = strings.TrimSpace(string(result.Status))
		}
		if status == "" {
			status = "unknown"
		}
		observability.RecordDeliveryAttempt(status)
	}
	return nil
}
