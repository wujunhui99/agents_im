package ws

import (
	"context"

	"github.com/wujunhui99/agents_im/pkg/gateway"
)

// MessageBackend 是 ws command 的消息域后端（03 §9 A3）：gateway 不再 in-process
// 调 monolith MessageLogic，4 个 command 经该接口转发 msg-rpc gRPC。
// 请求/响应直接使用 common/share/gateway 的 RPC 映射类型（contract.go 单一事实源）。
// send 后 gateway 不做本地 fanout——message_received 推送（含 sender 自己的 seq 回填）
// 由 msgtransfer 经 /internal/delivery/conversation 下发。
type MessageBackend interface {
	SendMessage(ctx context.Context, req gateway.SendMessageRPCRequest) (gateway.SendMessageRPCResponse, error)
	PullMessages(ctx context.Context, req gateway.PullMessagesRPCRequest) (gateway.PullMessagesRPCResponse, error)
	GetConversationSeqs(ctx context.Context, req gateway.GetConversationSeqsRPCRequest) (gateway.GetConversationSeqsRPCResponse, error)
	MarkConversationAsRead(ctx context.Context, req gateway.MarkConversationAsReadRPCRequest) (gateway.MarkConversationAsReadRPCResponse, error)
}
