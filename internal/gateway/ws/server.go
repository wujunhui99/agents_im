package ws

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/wujunhui99/agents_im/internal/apperror"
	authrepo "github.com/wujunhui99/agents_im/internal/auth/repository"
	"github.com/wujunhui99/agents_im/internal/auth/token"
	"github.com/wujunhui99/agents_im/internal/config"
	"github.com/wujunhui99/agents_im/internal/gateway"
	"github.com/wujunhui99/agents_im/internal/gateway/delivery"
	"github.com/wujunhui99/agents_im/internal/logic"
	"github.com/wujunhui99/agents_im/internal/observability"
	"github.com/wujunhui99/agents_im/internal/presence"
	gatewaysvc "github.com/wujunhui99/agents_im/internal/servicecontext/gateway"
)

const CommandHeartbeat = gateway.CommandHeartbeat

const (
	CloseCodeSessionReplaced = 4001
	CloseCodeSessionInvalid  = 4002
	defaultReadLimit         = 64 * 1024
	defaultWriteWait         = 10 * time.Second
)

type Server struct {
	auth           config.JWTAuthConfig
	tokenManager   token.Manager
	activeSessions authrepo.ActiveSessionRepository
	messageLogic   *logic.MessageLogic
	connections    *ConnectionManager
	dispatcher     delivery.Dispatcher
	presence       presence.PresenceStore
	presenceTTL    time.Duration
	instanceID     string
	wsConfig       config.GatewayWSConfig
	configErr      error
	origins        map[string]struct{}
	upgrader       websocket.Upgrader
	now            func() time.Time
}

type ServerOption func(*Server)

type Connection struct {
	ID          string
	UserID      string
	InstanceID  string
	RemoteAddr  string
	UserAgent   string
	DeviceID    string
	Platform    string
	SessionID   string
	ConnectedAt time.Time
	TraceID     string
	RequestID   string

	ws        *websocket.Conn
	writeMu   sync.Mutex
	lastSeen  time.Time
	lastSeenM sync.RWMutex
	limiter   *commandRateLimiter
}

type commandFrame struct {
	RequestID      string          `json:"request_id,omitempty"`
	RequestIDCamel string          `json:"requestId,omitempty"`
	TraceID        string          `json:"trace_id,omitempty"`
	TraceIDCamel   string          `json:"traceId,omitempty"`
	Type           string          `json:"type,omitempty"`
	Command        string          `json:"command,omitempty"`
	Payload        json.RawMessage `json:"payload,omitempty"`
}

type responseFrame struct {
	RequestID      string         `json:"request_id,omitempty"`
	RequestIDCamel string         `json:"requestId,omitempty"`
	TraceID        string         `json:"trace_id,omitempty"`
	TraceIDCamel   string         `json:"traceId,omitempty"`
	Type           string         `json:"type,omitempty"`
	Command        string         `json:"command,omitempty"`
	Status         string         `json:"status"`
	Error          *responseError `json:"error,omitempty"`
	Data           interface{}    `json:"data,omitempty"`
	Payload        interface{}    `json:"payload,omitempty"`
}

type responseError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type heartbeatData struct {
	ConnectionID string `json:"connection_id"`
	UserID       string `json:"user_id"`
	InstanceID   string `json:"instance_id,omitempty"`
	ServerTime   int64  `json:"server_time"`
}

func NewServer(serviceContext *gatewaysvc.ServiceContext, opts ...ServerOption) *Server {
	auth := config.DefaultJWTAuthConfig()
	var messageLogic *logic.MessageLogic
	if serviceContext != nil {
		auth = serviceContext.Auth
		messageLogic = serviceContext.MessageLogic
	}
	auth = normalizeAuth(auth)
	wsConfig, wsConfigErr := config.ResolveGatewayWSConfig(config.GatewayWSConfig{})

	server := &Server{
		auth:         auth,
		tokenManager: token.NewHMACTokenManager(auth.AccessSecret, time.Duration(auth.AccessExpire)*time.Second),
		messageLogic: messageLogic,
		connections:  NewConnectionManager(),
		presence:     presence.NewMemoryStore(),
		presenceTTL:  presence.HeartbeatTTL(config.DefaultPresenceConfig()),
		instanceID:   defaultGatewayInstanceID(),
		wsConfig:     wsConfig,
		configErr:    wsConfigErr,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
		},
		now: time.Now,
	}
	for _, opt := range opts {
		opt(server)
	}
	server.configureWebSocket()
	return server
}

func WithConnectionManager(manager *ConnectionManager) ServerOption {
	return func(s *Server) {
		if manager != nil {
			s.connections = manager
		}
	}
}

