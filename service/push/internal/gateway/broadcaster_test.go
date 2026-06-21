package gateway

import (
	"context"
	"errors"
	"sort"
	"testing"

	"google.golang.org/grpc"

	"github.com/wujunhui99/agents_im/pkg/gateway/delivery"
	"github.com/wujunhui99/agents_im/service/msggateway/gatewaypb"
)

type fakeConn struct {
	recipients []*gatewaypb.RecipientDeliveryResult
	err        error
}

func (f fakeConn) BatchPushOneMsg(context.Context, *gatewaypb.BatchPushOneMsgReq, ...grpc.CallOption) (*gatewaypb.BatchPushOneMsgResp, error) {
	if f.err != nil {
		return nil, f.err
	}
	return &gatewaypb.BatchPushOneMsgResp{Recipients: f.recipients}, nil
}

func (fakeConn) Close() error { return nil }

func delivered(userID string) *gatewaypb.RecipientDeliveryResult {
	return &gatewaypb.RecipientDeliveryResult{UserId: userID, Status: delivery.StatusDelivered}
}

func offline(userID string) *gatewaypb.RecipientDeliveryResult {
	return &gatewaypb.RecipientDeliveryResult{UserId: userID, Status: delivery.StatusOffline}
}

func newTestBroadcaster(conns map[string]conn) *Broadcaster {
	return &Broadcaster{conns: conns}
}

func TestBroadcastAggregatesDeliveredAcrossGateways(t *testing.T) {
	// userA delivered on gw1, userB delivered on gw2, userC nowhere → offline.
	b := newTestBroadcaster(map[string]conn{
		"gw1": fakeConn{recipients: []*gatewaypb.RecipientDeliveryResult{delivered("userA"), offline("userB"), offline("userC")}},
		"gw2": fakeConn{recipients: []*gatewaypb.RecipientDeliveryResult{offline("userA"), delivered("userB"), offline("userC")}},
	})

	result, err := b.Broadcast(context.Background(), PushRequest{
		ConversationID: "conv1",
		RecipientIDs:   []string{"userA", "userB", "userC"},
	})
	if err != nil {
		t.Fatalf("broadcast: %v", err)
	}
	if !result.Delivered["userA"] || !result.Delivered["userB"] {
		t.Fatalf("expected userA+userB delivered, got %+v", result.Delivered)
	}
	if result.Delivered["userC"] {
		t.Fatalf("userC should not be delivered")
	}
	sort.Strings(result.OfflineUserIDs)
	if len(result.OfflineUserIDs) != 1 || result.OfflineUserIDs[0] != "userC" {
		t.Fatalf("expected offline=[userC], got %v", result.OfflineUserIDs)
	}
}

func TestBroadcastErrorsWhenAnyGatewayFails(t *testing.T) {
	// A gateway RPC failure must fail the whole call: the user might be on that
	// unreachable instance — declaring false offline would mis-route the push.
	b := newTestBroadcaster(map[string]conn{
		"gw1": fakeConn{recipients: []*gatewaypb.RecipientDeliveryResult{delivered("userA")}},
		"gw2": fakeConn{err: errors.New("gw2 down")},
	})
	if _, err := b.Broadcast(context.Background(), PushRequest{RecipientIDs: []string{"userA"}}); err == nil {
		t.Fatal("expected error when a gateway fails")
	}
}

func TestBroadcastNoConns(t *testing.T) {
	b := newTestBroadcaster(map[string]conn{})
	if _, err := b.Broadcast(context.Background(), PushRequest{RecipientIDs: []string{"userA"}}); err == nil {
		t.Fatal("expected error with no gateway conns")
	}
}

func TestNewBroadcasterTargetParsing(t *testing.T) {
	single, err := NewBroadcaster("msggateway-headless:9100", 0, 0, 0)
	if err != nil {
		t.Fatalf("single target: %v", err)
	}
	if single.host != "msggateway-headless" || single.port != "9100" || len(single.static) != 0 {
		t.Fatalf("unexpected single parse: host=%q port=%q static=%v", single.host, single.port, single.static)
	}

	list, err := NewBroadcaster("10.0.0.1:9100, 10.0.0.2:9100", 0, 0, 0)
	if err != nil {
		t.Fatalf("list target: %v", err)
	}
	if len(list.static) != 2 {
		t.Fatalf("expected 2 static targets, got %v", list.static)
	}

	if _, err := NewBroadcaster("", 0, 0, 0); err == nil {
		t.Fatal("expected error for empty target")
	}
}
