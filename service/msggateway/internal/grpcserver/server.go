// Package grpcserver implements msggateway's downstream-push gRPC surface
// (03-message-pipeline §6.2). service/push broadcasts BatchPushOneMsg to every
// gateway instance (k8s headless DNS fan-out); each gateway delivers only to the
// WebSocket connections it holds locally and returns per-user status. The push
// service aggregates those statuses across all gateways to decide who is truly
// offline (二段式 toOfflinePush).
package grpcserver

import (
	"context"
	"encoding/json"
	"fmt"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/wujunhui99/agents_im/pkg/gateway/delivery"
	"github.com/wujunhui99/agents_im/service/msggateway/gatewaypb"
)

// PushDeliverer is the local-delivery surface the gRPC server needs
// (real: *ws.Server). It delivers to this instance's own connections only.
type PushDeliverer interface {
	PushToConversation(ctx context.Context, conversationID string, recipientUserIDs []string, event delivery.Event) (delivery.Result, error)
}

// Server adapts the gateway's local delivery to the GatewayService contract.
type Server struct {
	gatewaypb.UnimplementedGatewayServiceServer
	deliverer PushDeliverer
}

func New(deliverer PushDeliverer) *Server {
	return &Server{deliverer: deliverer}
}

func (s *Server) BatchPushOneMsg(ctx context.Context, req *gatewaypb.BatchPushOneMsgReq) (*gatewaypb.BatchPushOneMsgResp, error) {
	if s == nil || s.deliverer == nil {
		return nil, status.Error(codes.Unavailable, "gateway delivery not configured")
	}
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}

	var event delivery.Event
	if len(req.GetEventJson()) > 0 {
		if err := json.Unmarshal(req.GetEventJson(), &event); err != nil {
			return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("decode event_json: %v", err))
		}
	}

	result, err := s.deliverer.PushToConversation(ctx, req.GetConversationId(), req.GetPushToUserIds(), event)
	if err != nil {
		return nil, status.Error(codes.Unavailable, err.Error())
	}

	resp := &gatewaypb.BatchPushOneMsgResp{
		Recipients: make([]*gatewaypb.RecipientDeliveryResult, 0, len(result.Recipients)),
	}
	for _, recipient := range result.Recipients {
		resp.Recipients = append(resp.Recipients, &gatewaypb.RecipientDeliveryResult{
			UserId: recipient.UserID,
			Status: recipient.Status,
			Error:  recipient.Error,
		})
	}
	return resp, nil
}
