package ws

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
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
	upgrader     websocket.Upgrader
	now          func() time.Time
}

type ServerOption func(*Server)

type Connection struct {
	ID          string
	UserID      string
	ConnectedAt time.Time

	ws        *websocket.Conn
	writeMu   sync.Mutex
	lastSeen  time.Time
	lastSeenM sync.RWMutex
}

type commandFrame struct {
	RequestID      string          `json:"request_id,omitempty"`
	RequestIDCamel string          `json:"requestId,omitempty"`
	Type           string          `json:"type,omitempty"`
	Command        string          `json:"command,omitempty"`
	Payload        json.RawMessage `json:"payload,omitempty"`
}

type responseFrame struct {
	RequestID string         `json:"request_id"`
	Type      string         `json:"type"`
	Status    string         `json:"status"`
	Error     *responseError `json:"error,omitempty"`
	Data      interface{}    `json:"data,omitempty"`
}

type responseError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type heartbeatData struct {
	ConnectionID string `json:"connection_id"`
	UserID       string `json:"user_id"`
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

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.HandleWebSocket(w, r)
}

func (s *Server) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	claims, err := s.authenticate(r)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		return
	}
	if s.messageLogic == nil {
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
		ConnectedAt: now,
		ws:          socket,
		lastSeen:    now,
	}
	s.connections.Register(conn)
	defer func() {
		s.connections.Unregister(conn.ID)
		_ = socket.Close()
	}()

	s.readLoop(r.Context(), conn)
}

func (s *Server) Connections() *ConnectionManager {
	return s.connections
}

func (s *Server) DeliveryDispatcher() delivery.Dispatcher {
	return NewInMemoryDeliveryDispatcher(s.connections)
}

func (s *Server) PushToUser(ctx context.Context, userID string, event delivery.Event) (delivery.Result, error) {
	return s.DeliveryDispatcher().DeliverToUser(ctx, userID, event)
}

func (s *Server) PushToConversation(ctx context.Context, conversationID string, recipientUserIDs []string, event delivery.Event) (delivery.Result, error) {
	return s.DeliveryDispatcher().DeliverToConversation(ctx, conversationID, recipientUserIDs, event)
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
	frame, err := decodeCommandFrame(raw)
	if err != nil {
		return errorResponse("", "", apperror.InvalidArgument("command envelope is invalid"))
	}
	requestID := frame.requestID()
	commandType := frame.commandType()
	if strings.TrimSpace(requestID) == "" {
		return errorResponse("", commandType, apperror.InvalidArgument("request_id is required"))
	}
	if strings.TrimSpace(commandType) == "" {
		return errorResponse(requestID, "", apperror.InvalidArgument("type is required"))
	}

	data, err := s.dispatch(ctx, conn, commandType, frame.Payload)
	if err != nil {
		return errorResponse(requestID, commandType, err)
	}
	return responseFrame{
		RequestID: requestID,
		Type:      commandType,
		Status:    gateway.AckStatusOK,
		Data:      data,
	}
}

func (s *Server) dispatch(ctx context.Context, conn *Connection, commandType string, payload json.RawMessage) (interface{}, error) {
	switch commandType {
	case CommandHeartbeat:
		return heartbeatData{
			ConnectionID: conn.ID,
			UserID:       conn.UserID,
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

func (c *Connection) Info() ConnectionInfo {
	return ConnectionInfo{
		ConnectionID: c.ID,
		UserID:       c.UserID,
		ConnectedAt:  c.ConnectedAt,
		LastSeenAt:   c.LastSeen(),
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

func errorResponse(requestID string, commandType string, err error) responseFrame {
	appErr := apperror.From(err)
	code := string(appErr.Code)
	if appErr.Code == apperror.CodeAlreadyExists && strings.Contains(strings.ToLower(appErr.Message), "idempotency") {
		code = "IDEMPOTENCY_CONFLICT"
	}
	return responseFrame{
		RequestID: requestID,
		Type:      commandType,
		Status:    gateway.AckStatusError,
		Error: &responseError{
			Code:    code,
			Message: appErr.Message,
		},
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

func newConnectionID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err == nil {
		return "conn_" + hex.EncodeToString(b[:])
	}
	return "conn_" + strings.ReplaceAll(time.Now().UTC().Format("20060102150405.000000000"), ".", "")
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
