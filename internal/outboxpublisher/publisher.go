package outboxpublisher

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/wujunhui99/agents_im/internal/messaging"
	"github.com/wujunhui99/agents_im/internal/repository"
)

const (
	DefaultWorkerID = "outbox-kafka-publisher"
	DefaultTopic    = messaging.DefaultMessageEventsTopic

	defaultBatchLimit   = 100
	defaultLockDuration = 30 * time.Second
	defaultPollInterval = 100 * time.Millisecond
	defaultRetryBackoff = time.Second
	defaultMaxAttempts  = 5
	maxLastErrorLength  = 1024
)

type Publisher struct {
	repo         repository.OutboxRepository
	producer     messaging.Producer
	workerID     string
	batchLimit   int
	lockDuration time.Duration
	pollInterval time.Duration
	retryBackoff time.Duration
	maxAttempts  int
	now          func() time.Time
}

type Option func(*Publisher)

type RunOnceResult struct {
	Polled    int
	Published int
	Failed    int
	Errors    []error
}

func (r RunOnceResult) Err() error {
	return errors.Join(r.Errors...)
}

func New(repo repository.OutboxRepository, producer messaging.Producer, opts ...Option) (*Publisher, error) {
	if repo == nil {
		return nil, errors.New("outbox repository is required")
	}
	if producer == nil {
		return nil, errors.New("messaging producer is required")
	}

	p := &Publisher{
		repo:         repo,
		producer:     producer,
		workerID:     DefaultWorkerID,
		batchLimit:   defaultBatchLimit,
		lockDuration: defaultLockDuration,
		pollInterval: defaultPollInterval,
		retryBackoff: defaultRetryBackoff,
		maxAttempts:  defaultMaxAttempts,
		now:          time.Now,
	}
	for _, opt := range opts {
		opt(p)
	}
	p.workerID = strings.TrimSpace(p.workerID)
	if p.workerID == "" {
		p.workerID = DefaultWorkerID
	}
	if p.batchLimit <= 0 {
		p.batchLimit = defaultBatchLimit
	}
	if p.lockDuration <= 0 {
		p.lockDuration = defaultLockDuration
	}
	if p.pollInterval <= 0 {
		p.pollInterval = defaultPollInterval
	}
	if p.retryBackoff < 0 {
		p.retryBackoff = defaultRetryBackoff
	}
	if p.maxAttempts < 0 {
		p.maxAttempts = defaultMaxAttempts
	}
	if p.now == nil {
		p.now = time.Now
	}
	return p, nil
}

func WithWorkerID(workerID string) Option {
	return func(p *Publisher) {
		p.workerID = workerID
	}
}

func WithBatchLimit(limit int) Option {
	return func(p *Publisher) {
		p.batchLimit = limit
	}
}

func WithLockDuration(duration time.Duration) Option {
	return func(p *Publisher) {
		p.lockDuration = duration
	}
}

func WithPollInterval(interval time.Duration) Option {
	return func(p *Publisher) {
		p.pollInterval = interval
	}
}

func WithRetryBackoff(backoff time.Duration) Option {
	return func(p *Publisher) {
		p.retryBackoff = backoff
	}
}

func WithMaxAttempts(maxAttempts int) Option {
	return func(p *Publisher) {
		p.maxAttempts = maxAttempts
	}
}

func WithNow(now func() time.Time) Option {
	return func(p *Publisher) {
		p.now = now
	}
}

func (p *Publisher) Run(ctx context.Context) error {
	for {
		result, err := p.RunOnce(ctx)
		if err != nil {
			if isContextError(err) {
				return nil
			}
			return err
		}
		if result.Polled == 0 {
			if !sleepContext(ctx, p.pollInterval) {
				return nil
			}
		}
	}
}

func (p *Publisher) RunOnce(ctx context.Context) (RunOnceResult, error) {
	var result RunOnceResult
	if err := ctx.Err(); err != nil {
		return result, err
	}

	events, err := p.repo.PollPending(ctx, p.workerID, p.batchLimit, p.lockDuration)
	if err != nil {
		return result, err
	}
	result.Polled = len(events)

	for _, outboxEvent := range events {
		if err := ctx.Err(); err != nil {
			return result, err
		}

		messageEvent, err := MessageEventFromOutbox(outboxEvent)
		if err == nil {
			err = p.producer.Publish(ctx, messageEvent)
		}
		if err != nil {
			if isContextError(err) {
				return result, err
			}
			wrapped := fmt.Errorf("process outbox event %s: %w", outboxEvent.EventID, err)
			result.Errors = append(result.Errors, wrapped)
			if markErr := p.repo.MarkFailed(ctx, outboxEvent.EventID, p.workerID, p.failure(outboxEvent, wrapped)); markErr != nil {
				return result, fmt.Errorf("mark outbox event %s failed: %w", outboxEvent.EventID, markErr)
			}
			result.Failed++
			continue
		}

		if err := p.repo.MarkPublished(ctx, outboxEvent.EventID, p.workerID); err != nil {
			return result, fmt.Errorf("mark outbox event %s published: %w", outboxEvent.EventID, err)
		}
		result.Published++
	}

	return result, nil
}

