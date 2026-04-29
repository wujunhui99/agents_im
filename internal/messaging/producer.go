package messaging

import (
	"context"
	"errors"
	"sync"
)

var ErrProducerClosed = errors.New("messaging producer is closed")

type Producer interface {
	Publish(ctx context.Context, event MessageEvent) error
	Close() error
}

type NoopProducer struct{}

func NewNoopProducer() *NoopProducer {
	return &NoopProducer{}
}

func (p *NoopProducer) Publish(ctx context.Context, event MessageEvent) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return event.Validate()
}

func (p *NoopProducer) Close() error {
	return nil
}

type InMemoryProducer struct {
	mu     sync.RWMutex
	events []MessageEvent
	closed bool
}

func NewInMemoryProducer() *InMemoryProducer {
	return &InMemoryProducer{}
}

func (p *InMemoryProducer) Publish(ctx context.Context, event MessageEvent) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := event.Validate(); err != nil {
		return err
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return ErrProducerClosed
	}
	p.events = append(p.events, event.Clone())
	return nil
}

func (p *InMemoryProducer) Events() []MessageEvent {
	p.mu.RLock()
	defer p.mu.RUnlock()

	events := make([]MessageEvent, 0, len(p.events))
	for _, event := range p.events {
		events = append(events, event.Clone())
	}
	return events
}

func (p *InMemoryProducer) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.closed = true
	return nil
}
