package svc

import (
	"context"
	"sync"

	business "github.com/wujunhui99/agents_im/internal/logic"
	"github.com/wujunhui99/agents_im/pkg/apperror"
	"github.com/wujunhui99/agents_im/pkg/messaging"
)

// AgentSendFunc 把 AI 托管的写回桥接到 msg-rpc 自身 SendMessage（Kafka 模式下
// AI 回复与用户消息走同一条 Kafka 链路，防止 PG/Redis 双 seq 分裂）。
type AgentSendFunc func(ctx context.Context, req business.SendMessageRequest) (business.SendMessageResponse, error)

// kafkaModeSender 是晚绑定的 agentim.MessageSender：svc 构造期不能引用 logic 包
// （logic→svc 单向依赖），实现由 msg.go 启动时经 BindAgentResponseSender 注入。
type kafkaModeSender struct {
	mu sync.RWMutex
	fn AgentSendFunc
}

func (s *kafkaModeSender) bind(fn AgentSendFunc) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.fn = fn
}

func (s *kafkaModeSender) SendMessage(ctx context.Context, req business.SendMessageRequest) (business.SendMessageResponse, error) {
	s.mu.RLock()
	fn := s.fn
	s.mu.RUnlock()
	if fn == nil {
		return business.SendMessageResponse{}, apperror.Internal("agent response sender is not bound")
	}
	return fn(ctx, req)
}

// BindAgentResponseSender 注入 AI 写回实现（Kafka 唯一写链路，03 §9 B3b）。
func (s *ServiceContext) BindAgentResponseSender(fn AgentSendFunc) {
	if s.agentSender != nil {
		s.agentSender.bind(fn)
	}
}

// EventPublisher 是 SendMessage Kafka 写路径需要的最小 producer 面
// （生产实现 messaging.KafkaProducer；测试注入 fake）。
type EventPublisher interface {
	PublishEvent(ctx context.Context, topic string, event messaging.MessageEvent) error
}