func (p *Publisher) failure(event repository.OutboxEvent, err error) repository.OutboxFailure {
	lastError := strings.TrimSpace(err.Error())
	if len(lastError) > maxLastErrorLength {
		lastError = lastError[:maxLastErrorLength]
	}

	nextAttemptAt := time.Time{}
	if p.maxAttempts == 0 || event.AttemptCount+1 < p.maxAttempts {
		nextAttemptAt = p.now().UTC().Add(p.retryBackoff)
	}

	return repository.OutboxFailure{
		NextAttemptAt: nextAttemptAt,
		LastError:     lastError,
	}
}

func MessageEventFromOutbox(event repository.OutboxEvent) (messaging.MessageEvent, error) {
	if event.EventType != repository.OutboxEventTypeMessageCreated {
		return messaging.MessageEvent{}, fmt.Errorf("unsupported outbox event_type %q", event.EventType)
	}
	if event.AggregateType != 0 && event.AggregateType != repository.OutboxAggregateTypeMessage {
		return messaging.MessageEvent{}, fmt.Errorf("unsupported outbox aggregate_type %q", event.AggregateType)
	}

	var payload repository.MessageCreatedOutboxPayload
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		return messaging.MessageEvent{}, fmt.Errorf("decode message.created outbox payload: %w", err)
	}

	message := payload.Message
	content, err := messageContent(message)
	if err != nil {
		return messaging.MessageEvent{}, err
	}
	accepted := messaging.MessageEvent{
		EventID:        event.EventID,
		EventType:      messaging.EventTypeMessageAccepted,
		ConversationID: firstNonEmpty(message.ConversationID, event.ConversationID),
		ServerMsgID:    firstNonEmpty(message.ServerMsgID, event.ServerMsgID),
		Seq:            firstNonZero(message.Seq, event.Seq),
		SenderID:       message.SenderID,
		ChatType:       message.ChatType,
		CreatedAt:      messageCreatedAt(message, event),
		Payload: messaging.MessageEventPayload{
			ClientMsgID:           message.ClientMsgID,
			ReceiverID:            message.ReceiverID,
			ReceiverIDs:           receiverIDs(message, payload.VisibleUserIDs),
			GroupID:               message.GroupID,
			ContentType:           message.ContentType,
			Content:               content,
			MessageOrigin:         message.MessageOrigin,
			AgentAccountID:        message.AgentAccountID,
			TriggerServerMsgID:    message.TriggerServerMsgID,
			AgentRunID:            message.AgentRunID,
			AllowRecursiveTrigger: message.AllowRecursiveTrigger,
		},
	}
	if err := accepted.Validate(); err != nil {
		return messaging.MessageEvent{}, fmt.Errorf("build message.accepted event: %w", err)
	}
	return accepted, nil
}

func messageContent(message repository.Message) (json.RawMessage, error) {
	switch message.ContentType {
	case "", repository.ContentTypeText:
		content, err := json.Marshal(map[string]string{"text": message.Content})
		if err != nil {
			return nil, fmt.Errorf("encode text message content: %w", err)
		}
		return json.RawMessage(content), nil
	default:
		content := strings.TrimSpace(message.Content)
		if content == "" {
			return nil, nil
		}
		if json.Valid([]byte(content)) {
			return json.RawMessage(append([]byte(nil), content...)), nil
		}
		encoded, err := json.Marshal(message.Content)
		if err != nil {
			return nil, fmt.Errorf("encode message content: %w", err)
		}
		return json.RawMessage(encoded), nil
	}
}

func receiverIDs(message repository.Message, visibleUserIDs []string) []string {
	ids := make([]string, 0, len(visibleUserIDs)+1)
	if message.ChatType == repository.ChatTypeSingle && message.ReceiverID != "" {
		ids = append(ids, message.ReceiverID)
	}
	includeSender := shouldDeliverMessageToSender(message)
	if includeSender {
		ids = append(ids, message.SenderID)
	}
	for _, userID := range visibleUserIDs {
		if userID != "" && (includeSender || userID != message.SenderID) {
			ids = append(ids, userID)
		}
	}
	return uniqueSorted(ids)
}

func shouldDeliverMessageToSender(message repository.Message) bool {
	return strings.ToLower(strings.TrimSpace(message.ChatType)) == repository.ChatTypeSingle &&
		strings.ToLower(strings.TrimSpace(message.MessageOrigin)) == repository.MessageOriginAI
}

func uniqueSorted(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	sort.Strings(values)
	unique := values[:0]
	var previous string
	for _, value := range values {
		if value == "" || value == previous {
			continue
		}
		unique = append(unique, value)
		previous = value
	}
	return append([]string(nil), unique...)
}

func messageCreatedAt(message repository.Message, event repository.OutboxEvent) int64 {
	if message.CreatedAt > 0 {
		return message.CreatedAt
	}
	if message.SendTime > 0 {
		return message.SendTime
	}
	if !event.CreatedAt.IsZero() {
		return event.CreatedAt.UTC().UnixMilli()
	}
	return 0
}

func firstNonEmpty(first string, second string) string {
	if first != "" {
		return first
	}
	return second
}

func firstNonZero(first int64, second int64) int64 {
	if first != 0 {
		return first
	}
	return second
}

func isContextError(err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}

func sleepContext(ctx context.Context, duration time.Duration) bool {
	if duration <= 0 {
		return ctx.Err() == nil
	}
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
