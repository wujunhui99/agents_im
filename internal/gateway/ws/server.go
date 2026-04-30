package ws

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/wujunhui99/agents_im/internal/apperror"
	"github.com/wujunhui99/agents_im/internal/auth/token"
	"github.com/wujunhui99/agents_im/internal/config"
	"github.com/wujunhui99/agents_im/internal/gateway"
	"github.com/wujunhui99/agents_im/internal/gateway/delivery"
	"github.com/wujunhui99/agents_im/internal/logic"
	"github.com/wujunhui99/agents_im/internal/observability"
	"github.com/wujunhui99/agents_im/internal/presence"
	"github.com/wujunhui99/agents_im/internal/svc"
)

const CommandHeartbeat = gateway.CommandHeartbeat

const (
	defaultReadLimit        = 64 * 1024
	defaultWriteWait        = 10 * time.Second
	defaultHeartbeatTimeout = 75 * time.Second
)

type Server struct {
	auth         config.JWTAuthConfig
	tokenManager token.Manager
	messageLogic *logic.MessageLogic
	connections  *ConnectionManager
	dispatcher   delivery.Dispatcher
	presence     presence.PresenceStore
	presenceTTL  time.Duration
	instanceID   string
	upgrader     websocket.Upgrader
	now          func() time.Time
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
	ConnectedAt time.Time
	TraceID     string
	RequestID   string

	ws        *websocket.Conn
	writeMu   sync.Mutex
	lastSeen  time.Time
	lastSeenM sync.RWMutex
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

func NewServer(serviceContext *svc.ServiceContext, opts ...ServerOption) *Server {
	auth := config.DefaultJWTAuthConfig()
	var messageLogic *logic.MessageLogic
	if serviceContext != nil {
		auth = serviceContext.Auth
		messageLogic = serviceContext.MessageLogic
	}
	auth = normalizeAuth(auth)

	server := &Server{
		auth:         auth,
		tokenManager: token.NewHMACTokenManager(auth.AccessSecret, time.Duration(auth.AccessExpire)*time.Second),
		messageLogic: messageLogic,
		connections:  NewConnectionManager(),
		presence:     presence.NewMemoryStore(),
		presenceTTL:  presence.HeartbeatTTL(config.DefaultPresenceConfig()),
		instanceID:   defaultGatewayInstanceID(),
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
		},
		now: time.Now,
	}
	for _, opt := range opts {
		opt(server)
	}
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

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.HandleWebSocket(w, r)
}

func (s *Server) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	tracedRequest, traceContext := observability.EnsureHTTPTrace(r)
	r = tracedRequest
	observability.InjectTraceHeaders(w, traceContext)

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
		ConnectedAt: now,
		TraceID:     traceContext.TraceID,
		RequestID:   traceContext.RequestID,
		ws:          socket,
		lastSeen:    now,
	}
	_ = s.registerPresence(r.Context(), conn)
	s.connections.Register(conn)
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

	s.readLoop(r.Context(), conn)
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
	if rawToken == "" {
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
	return claims, nil
}

func (s *Server) readLoop(ctx context.Context, conn *Connection) {
	conn.ws.SetReadLimit(defaultReadLimit)
	_ = conn.ws.SetReadDeadline(s.now().Add(defaultHeartbeatTimeout))
	conn.ws.SetPongHandler(func(string) error {
		conn.touch(s.now())
		_ = s.refreshPresence(ctx, conn)
		return conn.ws.SetReadDeadline(s.now().Add(defaultHeartbeatTimeout))
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

		resp := s.handleCommand(ctx, conn, raw)
		if err := conn.writeJSON(resp); err != nil {
			return
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
	recipients := pushRecipients(senderID, message, recipientUserIDs)
	if len(recipients) == 0 {
		return
	}
	_, err := s.PushToConversation(ctx, message.ConversationID, recipients, delivery.NewMessageEvent(delivery.EventMessageReceived, toDeliveryMessage(message)))
	if err != nil {
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
	default:
		return "INTERNAL"
	}
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
		ServerMsgID:    message.ServerMsgID,
		ClientMsgID:    message.ClientMsgID,
		ConversationID: message.ConversationID,
		Seq:            message.Seq,
		SenderID:       message.SenderID,
		ReceiverID:     message.ReceiverID,
		GroupID:        message.GroupID,
		ChatType:       message.ChatType,
		ContentType:    message.ContentType,
		Content:        message.Content,
		SendTime:       message.SendTime,
		CreatedAt:      message.CreatedAt,
	}
}

func toGatewayMessage(message logic.Message) gateway.MessageSnapshot {
	return gateway.MessageSnapshot{
		ServerMsgID:    message.ServerMsgID,
		ClientMsgID:    message.ClientMsgID,
		ConversationID: message.ConversationID,
		Seq:            message.Seq,
		SenderID:       message.SenderID,
		ReceiverID:     message.ReceiverID,
		GroupID:        message.GroupID,
		ChatType:       message.ChatType,
		ContentType:    message.ContentType,
		Content:        message.Content,
		SendTime:       message.SendTime,
		CreatedAt:      message.CreatedAt,
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
	if s.messageLogic == nil {
		return errNilMessageLogic
	}
	return nil
}
