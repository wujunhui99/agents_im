// Package gateway broadcasts downstream pushes to every msggateway instance.
//
// Service discovery is k8s-native (NO etcd): the configured Target is a single
// host:port pointing at msggateway's HEADLESS Service, whose DNS A records list
// every gateway pod IP. The broadcaster resolves those IPs, keeps one gRPC conn
// per pod, and fans BatchPushOneMsg out to all of them concurrently — each
// gateway delivers only to its local WebSocket connections (03 §6.2 / D4).
//
// go-zero's default dns:/// client cannot do this: its load balancer sends each
// RPC to ONE backend, so it can never reach every gateway. Hence the explicit
// one-conn-per-pod pool here. With N=1 (current/dev) it degrades to a single conn.
package gateway

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/wujunhui99/agents_im/pkg/gateway/delivery"
	"github.com/wujunhui99/agents_im/pkg/observability"
	"github.com/wujunhui99/agents_im/service/msggateway/gatewaypb"
)

// dialFunc builds a client for one resolved gateway address (injectable for tests).
type dialFunc func(ctx context.Context, addr string) (conn, error)

// conn is the subset of *grpc.ClientConn the broadcaster needs.
type conn interface {
	gatewaypb.GatewayServiceClient
	Close() error
}

// Broadcaster maintains a conn per gateway instance and fans pushes out.
type Broadcaster struct {
	port        string
	host        string   // headless DNS name (single-target mode)
	static      []string // explicit host:port list (comma-target mode)
	dialTimeout time.Duration
	pushTimeout time.Duration
	refresh     time.Duration
	resolve     func(ctx context.Context, host string) ([]string, error)
	dial        dialFunc

	mu    sync.Mutex
	conns map[string]conn // addr -> conn
}

// Result reports per-recipient online delivery across all gateways.
type Result struct {
	Delivered      map[string]bool
	OfflineUserIDs []string
}

// PushRequest is one message to broadcast.
type PushRequest struct {
	ConversationID string
	RecipientIDs   []string
	Event          delivery.Event
	EventJSON      []byte
}

func NewBroadcaster(target string, dialTimeout, pushTimeout, refresh time.Duration) (*Broadcaster, error) {
	target = strings.TrimSpace(target)
	if target == "" {
		return nil, errors.New("push gateway target is required")
	}
	b := &Broadcaster{
		dialTimeout: orDefault(dialTimeout, 5*time.Second),
		pushTimeout: orDefault(pushTimeout, 5*time.Second),
		refresh:     orDefault(refresh, 30*time.Second),
		resolve:     defaultResolve,
		dial:        defaultDial,
		conns:       map[string]conn{},
	}
	if strings.Contains(target, ",") {
		for _, item := range strings.Split(target, ",") {
			if item = strings.TrimSpace(item); item != "" {
				b.static = append(b.static, item)
			}
		}
		if len(b.static) == 0 {
			return nil, errors.New("push gateway target list is empty")
		}
		return b, nil
	}
	host, port, err := net.SplitHostPort(target)
	if err != nil {
		return nil, fmt.Errorf("push gateway target must be host:port: %w", err)
	}
	b.host = host
	b.port = port
	return b, nil
}

// Start does an initial resolve (fail-loud if no gateway reachable) and then
// re-resolves periodically so new gateway pods join the broadcast set.
func (b *Broadcaster) Start(ctx context.Context) error {
	if err := b.refreshConns(ctx); err != nil {
		return err
	}
	go b.loop(ctx)
	return nil
}

func (b *Broadcaster) loop(ctx context.Context) {
	ticker := time.NewTicker(b.refresh)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := b.refreshConns(ctx); err != nil {
				logx.WithContext(ctx).Errorf("push: refresh gateway conns: %v", err)
			}
		}
	}
}

// targets resolves the current set of gateway addresses.
func (b *Broadcaster) targets(ctx context.Context) ([]string, error) {
	if len(b.static) > 0 {
		return append([]string(nil), b.static...), nil
	}
	ips, err := b.resolve(ctx, b.host)
	if err != nil {
		return nil, fmt.Errorf("resolve gateway host %q: %w", b.host, err)
	}
	addrs := make([]string, 0, len(ips))
	for _, ip := range ips {
		addrs = append(addrs, net.JoinHostPort(ip, b.port))
	}
	sort.Strings(addrs)
	return addrs, nil
}

