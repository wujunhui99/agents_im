package transfer

import (
	"context"
	"strings"
	"time"

	"github.com/wujunhui99/agents_im/internal/repository"
)

type RepositoryDeliveryAttemptRecorder struct {
	repo repository.DeliveryAttemptRepository
	now  func() time.Time
}

func NewRepositoryDeliveryAttemptRecorder(repo repository.DeliveryAttemptRepository) *RepositoryDeliveryAttemptRecorder {
	return &RepositoryDeliveryAttemptRecorder{
		repo: repo,
		now:  time.Now,
	}
}

func (r *RepositoryDeliveryAttemptRecorder) RecordDeliveryResults(ctx context.Context, envelope Envelope, result ProcessResult) error {
	if r == nil || r.repo == nil || len(result.RecipientResults) == 0 {
		return nil
	}
	now := time.Now
	if r.now != nil {
		now = r.now
	}

	for _, recipient := range result.RecipientResults {
		userID := strings.TrimSpace(recipient.UserID)
		if userID == "" {
			continue
		}
		input := repository.RecordDeliveryAttemptInput{
			ServerMsgID:     envelope.Event.ServerMsgID,
			ConversationID:  envelope.Event.ConversationID,
			RecipientUserID: userID,
			Status:          repositoryDeliveryStatus(recipient.Status),
			AttemptCount:    envelope.Attempt,
			LastError:       firstDeliveryError(recipient.Error, result.Err),
		}
		if input.Status == repository.DeliveryStatusFailed && result.Status == StatusRetryable {
			retryAfter := result.RetryAfter
			if retryAfter > 0 {
				input.NextRetryAt = now().UTC().Add(retryAfter)
			}
		}
		if err := r.repo.RecordDeliveryAttemptResult(ctx, input); err != nil {
			return err
		}
	}
	return nil
}

func repositoryDeliveryStatus(status RecipientDeliveryStatus) string {
	switch status {
	case RecipientDeliveryDelivered:
		return repository.DeliveryStatusDelivered
	case RecipientDeliveryOffline:
		return repository.DeliveryStatusOffline
	default:
		return repository.DeliveryStatusFailed
	}
}

func firstDeliveryError(message string, err error) string {
	if strings.TrimSpace(message) != "" {
		return message
	}
	if err != nil {
		return err.Error()
	}
	return ""
}
