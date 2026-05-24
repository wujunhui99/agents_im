package logic

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/wujunhui99/agents_im/internal/domain/agentaudit"
	"github.com/wujunhui99/agents_im/internal/model"
	"github.com/wujunhui99/agents_im/internal/repository"
)

func TestAdminConversationLookupReturnsOrderedSanitizedMessages(t *testing.T) {
	ctx := context.Background()
	accountRepo := repository.NewMemoryRepository()
	messageRepo := repository.NewMemoryMessageRepository()
	adminLogic := NewAdminLogic(AdminLogicConfig{
		Accounts: accountRepo,
		Friends:  accountRepo,
		Messages: messageRepo,
	})

	if _, _, err := messageRepo.CreateMessageIdempotent(ctx, repository.CreateMessageInput{
		SenderID:           "1001",
		ReceiverID:         "2002",
		ChatType:           repository.ChatTypeSingle,
		ClientMsgID:        "client-admin-1",
		ContentType:        repository.ContentTypeText,
		Content:            "hello",
		MessageOrigin:      repository.MessageOriginHuman,
		ParticipantUserIDs: []string{"1001", "2002"},
	}); err != nil {
		t.Fatalf("seed first message: %v", err)
	}
	second, _, err := messageRepo.CreateMessageIdempotent(ctx, repository.CreateMessageInput{
		SenderID:           "2002",
		ReceiverID:         "1001",
		ChatType:           repository.ChatTypeSingle,
		ClientMsgID:        "client-admin-2",
		ContentType:        repository.ContentTypeText,
		Content:            "api_token=must-not-leak bearer=also-secret",
		MessageOrigin:      repository.MessageOriginAI,
		AgentAccountID:     "2002",
		TriggerServerMsgID: "msg_trigger",
		AgentRunID:         "run_admin_1",
		ParticipantUserIDs: []string{"1001", "2002"},
	})
	if err != nil {
		t.Fatalf("seed second message: %v", err)
	}

	resp, err := adminLogic.GetConversationMessages(ctx, AdminConversationMessagesRequest{
		ConversationID: second.ConversationID,
		Limit:          20,
	})
	if err != nil {
		t.Fatalf("admin conversation lookup: %v", err)
	}
	if resp.ConversationID != second.ConversationID || len(resp.Messages) != 2 {
		t.Fatalf("conversation response mismatch: %+v", resp)
	}
	if resp.Messages[0].Seq != 1 || resp.Messages[1].Seq != 2 {
		t.Fatalf("messages were not ordered by seq asc: %+v", resp.Messages)
	}
	if strings.Contains(resp.Messages[1].Content, "must-not-leak") || strings.Contains(resp.Messages[1].Content, "also-secret") {
		t.Fatalf("message content was not sanitized: %+v", resp.Messages[1])
	}
	if resp.Messages[1].MessageOrigin != repository.MessageOriginAI || resp.Messages[1].AgentRunID != "run_admin_1" {
		t.Fatalf("AI metadata missing from admin message: %+v", resp.Messages[1])
	}
}

func TestAdminUserDetailFriendsAndConversationsAreSafeReadOnlyViews(t *testing.T) {
	ctx := context.Background()
	accountRepo := repository.NewMemoryRepository()
	messageRepo := repository.NewMemoryMessageRepository()
	user, friend := seedAdminUserAndAcceptedFriend(t, ctx, accountRepo)
	_, _, err := messageRepo.CreateMessageIdempotent(ctx, repository.CreateMessageInput{
		SenderID:           user.UserID,
		ReceiverID:         friend.UserID,
		ChatType:           repository.ChatTypeSingle,
		ClientMsgID:        "client-user-conversation",
		ContentType:        repository.ContentTypeText,
		Content:            "conversation for admin",
		MessageOrigin:      repository.MessageOriginHuman,
		ParticipantUserIDs: []string{user.UserID, friend.UserID},
	})
	if err != nil {
		t.Fatalf("seed conversation: %v", err)
	}
	adminLogic := NewAdminLogic(AdminLogicConfig{
		Accounts: accountRepo,
		Friends:  accountRepo,
		Messages: messageRepo,
	})

	detail, err := adminLogic.GetUserDetail(ctx, AdminUserDetailRequest{AccountID: user.UserID})
	if err != nil {
		t.Fatalf("admin user detail: %v", err)
	}
	assertNoAdminSecretFields(t, detail)
	if detail.User.UserID != user.UserID || detail.User.Identifier != user.Identifier || detail.User.AccountType != string(model.AccountTypeUser) {
		t.Fatalf("safe user fields mismatch: %+v", detail.User)
	}

	friends, err := adminLogic.GetUserFriends(ctx, AdminUserFriendsRequest{AccountID: user.UserID})
	if err != nil {
		t.Fatalf("admin user friends: %v", err)
	}
	if len(friends.Friends) != 1 || friends.Friends[0].FriendID != friend.UserID || friends.Friends[0].Status != model.FriendshipStatusAccepted {
		t.Fatalf("accepted friends mismatch: %+v", friends.Friends)
	}
	if friends.Friends[0].Friend == nil || friends.Friends[0].Friend.Identifier != friend.Identifier {
		t.Fatalf("friend profile missing: %+v", friends.Friends[0])
	}

	conversations, err := adminLogic.GetUserConversations(ctx, AdminUserConversationsRequest{AccountID: user.UserID})
	if err != nil {
		t.Fatalf("admin user conversations: %v", err)
	}
	if len(conversations.Conversations) != 1 || conversations.Conversations[0].ConversationID == "" {
		t.Fatalf("conversation ids missing: %+v", conversations.Conversations)
	}
	assertNoAdminSecretFields(t, conversations)
}