func (b *Broadcaster) refreshConns(ctx context.Context) error {
	addrs, err := b.targets(ctx)
	if err != nil {
		return err
	}
	if len(addrs) == 0 {
		return fmt.Errorf("no gateway instances resolved for %q", b.host)
	}
	want := make(map[string]struct{}, len(addrs))
	for _, addr := range addrs {
		want[addr] = struct{}{}
	}

	b.mu.Lock()
	defer b.mu.Unlock()
	// Drop conns to vanished pods.
	for addr, c := range b.conns {
		if _, ok := want[addr]; !ok {
			_ = c.Close()
			delete(b.conns, addr)
		}
	}
	// Dial new pods.
	for addr := range want {
		if _, ok := b.conns[addr]; ok {
			continue
		}
		dialCtx, cancel := context.WithTimeout(ctx, b.dialTimeout)
		c, err := b.dial(dialCtx, addr)
		cancel()
		if err != nil {
			logx.WithContext(ctx).Errorf("push: dial gateway %s: %v", addr, err)
			continue
		}
		b.conns[addr] = c
	}
	if len(b.conns) == 0 {
		return fmt.Errorf("no gateway conns established (resolved %d addr)", len(addrs))
	}
	return nil
}

func (b *Broadcaster) snapshot() []conn {
	b.mu.Lock()
	defer b.mu.Unlock()
	conns := make([]conn, 0, len(b.conns))
	for _, c := range b.conns {
		conns = append(conns, c)
	}
	return conns
}

// Broadcast pushes one message to every gateway and aggregates per-user delivery.
// A gateway-level RPC error fails the whole call (returns error): the recipient
// might be connected to that unreachable instance, so the caller must retry the
// batch rather than declare false offline (fail-loud, at-least-once).
func (b *Broadcaster) Broadcast(ctx context.Context, req PushRequest) (Result, error) {
	conns := b.snapshot()
	if len(conns) == 0 {
		return Result{}, errors.New("push: no gateway conns available")
	}
	grpcReq := &gatewaypb.BatchPushOneMsgReq{
		ConversationId: req.ConversationID,
		PushToUserIds:  req.RecipientIDs,
		EventJson:      req.EventJSON,
	}

	type outcome struct {
		recipients []*gatewaypb.RecipientDeliveryResult
		err        error
	}
	results := make([]outcome, len(conns))
	var wg sync.WaitGroup
	for i, c := range conns {
		wg.Add(1)
		go func(i int, c conn) {
			defer wg.Done()
			callCtx, cancel := context.WithTimeout(ctx, b.pushTimeout)
			defer cancel()
			resp, err := c.BatchPushOneMsg(callCtx, grpcReq)
			if err != nil {
				results[i] = outcome{err: err}
				return
			}
			results[i] = outcome{recipients: resp.GetRecipients()}
		}(i, c)
	}
	wg.Wait()

	delivered := make(map[string]bool, len(req.RecipientIDs))
	var firstErr error
	for _, r := range results {
		if r.err != nil {
			if firstErr == nil {
				firstErr = r.err
			}
			continue
		}
		for _, recipient := range r.recipients {
			if recipient.GetStatus() == delivery.StatusDelivered {
				delivered[recipient.GetUserId()] = true
			}
		}
	}
	if firstErr != nil {
		return Result{}, fmt.Errorf("push: gateway broadcast failed: %w", firstErr)
	}

	offline := make([]string, 0)
	for _, userID := range req.RecipientIDs {
		userID = strings.TrimSpace(userID)
		if userID == "" {
			continue
		}
		if !delivered[userID] {
			offline = append(offline, userID)
		}
	}
	return Result{Delivered: delivered, OfflineUserIDs: offline}, nil
}

// Ready reports whether at least one gateway conn is established.
func (b *Broadcaster) Ready(context.Context) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if len(b.conns) == 0 {
		return errors.New("no gateway conns")
	}
	return nil
}

func (b *Broadcaster) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	for addr, c := range b.conns {
		_ = c.Close()
		delete(b.conns, addr)
	}
	return nil
}

func defaultResolve(ctx context.Context, host string) ([]string, error) {
	return net.DefaultResolver.LookupHost(ctx, host)
}

func defaultDial(ctx context.Context, addr string) (conn, error) {
	cc, err := grpc.NewClient(addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithChainUnaryInterceptor(observability.GRPCUnaryClientInterceptor()),
	)
	if err != nil {
		return nil, err
	}
	return grpcConn{ClientConn: cc, GatewayServiceClient: gatewaypb.NewGatewayServiceClient(cc)}, nil
}

type grpcConn struct {
	*grpc.ClientConn
	gatewaypb.GatewayServiceClient
}

func orDefault(d, fallback time.Duration) time.Duration {
	if d <= 0 {
		return fallback
	}
	return d
}