func WithTokenManager(manager token.Manager) ServerOption {
	return func(s *Server) {
		if manager != nil {
			s.tokenManager = manager
		}
	}
}

func WithActiveSessionRepository(repo authrepo.ActiveSessionRepository) ServerOption {
	return func(s *Server) {
		s.activeSessions = repo
	}
}

func WithNow(now func() time.Time) ServerOption {
	return func(s *Server) {
		if now != nil {
			s.now = now
		}
	}
}

func WithPresenceStore(store presence.PresenceStore) ServerOption {
	return func(s *Server) {
		s.presence = store
	}
}

func WithDeliveryDispatcher(dispatcher delivery.Dispatcher) ServerOption {
	return func(s *Server) {
		s.dispatcher = dispatcher
	}
}

func WithPresenceTTL(ttl time.Duration) ServerOption {
	return func(s *Server) {
		if ttl > 0 {
			s.presenceTTL = ttl
		}
	}
}

func WithInstanceID(instanceID string) ServerOption {
	return func(s *Server) {
		if strings.TrimSpace(instanceID) != "" {
			s.instanceID = strings.TrimSpace(instanceID)
		}
	}
}

func WithGatewayWSConfig(wsConfig config.GatewayWSConfig) ServerOption {
	return func(s *Server) {
		s.wsConfig = wsConfig
		s.configErr = nil
	}
}

func (s *Server) configureWebSocket() {
	resolved, err := config.ResolveGatewayWSConfig(s.wsConfig)
	if err != nil {
		s.configErr = err
		s.origins = nil
	} else {
		s.wsConfig = resolved
		s.configErr = nil
		s.origins = originSet(resolved.AllowedOrigins)
	}
	s.upgrader.CheckOrigin = s.checkOrigin
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.HandleWebSocket(w, r)
}

func (s *Server) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	tracedRequest, traceContext := observability.EnsureHTTPTrace(r)
	r = tracedRequest
	ctx, span := observability.StartSpan(r.Context(), "websocket.handshake")
	defer span.End()
	r = r.WithContext(ctx)
	traceContext = observability.TraceContextFromContext(ctx)
	observability.InjectTraceHeaders(w, traceContext)

	if s.configErr != nil {
		log.Printf("websocket_handshake_failed trace_id=%s request_id=%s status=config_error error=%v", traceContext.TraceID, traceContext.RequestID, s.configErr)
		http.Error(w, http.StatusText(http.StatusServiceUnavailable), http.StatusServiceUnavailable)
		return
	}
	claims, err := s.authenticate(r)
	if err != nil {
		log.Printf("websocket_handshake_failed trace_id=%s request_id=%s status=unauthorized remote_addr=%s", traceContext.TraceID, traceContext.RequestID, r.RemoteAddr)
		http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		return
	}
	if s.messageLogic == nil {
		log.Printf("websocket_handshake_failed trace_id=%s request_id=%s user_id=%s status=not_ready", traceContext.TraceID, traceContext.RequestID, claims.UserID)
		http.Error(w, http.StatusText(http.StatusServiceUnavailable), http.StatusServiceUnavailable)
		return
	}

	socket, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}

	now := s.now().UTC()
	conn := &Connection{
		ID:          newConnectionID(),
		UserID:      claims.UserID,
		InstanceID:  s.instanceID,
		RemoteAddr:  r.RemoteAddr,
		UserAgent:   r.UserAgent(),
		DeviceID:    strings.TrimSpace(r.Header.Get("X-Device-ID")),
		Platform:    clientPlatform(r),
		SessionID:   claims.SessionID,
		ConnectedAt: now,
		TraceID:     traceContext.TraceID,
		RequestID:   traceContext.RequestID,
		ws:          socket,
		lastSeen:    now,
		limiter:     newCommandRateLimiter(s.wsConfig, s.now),
	}
	_ = s.registerPresence(r.Context(), conn)
	replaced := s.connections.Register(conn)
	for _, previous := range replaced {
		_ = previous.closeWithCode(CloseCodeSessionReplaced, "session replaced")
	}
	observability.RecordWebSocketConnectionEvent("opened")
	observability.SetWebSocketConnections(s.connections.Count())
	log.Printf("websocket_connected trace_id=%s request_id=%s connection_id=%s user_id=%s instance_id=%s", conn.TraceID, conn.RequestID, conn.ID, conn.UserID, conn.InstanceID)
	defer func() {
		s.connections.Unregister(conn.ID)
		observability.RecordWebSocketConnectionEvent("closed")
		observability.SetWebSocketConnections(s.connections.Count())
		_ = s.unregisterPresence(conn)
		_ = socket.Close()
		log.Printf("websocket_disconnected trace_id=%s request_id=%s connection_id=%s user_id=%s instance_id=%s", conn.TraceID, conn.RequestID, conn.ID, conn.UserID, conn.InstanceID)
	}()

	connCtx, cancel := context.WithCancel(r.Context())
	defer cancel()
	go s.pingLoop(connCtx, conn)
	s.readLoop(connCtx, conn)
}

