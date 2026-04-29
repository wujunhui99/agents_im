package handler

import (
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/wujunhui99/agents_im/internal/apperror"
	"github.com/wujunhui99/agents_im/internal/logic"
	"github.com/wujunhui99/agents_im/internal/response"
	"github.com/wujunhui99/agents_im/internal/svc"
)

func RegisterMessageHandlers(mux *http.ServeMux, ctx *svc.ServiceContext) {
	registerHealthHandler(mux)
	registerMessageHandlers(mux, ctx)
}

func registerMessageHandlers(mux *http.ServeMux, ctx *svc.ServiceContext) {
	mux.HandleFunc("/messages", messagesHandler(ctx))
	mux.HandleFunc("/conversations/seqs", conversationSeqsHandler(ctx))
	mux.HandleFunc("/conversations/", conversationByIDHandler(ctx))
}

type sendMessageHTTPReq struct {
	ReceiverID  string `json:"receiverId"`
	GroupID     string `json:"groupId"`
	ChatType    string `json:"chatType"`
	ClientMsgID string `json:"clientMsgId"`
	ContentType string `json:"contentType"`
	Content     string `json:"content"`
}

type markReadHTTPReq struct {
	HasReadSeq int64 `json:"hasReadSeq"`
}

func messagesHandler(ctx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/messages" {
			response.WriteError(w, apperror.NotFound("route not found"))
			return
		}
		if r.Method != http.MethodPost {
			methodNotAllowed(w)
			return
		}

		userID, err := currentUserID(r)
		if err != nil {
			response.WriteError(w, err)
			return
		}
		var req sendMessageHTTPReq
		if err := decodeJSON(r, &req); err != nil {
			response.WriteError(w, err)
			return
		}

		result, err := ctx.MessageLogic.SendMessage(r.Context(), logic.SendMessageRequest{
			SenderID:    userID,
			ReceiverID:  req.ReceiverID,
			GroupID:     req.GroupID,
			ChatType:    req.ChatType,
			ClientMsgID: req.ClientMsgID,
			ContentType: req.ContentType,
			Content:     req.Content,
		})
		if err != nil {
			response.WriteError(w, err)
			return
		}
		response.WriteOK(w, result)
	}
}

func conversationSeqsHandler(ctx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/conversations/seqs" {
			response.WriteError(w, apperror.NotFound("route not found"))
			return
		}
		if r.Method != http.MethodGet {
			methodNotAllowed(w)
			return
		}

		userID, err := currentUserID(r)
		if err != nil {
			response.WriteError(w, err)
			return
		}

		result, err := ctx.MessageLogic.GetConversationSeqs(r.Context(), logic.GetConversationSeqsRequest{
			UserID:          userID,
			ConversationIDs: splitCommaQuery(r.URL.Query().Get("conversationIds")),
		})
		if err != nil {
			response.WriteError(w, err)
			return
		}
		response.WriteOK(w, result)
	}
}

func conversationByIDHandler(ctx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		conversationID, action, err := splitConversationPath(r.URL.Path)
		if err != nil {
			response.WriteError(w, err)
			return
		}

		switch {
		case action == "messages" && r.Method == http.MethodGet:
			pullMessages(w, r, ctx, conversationID)
		case action == "read" && r.Method == http.MethodPost:
			markConversationAsRead(w, r, ctx, conversationID)
		case action == "messages" || action == "read":
			methodNotAllowed(w)
		default:
			response.WriteError(w, apperror.NotFound("route not found"))
		}
	}
}

func pullMessages(w http.ResponseWriter, r *http.Request, ctx *svc.ServiceContext, conversationID string) {
	userID, err := currentUserID(r)
	if err != nil {
		response.WriteError(w, err)
		return
	}

	fromSeq, err := parseInt64Query(r.URL.Query(), "fromSeq")
	if err != nil {
		response.WriteError(w, err)
		return
	}
	toSeq, err := parseInt64Query(r.URL.Query(), "toSeq")
	if err != nil {
		response.WriteError(w, err)
		return
	}
	limit, err := parseIntQuery(r.URL.Query(), "limit")
	if err != nil {
		response.WriteError(w, err)
		return
	}

	result, err := ctx.MessageLogic.PullMessages(r.Context(), logic.PullMessagesRequest{
		UserID:         userID,
		ConversationID: conversationID,
		FromSeq:        fromSeq,
		ToSeq:          toSeq,
		Limit:          limit,
		Order:          r.URL.Query().Get("order"),
	})
	if err != nil {
		response.WriteError(w, err)
		return
	}
	response.WriteOK(w, result)
}

func markConversationAsRead(w http.ResponseWriter, r *http.Request, ctx *svc.ServiceContext, conversationID string) {
	userID, err := currentUserID(r)
	if err != nil {
		response.WriteError(w, err)
		return
	}

	var req markReadHTTPReq
	if err := decodeJSON(r, &req); err != nil {
		response.WriteError(w, err)
		return
	}

	result, err := ctx.MessageLogic.MarkConversationAsRead(r.Context(), logic.MarkConversationAsReadRequest{
		UserID:         userID,
		ConversationID: conversationID,
		HasReadSeq:     req.HasReadSeq,
	})
	if err != nil {
		response.WriteError(w, err)
		return
	}
	response.WriteOK(w, result)
}

func splitConversationPath(path string) (string, string, error) {
	rest := strings.TrimPrefix(path, "/conversations/")
	if rest == "" || rest == path {
		return "", "", apperror.NotFound("route not found")
	}

	parts := strings.Split(rest, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", apperror.NotFound("route not found")
	}

	conversationID, err := url.PathUnescape(parts[0])
	if err != nil {
		return "", "", apperror.InvalidArgument("conversation_id path is invalid")
	}
	action, err := url.PathUnescape(parts[1])
	if err != nil {
		return "", "", apperror.InvalidArgument("conversation action path is invalid")
	}
	return strings.TrimSpace(conversationID), strings.TrimSpace(action), nil
}

func splitCommaQuery(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}

	rawParts := strings.Split(value, ",")
	parts := make([]string, 0, len(rawParts))
	for _, part := range rawParts {
		part = strings.TrimSpace(part)
		if part != "" {
			parts = append(parts, part)
		}
	}
	return parts
}

func parseInt64Query(values url.Values, key string) (int64, error) {
	value := strings.TrimSpace(values.Get(key))
	if value == "" {
		return 0, nil
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0, apperror.InvalidArgument(key + " must be an integer")
	}
	return parsed, nil
}

func parseIntQuery(values url.Values, key string) (int, error) {
	value := strings.TrimSpace(values.Get(key))
	if value == "" {
		return 0, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, apperror.InvalidArgument(key + " must be an integer")
	}
	return parsed, nil
}