func TestAdminLLMTraceListAndDetailReturnMetadataWithConversationLinks(t *testing.T) {
	ctx := context.Background()
	auditRepo := repository.NewMemoryAgentAuditRepository()
	startedAt := time.Date(2026, 5, 18, 8, 0, 0, 0, time.UTC)
	finishedAt := startedAt.Add(1500 * time.Millisecond)
	if _, err := auditRepo.CreateAgentRun(ctx, agentaudit.CreateRunInput{
		RunID:            "run_trace_1",
		AgentID:          "agent_1",
		ConversationID:   "single:1001:2002",
		TriggerMessageID: "msg_human_1",
		RequestingUserID: "1001",
		Status:           agentaudit.StatusFailed,
		InputSummary: agentaudit.Summary{
			"provider":     "deepseek",
			"model":        "deepseek-chat",
			"prompt_hash":  "prompt-hash-1",
			"prompt_ver":   "ai-hosting-v1",
			"provider_key": "must-not-leak",
		},
		OutputSummary: agentaudit.Summary{
			"latency_ms":   int64(1500),
			"total_tokens": int64(42),
		},
		ErrorCode:    "LLM_FAILED",
		ErrorMessage: "upstream bearer must-not-leak failed",
		TraceID:      "trace_admin_1",
		RequestID:    "req_admin_1",
		StartedAt:    startedAt,
		FinishedAt:   finishedAt,
	}); err != nil {
		t.Fatalf("seed agent run: %v", err)
	}
	adminLogic := NewAdminLogic(AdminLogicConfig{AgentAudits: auditRepo})

	list, err := adminLogic.ListLLMTraces(ctx, AdminLLMTraceListRequest{Limit: 20})
	if err != nil {
		t.Fatalf("admin trace list: %v", err)
	}
	if len(list.Traces) != 1 {
		t.Fatalf("trace list mismatch: %+v", list.Traces)
	}
	trace := list.Traces[0]
	if trace.TraceID != "trace_admin_1" || trace.RunID != "run_trace_1" || trace.ConversationID != "single:1001:2002" {
		t.Fatalf("trace metadata missing links: %+v", trace)
	}
	if trace.TraceURL != "" {
		t.Fatalf("legacy non-OTel trace id should not produce trace URL: %+v", trace)
	}
	if trace.Provider != "deepseek" || trace.Model != "deepseek-chat" || trace.PromptHash != "prompt-hash-1" {
		t.Fatalf("trace model/prompt metadata missing: %+v", trace)
	}
	assertNoAdminSecretFields(t, list)

	detail, err := adminLogic.GetLLMTraceDetail(ctx, AdminLLMTraceDetailRequest{TraceID: "trace_admin_1"})
	if err != nil {
		t.Fatalf("admin trace detail: %v", err)
	}
	if detail.Trace.RunID != "run_trace_1" || detail.Trace.ConversationID != "single:1001:2002" {
		t.Fatalf("trace detail mismatch: %+v", detail.Trace)
	}
	assertNoAdminSecretFields(t, detail)
}

func TestAdminTraceIncludesTraceURLForOTelTraceID(t *testing.T) {
	trace := adminTraceFromRun(agentaudit.AgentRun{
		RunID:     "run_otel_trace",
		TraceID:   "4bf92f3577b34da6a3ce929d0e0e4736",
		AgentID:   "agent_1",
		Status:    agentaudit.StatusSucceeded,
		CreatedAt: time.Date(2026, 5, 21, 8, 0, 0, 0, time.UTC),
	})

	if !strings.HasPrefix(trace.TraceURL, "https://grafana.agenticim.xyz/explore?") ||
		!strings.Contains(trace.TraceURL, "Tempo") ||
		!strings.Contains(trace.TraceURL, "4bf92f3577b34da6a3ce929d0e0e4736") {
		t.Fatalf("trace URL mismatch: %+v", trace)
	}
}

func seedAdminUserAndAcceptedFriend(t *testing.T, ctx context.Context, repo *repository.MemoryRepository) (model.User, model.User) {
	t.Helper()
	user, err := repo.Create(ctx, model.User{
		Identifier:  "alice_admin_lookup",
		DisplayName: "Alice",
		Name:        "Alice",
		AccountType: model.AccountTypeUser,
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	friend, err := repo.Create(ctx, model.User{
		Identifier:  "bob_admin_lookup",
		DisplayName: "Bob",
		Name:        "Bob",
		AccountType: model.AccountTypeUser,
	})
	if err != nil {
		t.Fatalf("create friend: %v", err)
	}
	if _, _, err := repo.AddFriend(ctx, user.UserID, friend.UserID); err != nil {
		t.Fatalf("add friend pending: %v", err)
	}
	if _, _, err := repo.AcceptFriendRequest(ctx, friend.UserID, user.UserID); err != nil {
		t.Fatalf("accept friend: %v", err)
	}
	return user, friend
}

func assertNoAdminSecretFields(t *testing.T, value any) {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal admin response: %v", err)
	}
	lower := strings.ToLower(string(raw))
	for _, forbidden := range []string{
		"must-not-leak",
		"also-secret",
		"provider_key",
		"secret",
		"access_token",
		"refresh_token",
		"authorization",
		"cookie",
		"dsn",
		"database_url",
	} {
		if strings.Contains(lower, forbidden) {
			t.Fatalf("admin response leaked forbidden value %q: %s", forbidden, string(raw))
		}
	}
}