func (s *Server) Connections() *ConnectionManager {
	return s.connections
}

func (s *Server) DeliveryDispatcher() delivery.Dispatcher {
	if s.dispatcher != nil {
		return s.dispatcher
	}
	return NewPresenceAwareDeliveryDispatcher(s.connections, s.presence, s.instanceID)
}

func (s *Server) PushToUser(ctx context.Context, userID string, event delivery.Event) (delivery.Result, error) {
	return s.DeliveryDispatcher().DeliverToUser(ctx, userID, event)
}

func (s *Server) PushToConversation(ctx context.Context, conversationID string, recipientUserIDs []string, event delivery.Event) (delivery.Result, error) {
	return s.DeliveryDispatcher().DeliverToConversation(ctx, conversationID, recipientUserIDs, event)
}

func (s *Server) HandleInternalConversationDelivery(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}
	var request struct {
		ConversationID   string         `json:"conversation_id"`
		RecipientUserIDs []string       `json:"recipient_user_ids"`
		Event            delivery.Event `json:"event"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, defaultReadLimit))
	if err := decoder.Decode(&request); err != nil {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}
	result, err := s.PushToConversation(r.Context(), request.ConversationID, request.RecipientUserIDs, request.Event)
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(result)
}

func (s *Server) HandleInternalUserPresence(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}
	userID := strings.TrimSpace(r.URL.Query().Get("user_id"))
	if userID == "" {
		http.Error(w, "user_id is required", http.StatusBadRequest)
		return
	}
	online, err := s.IsUserOnline(r.Context(), userID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(struct {
		UserID string `json:"user_id"`
		Online bool   `json:"online"`
	}{
		UserID: userID,
		Online: online,
	})
}

func (s *Server) IsUserOnline(ctx context.Context, userID string) (bool, error) {
	if s.presence == nil {
		return false, errors.New("presence store is not configured")
	}
	return s.presence.IsUserOnline(ctx, userID)
}

func (s *Server) registerPresence(ctx context.Context, conn *Connection) error {
	if s.presence == nil || conn == nil {
		return nil
	}
	return s.presence.RegisterConnection(ctx, conn.presenceMetadata(conn.LastSeen()), s.presenceTTL)
}

func (s *Server) refreshPresence(ctx context.Context, conn *Connection) error {
	if s.presence == nil || conn == nil {
		return nil
	}
	if err := s.presence.Heartbeat(ctx, conn.UserID, conn.ID, s.presenceTTL); err != nil {
		if errors.Is(err, presence.ErrConnectionNotFound) {
			return s.registerPresence(ctx, conn)
		}
		return err
	}
	return nil
}

func (s *Server) unregisterPresence(conn *Connection) error {
	if s.presence == nil || conn == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	return s.presence.UnregisterConnection(ctx, conn.UserID, conn.ID)
}

func (s *Server) authenticate(r *http.Request) (token.Claims, error) {
	rawToken := bearerToken(r.Header.Get("Authorization"))
	if rawToken == "" && s.wsConfig.AllowQueryToken {
		rawToken = strings.TrimSpace(r.URL.Query().Get("token"))
	}
	if rawToken == "" {
		return token.Claims{}, apperror.Unauthenticated("token is required")
	}
	claims, err := s.tokenManager.Validate(rawToken)
	if err != nil {
		return token.Claims{}, err
	}
	if strings.TrimSpace(claims.UserID) == "" {
		return token.Claims{}, apperror.Unauthenticated("authenticated user is required")
	}
	if err := s.validateActiveSession(r.Context(), claims); err != nil {
		return token.Claims{}, err
	}
	return claims, nil
}

func (s *Server) validateActiveSession(ctx context.Context, claims token.Claims) error {
	if s.activeSessions == nil {
		return nil
	}
	return authrepo.ValidateActiveSession(ctx, s.activeSessions, claims)
}

func (s *Server) readLoop(ctx context.Context, conn *Connection) {
	conn.ws.SetReadLimit(defaultReadLimit)
	_ = conn.ws.SetReadDeadline(s.now().Add(s.heartbeatTimeout()))
	conn.ws.SetPongHandler(func(string) error {
		conn.touch(s.now())
		_ = s.refreshPresence(ctx, conn)
		return conn.ws.SetReadDeadline(s.now().Add(s.heartbeatTimeout()))
	})

	for {
		messageType, raw, err := conn.ws.ReadMessage()
		if err != nil {
			return
		}
		conn.touch(s.now())
		if messageType != websocket.TextMessage && messageType != websocket.BinaryMessage {
			continue
		}
		if err := s.validateActiveSession(ctx, token.Claims{UserID: conn.UserID, SessionID: conn.SessionID}); err != nil {
			_ = conn.closeWithCode(CloseCodeSessionInvalid, "session invalid")
			return
		}

		resp := s.handleCommand(ctx, conn, raw)
		if err := conn.writeJSON(resp); err != nil {
			return
		}
	}
}

func (s *Server) pingLoop(ctx context.Context, conn *Connection) {
	interval := s.pingInterval()
	if interval <= 0 || conn == nil || conn.ws == nil {
		return
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			deadline := time.Now().Add(defaultWriteWait)
			conn.writeMu.Lock()
			err := conn.ws.WriteControl(websocket.PingMessage, nil, deadline)
			conn.writeMu.Unlock()
			if err != nil {
				_ = conn.ws.Close()
				return
			}
		}
	}
}

func (s *Server) handleCommand(ctx context.Context, conn *Connection, raw []byte) responseFrame {
	traceContext := conn.traceContext()
	frame, err := decodeCommandFrame(raw)
	if err != nil {
		resp := errorResponse("", "", apperror.InvalidArgument("command envelope is invalid"))
		resp.TraceID = traceContext.TraceID
		logWebSocketCommand(conn, traceContext, resp)
		return resp
	}
	requestID := frame.requestID()
	commandType := frame.commandType()
	if frameTraceID := frame.traceID(); frameTraceID != "" {
		traceContext = observability.NewTraceContext(frameTraceID, traceContext.RequestID)
	}
	ctx = observability.ContextWithTrace(ctx, traceContext)
	ctx, span := observability.StartSpan(ctx, "websocket.command."+commandType)
	defer span.End()
	if conn != nil && conn.limiter != nil && !conn.limiter.Allow() {
		resp := errorResponse(requestID, commandType, apperror.RateLimited("command rate limit exceeded"))
		resp.TraceID = traceContext.TraceID
		logWebSocketCommand(conn, traceContext, resp)
		return resp
	}
	if strings.TrimSpace(requestID) == "" {
		resp := errorResponse("", commandType, apperror.InvalidArgument("request_id is required"))
		resp.TraceID = traceContext.TraceID
		logWebSocketCommand(conn, traceContext, resp)
		return resp
	}
	if strings.TrimSpace(commandType) == "" {
		resp := errorResponse(requestID, "", apperror.InvalidArgument("type is required"))
		resp.TraceID = traceContext.TraceID
		logWebSocketCommand(conn, traceContext, resp)
		return resp
	}

	data, err := s.dispatch(ctx, conn, commandType, frame.Payload)
	if err != nil {
		resp := errorResponse(requestID, commandType, err)
		resp.TraceID = traceContext.TraceID
		logWebSocketCommand(conn, traceContext, resp)
		return resp
	}
	resp := responseFrame{
		RequestID:      requestID,
		RequestIDCamel: requestID,
		TraceID:        traceContext.TraceID,
		TraceIDCamel:   traceContext.TraceID,
		Type:           commandType,
		Command:        commandType,
		Status:         gateway.AckStatusOK,
		Data:           data,
		Payload:        data,
	}
	logWebSocketCommand(conn, traceContext, resp)
	return resp
}

func (s *Server) dispatch(ctx context.Context, conn *Connection, commandType string, payload json.RawMessage) (interface{}, error) {
	switch commandType {
	case CommandHeartbeat:
		_ = s.refreshPresence(ctx, conn)
		return heartbeatData{
			ConnectionID: conn.ID,
			UserID:       conn.UserID,
			InstanceID:   conn.InstanceID,
			ServerTime:   s.now().UTC().UnixMilli(),
		}, nil
	case gateway.CommandSendMessage:
		var req gateway.SendMessageCommandRequest
		if err := unmarshalPayload(payload, &req); err != nil {
			return nil, err
		}
		mapped := gateway.MapSendMessageRequest(conn.UserID, req)
		result, err := s.messageLogic.SendMessage(ctx, logic.SendMessageRequest{
			SenderID:    mapped.SenderID,
			ReceiverID:  mapped.ReceiverID,
			GroupID:     mapped.GroupID,
			ChatType:    mapped.ChatType,
			ClientMsgID: mapped.ClientMsgID,
			ContentType: mapped.ContentType,
			Content:     mapped.Content,
		})
		if err != nil {
			return nil, err
		}
		if !result.Deduplicated {
			s.pushNewMessage(ctx, conn.UserID, result.Message, result.RecipientUserIDs)
		}
		return gateway.MapSendMessageResponse(gateway.SendMessageRPCResponse{
			Message:      toGatewayMessage(result.Message),
			Deduplicated: result.Deduplicated,
		}), nil
	case gateway.CommandPullMessages:
		var req gateway.PullMessagesCommandRequest
		if err := unmarshalPayload(payload, &req); err != nil {
			return nil, err
		}
		mapped := gateway.MapPullMessagesRequest(conn.UserID, req)
		result, err := s.messageLogic.PullMessages(ctx, logic.PullMessagesRequest{
			UserID:         mapped.UserID,
			ConversationID: mapped.ConversationID,
			FromSeq:        mapped.FromSeq,
			ToSeq:          mapped.ToSeq,
			Limit:          int(mapped.Limit),
			Order:          mapped.Order,
		})
		if err != nil {
			return nil, err
		}
		messages := make([]gateway.MessageSnapshot, 0, len(result.Messages))
		for _, message := range result.Messages {
			messages = append(messages, toGatewayMessage(message))
		}
		return gateway.MapPullMessagesResponse(gateway.PullMessagesRPCResponse{
			Messages: messages,
			IsEnd:    result.IsEnd,
			NextSeq:  result.NextSeq,
		}), nil
	case gateway.CommandGetConversationSeqs:
		var req gateway.GetConversationSeqsCommandRequest
		if err := unmarshalPayload(payload, &req); err != nil {
			return nil, err
		}
		mapped := gateway.MapGetConversationSeqsRequest(conn.UserID, req)
		result, err := s.messageLogic.GetConversationSeqs(ctx, logic.GetConversationSeqsRequest{
			UserID:          mapped.UserID,
			ConversationIDs: mapped.ConversationIDs,
		})
		if err != nil {
			return nil, err
		}
		states := make([]gateway.ConversationSeqState, 0, len(result.States))
		for _, state := range result.States {
			states = append(states, toGatewayConversationState(state))
		}
		return gateway.MapGetConversationSeqsResponse(gateway.GetConversationSeqsRPCResponse{States: states}), nil
	case gateway.CommandMarkConversationRead:
		var req gateway.MarkConversationReadCommandRequest
		if err := unmarshalPayload(payload, &req); err != nil {
			return nil, err
		}
		mapped := gateway.MapMarkConversationReadRequest(conn.UserID, req)
		result, err := s.messageLogic.MarkConversationAsRead(ctx, logic.MarkConversationAsReadRequest{
			UserID:         mapped.UserID,
			ConversationID: mapped.ConversationID,
			HasReadSeq:     mapped.HasReadSeq,
		})
		if err != nil {
			return nil, err
		}
		return gateway.MapMarkConversationReadResponse(gateway.MarkConversationAsReadRPCResponse{
			ConversationID: result.ConversationID,
			HasReadSeq:     result.HasReadSeq,
			MaxSeq:         result.MaxSeq,
			UnreadCount:    result.UnreadCount,
			Updated:        result.Updated,
		}), nil
	default:
		return nil, apperror.InvalidArgument("unsupported command type")
	}
}

func (s *Server) pushNewMessage(ctx context.Context, senderID string, message logic.Message, recipientUserIDs []string) {
	ctx, span := observability.StartSpan(ctx, "websocket.push_new_message")
	defer span.End()
	recipients := pushRecipients(senderID, message, recipientUserIDs)
	if len(recipients) == 0 {
		return
	}
	deliveryMessage := toDeliveryMessage(message)
	applyTraceContextToDeliveryMessage(ctx, &deliveryMessage)
	_, err := s.PushToConversation(ctx, message.ConversationID, recipients, delivery.NewMessageEvent(delivery.EventMessageReceived, deliveryMessage))
	if err != nil {
		observability.RecordSpanError(span, err)
		traceContext := observability.TraceContextFromContext(ctx)
		log.Printf("websocket_push_failed trace_id=%s conversation_id=%s server_msg_id=%s error=%v", traceContext.TraceID, message.ConversationID, message.ServerMsgID, err)
	}
}

func pushRecipients(senderID string, message logic.Message, recipientUserIDs []string) []string {
	senderID = strings.TrimSpace(senderID)
	seen := map[string]struct{}{}
	recipients := make([]string, 0, len(recipientUserIDs))
	add := func(userID string) {
		userID = strings.TrimSpace(userID)
		if userID == "" || userID == senderID {
			return
		}
		if _, ok := seen[userID]; ok {
			return
		}
		seen[userID] = struct{}{}
		recipients = append(recipients, userID)
	}

	for _, userID := range recipientUserIDs {
		add(userID)
	}
	if len(recipientUserIDs) > 0 {
		return recipients
	}

	switch strings.ToLower(strings.TrimSpace(message.ChatType)) {
	case logic.MessageChatTypeSingle:
		add(message.ReceiverID)
	}
	return recipients
}

func (c *Connection) Info() ConnectionInfo {
	return ConnectionInfo{
		ConnectionID: c.ID,
		UserID:       c.UserID,
		InstanceID:   c.InstanceID,
		ConnectedAt:  c.ConnectedAt,
		LastSeenAt:   c.LastSeen(),
	}
}

func (c *Connection) presenceMetadata(lastHeartbeat time.Time) presence.ConnectionMetadata {
	return presence.ConnectionMetadata{
		UserID:          c.UserID,
		ConnectionID:    c.ID,
		InstanceID:      c.InstanceID,
		GatewayID:       c.InstanceID,
		DeviceID:        c.DeviceID,
		Platform:        c.Platform,
		RemoteAddr:      c.RemoteAddr,
		UserAgent:       c.UserAgent,
		ConnectedAt:     c.ConnectedAt,
		LastHeartbeatAt: lastHeartbeat.UTC(),
	}
}

func (c *Connection) LastSeen() time.Time {
	c.lastSeenM.RLock()
	defer c.lastSeenM.RUnlock()

	return c.lastSeen
}

func (c *Connection) touch(now time.Time) {
	c.lastSeenM.Lock()
	c.lastSeen = now.UTC()
	c.lastSeenM.Unlock()
}

func (c *Connection) writeJSON(value interface{}) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()

	_ = c.ws.SetWriteDeadline(time.Now().Add(defaultWriteWait))
	return c.ws.WriteJSON(value)
}

func (c *Connection) closeWithCode(code int, reason string) error {
	if c == nil || c.ws == nil {
		return nil
	}
	c.writeMu.Lock()
	_ = c.ws.SetWriteDeadline(time.Now().Add(defaultWriteWait))
	err := c.ws.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(code, reason), time.Now().Add(defaultWriteWait))
	c.writeMu.Unlock()
	_ = c.ws.Close()
	return err
}

func decodeCommandFrame(raw []byte) (commandFrame, error) {
	var frame commandFrame
	if err := json.Unmarshal(raw, &frame); err != nil {
		return commandFrame{}, err
	}
	return frame, nil
}

func (f commandFrame) requestID() string {
	if strings.TrimSpace(f.RequestID) != "" {
		return strings.TrimSpace(f.RequestID)
	}
	return strings.TrimSpace(f.RequestIDCamel)
}

func (f commandFrame) commandType() string {
	if strings.TrimSpace(f.Type) != "" {
		return strings.TrimSpace(f.Type)
	}
	return strings.TrimSpace(f.Command)
}

func (f commandFrame) traceID() string {
	if strings.TrimSpace(f.TraceID) != "" {
		return strings.TrimSpace(f.TraceID)
	}
	return strings.TrimSpace(f.TraceIDCamel)
}

func errorResponse(requestID string, commandType string, err error) responseFrame {
	appErr := apperror.From(err)
	return responseFrame{
		RequestID:      requestID,
		RequestIDCamel: requestID,
		Type:           commandType,
		Command:        commandType,
		Status:         gateway.AckStatusError,
		Error: &responseError{
			Code:    frontendErrorCode(appErr),
			Message: appErr.Message,
		},
	}
}

func frontendErrorCode(appErr *apperror.Error) string {
	if appErr == nil {
		return "INTERNAL"
	}
	switch appErr.Code {
	case apperror.CodeUnauthenticated:
		return "UNAUTHORIZED"
	case apperror.CodeInvalidArgument:
		return "VALIDATION_ERROR"
	case apperror.CodeNotFound:
		return "NOT_FOUND"
	case apperror.CodeAlreadyExists:
		return "CONFLICT"
	case apperror.CodeRateLimited:
		return "RATE_LIMITED"
	default:
		return "INTERNAL"
	}
}

func (s *Server) checkOrigin(r *http.Request) bool {
	origin := strings.TrimSpace(r.Header.Get("Origin"))
	if origin == "" {
		return true
	}
	normalized, ok := normalizeRequestOrigin(origin)
	if !ok {
		return false
	}
	if len(s.origins) > 0 {
		_, allowed := s.origins[normalized]
		return allowed
	}
	return normalized == sameRequestOrigin(r)
}

func originSet(origins []string) map[string]struct{} {
	if len(origins) == 0 {
		return nil
	}
	set := make(map[string]struct{}, len(origins))
	for _, origin := range origins {
		origin = strings.TrimSpace(origin)
		if origin != "" {
			set[origin] = struct{}{}
		}
	}
	return set
}

func normalizeRequestOrigin(origin string) (string, bool) {
	parsed, err := url.Parse(strings.TrimSpace(origin))
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || strings.TrimSpace(parsed.Host) == "" {
		return "", false
	}
	if parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || (parsed.Path != "" && parsed.Path != "/") {
		return "", false
	}
	return strings.ToLower(parsed.Scheme) + "://" + strings.ToLower(parsed.Host), true
}

func sameRequestOrigin(r *http.Request) string {
	if r == nil {
		return ""
	}
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	if forwarded := strings.TrimSpace(r.Header.Get("X-Forwarded-Proto")); forwarded != "" {
		if idx := strings.Index(forwarded, ","); idx >= 0 {
			forwarded = forwarded[:idx]
		}
		forwarded = strings.ToLower(strings.TrimSpace(forwarded))
		if forwarded == "http" || forwarded == "https" {
			scheme = forwarded
		}
	}
	host := strings.ToLower(strings.TrimSpace(r.Host))
	if host == "" {
		host = strings.ToLower(strings.TrimSpace(r.Header.Get("Host")))
	}
	if host == "" {
		return ""
	}
	return scheme + "://" + host
}

func (s *Server) pingInterval() time.Duration {
	if s.wsConfig.PingIntervalSeconds <= 0 {
		return time.Duration(config.DefaultGatewayWSConfig().PingIntervalSeconds) * time.Second
	}
	return time.Duration(s.wsConfig.PingIntervalSeconds) * time.Second
}

func (s *Server) heartbeatTimeout() time.Duration {
	if s.wsConfig.HeartbeatTimeoutSeconds <= 0 {
		return time.Duration(config.DefaultGatewayWSConfig().HeartbeatTimeoutSeconds) * time.Second
	}
	return time.Duration(s.wsConfig.HeartbeatTimeoutSeconds) * time.Second
}

type commandRateLimiter struct {
	mu     sync.Mutex
	rate   float64
	burst  float64
	tokens float64
	last   time.Time
	now    func() time.Time
}

func newCommandRateLimiter(wsConfig config.GatewayWSConfig, now func() time.Time) *commandRateLimiter {
	if wsConfig.CommandRateLimitPerSecond <= 0 {
		return nil
	}
	burst := wsConfig.CommandRateLimitBurst
	if burst <= 0 {
		burst = wsConfig.CommandRateLimitPerSecond
	}
	if now == nil {
		now = time.Now
	}
	return &commandRateLimiter{
		rate:   float64(wsConfig.CommandRateLimitPerSecond),
		burst:  float64(burst),
		tokens: float64(burst),
		last:   now().UTC(),
		now:    now,
	}
}

func (l *commandRateLimiter) Allow() bool {
	if l == nil {
		return true
	}
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now().UTC()
	if l.last.IsZero() {
		l.last = now
	}
	elapsed := now.Sub(l.last).Seconds()
	if elapsed > 0 {
		l.tokens += elapsed * l.rate
		if l.tokens > l.burst {
			l.tokens = l.burst
		}
		l.last = now
	}
	if l.tokens < 1 {
		return false
	}
	l.tokens--
	return true
}

func unmarshalPayload(payload json.RawMessage, dst interface{}) error {
	if len(payload) == 0 || string(payload) == "null" {
		payload = []byte("{}")
	}
	if err := json.Unmarshal(payload, dst); err != nil {
		return apperror.InvalidArgument("command payload is invalid")
	}
	return nil
}

func (c *Connection) traceContext() observability.TraceContext {
	if c == nil {
		return observability.NewTraceContext("", "")
	}
	return observability.NewTraceContext(c.TraceID, c.RequestID)
}

func logWebSocketCommand(conn *Connection, traceContext observability.TraceContext, resp responseFrame) {
	connectionID := ""
	userID := ""
	if conn != nil {
		connectionID = conn.ID
		userID = conn.UserID
	}
	code := ""
	if resp.Error != nil {
		code = resp.Error.Code
	}
	log.Printf(
		"websocket_command trace_id=%s request_id=%s connection_id=%s user_id=%s command=%s status=%s error_code=%s",
		traceContext.TraceID,
		resp.RequestID,
		connectionID,
		userID,
		resp.Type,
		resp.Status,
		code,
	)
}

func toDeliveryMessage(message logic.Message) delivery.Message {
	return delivery.Message{
		ServerMsgID:           message.ServerMsgID,
		ClientMsgID:           message.ClientMsgID,
		ConversationID:        message.ConversationID,
		Seq:                   message.Seq,
		SenderID:              message.SenderID,
		ReceiverID:            message.ReceiverID,
		GroupID:               message.GroupID,
		ChatType:              message.ChatType,
		ContentType:           message.ContentType,
		Content:               message.Content,
		MessageOrigin:         message.MessageOrigin,
		AgentAccountID:        message.AgentAccountID,
		TriggerServerMsgID:    message.TriggerServerMsgID,
		AgentRunID:            message.AgentRunID,
		AllowRecursiveTrigger: message.AllowRecursiveTrigger,
		SendTime:              message.SendTime,
		CreatedAt:             message.CreatedAt,
	}
}

func applyTraceContextToDeliveryMessage(ctx context.Context, message *delivery.Message) {
	if message == nil {
		return
	}
	traceContext := observability.TraceContextFromContext(ctx)
	if traceContext.TraceID == "" {
		return
	}
	message.TraceID = traceContext.TraceID
	message.RequestID = traceContext.RequestID
	message.TraceParent = traceContext.TraceParent
	message.TraceState = traceContext.TraceState
}

func toGatewayMessage(message logic.Message) gateway.MessageSnapshot {
	return gateway.MessageSnapshot{
		ServerMsgID:           message.ServerMsgID,
		ClientMsgID:           message.ClientMsgID,
		ConversationID:        message.ConversationID,
		Seq:                   message.Seq,
		SenderID:              message.SenderID,
		ReceiverID:            message.ReceiverID,
		GroupID:               message.GroupID,
		ChatType:              message.ChatType,
		ContentType:           message.ContentType,
		Content:               message.Content,
		MessageOrigin:         message.MessageOrigin,
		AgentAccountID:        message.AgentAccountID,
		TriggerServerMsgID:    message.TriggerServerMsgID,
		AgentRunID:            message.AgentRunID,
		AllowRecursiveTrigger: message.AllowRecursiveTrigger,
		SendTime:              message.SendTime,
		CreatedAt:             message.CreatedAt,
	}
}

func toGatewayConversationState(state logic.ConversationSeqState) gateway.ConversationSeqState {
	var lastMessage *gateway.MessageSnapshot
	if state.LastMessage != nil {
		message := toGatewayMessage(*state.LastMessage)
		lastMessage = &message
	}
	return gateway.ConversationSeqState{
		ConversationID: state.ConversationID,
		MaxSeq:         state.MaxSeq,
		HasReadSeq:     state.HasReadSeq,
		UnreadCount:    state.UnreadCount,
		MaxSeqTime:     state.MaxSeqTime,
		LastMessage:    lastMessage,
	}
}

func bearerToken(headerValue string) string {
	headerValue = strings.TrimSpace(headerValue)
	if headerValue == "" {
		return ""
	}
	parts := strings.Fields(headerValue)
	if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
		return strings.TrimSpace(parts[1])
	}
	return ""
}

func clientPlatform(r *http.Request) string {
	return strings.TrimSpace(firstNonEmptyString(
		r.Header.Get("X-Client-Platform"),
		r.Header.Get("X-Platform"),
		r.URL.Query().Get("platform"),
	))
}

func newConnectionID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err == nil {
		return "conn_" + hex.EncodeToString(b[:])
	}
	return "conn_" + strings.ReplaceAll(time.Now().UTC().Format("20060102150405.000000000"), ".", "")
}

func defaultGatewayInstanceID() string {
	if value := firstNonEmptyString(os.Getenv("GATEWAY_INSTANCE_ID"), os.Getenv("AGENTS_IM_GATEWAY_INSTANCE_ID")); value != "" {
		return value
	}
	if hostname, err := os.Hostname(); err == nil && strings.TrimSpace(hostname) != "" {
		return strings.TrimSpace(hostname)
	}
	return "gateway-local"
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func normalizeAuth(auth config.JWTAuthConfig) config.JWTAuthConfig {
	defaults := config.DefaultJWTAuthConfig()
	if strings.TrimSpace(auth.AccessSecret) == "" {
		auth.AccessSecret = defaults.AccessSecret
	}
	if auth.AccessExpire <= 0 {
		auth.AccessExpire = defaults.AccessExpire
	}
	return auth
}

var errNilMessageLogic = errors.New("message logic is not configured")

func (s *Server) Ready() error {
	if s.configErr != nil {
		return s.configErr
	}
	if s.messageLogic == nil {
		return errNilMessageLogic
	}
	return nil
}
