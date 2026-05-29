package tests

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sort"
	"testing"
	"time"

	"github.com/wujunhui99/agents_im/pkg/apperror"
	"github.com/wujunhui99/agents_im/internal/logic"
	"github.com/wujunhui99/agents_im/internal/repository"
	messagesvc "github.com/wujunhui99/agents_im/internal/servicecontext/message"
)

func TestMessageSingleChatSeqStartsAtOne(t *testing.T) {
	messageLogic := newMessageTestLogic()

	result, err := messageLogic.SendMessage(context.Background(), testSendRequest("usr_b", "usr_a", "client-1", "hello"))
	if err != nil {
		t.Fatalf("send message: %v", err)
	}
	if result.Deduplicated {
		t.Fatal("first send should not be deduplicated")
	}
	if result.Message.Seq != 1 {
		t.Fatalf("seq = %d, want 1", result.Message.Seq)
	}
	wantConversationID := repository.SingleConversationID("usr_b", "usr_a")
	if result.Message.ConversationID != wantConversationID {
		t.Fatalf("conversation_id = %q, want %q", result.Message.ConversationID, wantConversationID)
	}
	if result.Message.ServerMsgID == "" {
		t.Fatal("server_msg_id is empty")
	}
}

func TestMessageSendIdempotentRetry(t *testing.T) {
	messageLogic := newMessageTestLogic()
	ctx := context.Background()
	req := testSendRequest("usr_a", "usr_b", "client-retry", "hello")

	first, err := messageLogic.SendMessage(ctx, req)
	if err != nil {
		t.Fatalf("first send: %v", err)
	}
	second, err := messageLogic.SendMessage(ctx, req)
	if err != nil {
		t.Fatalf("retry send: %v", err)
	}
	if !second.Deduplicated {
		t.Fatal("retry should be deduplicated")
	}
	if second.Message.ServerMsgID != first.Message.ServerMsgID || second.Message.Seq != first.Message.Seq {
		t.Fatalf("deduplicated response changed message: first=%+v second=%+v", first.Message, second.Message)
	}

	states, err := messageLogic.GetConversationSeqs(ctx, logic.GetConversationSeqsRequest{
		UserID:          "usr_a",
		ConversationIDs: []string{first.Message.ConversationID},
	})
	if err != nil {
		t.Fatalf("get seqs: %v", err)
	}
	if states.States[0].MaxSeq != 1 {
		t.Fatalf("idempotent retry should not advance max seq: %+v", states.States[0])
	}
}

func TestMessageSendIdempotencyConflict(t *testing.T) {
	messageLogic := newMessageTestLogic()
	ctx := context.Background()

	if _, err := messageLogic.SendMessage(ctx, testSendRequest("usr_a", "usr_b", "client-conflict", "hello")); err != nil {
		t.Fatalf("first send: %v", err)
	}
	_, err := messageLogic.SendMessage(ctx, testSendRequest("usr_a", "usr_b", "client-conflict", "changed"))
	if err == nil || apperror.From(err).Code != apperror.CodeAlreadyExists {
		t.Fatalf("conflict error = %v, want ALREADY_EXISTS", err)
	}
}

func TestMessagePullBySeqRange(t *testing.T) {
	messageLogic := newMessageTestLogic()
	ctx := context.Background()

	first, err := messageLogic.SendMessage(ctx, testSendRequest("usr_a", "usr_b", "client-pull-1", "one"))
	if err != nil {
		t.Fatalf("send first: %v", err)
	}
	if _, err := messageLogic.SendMessage(ctx, testSendRequest("usr_b", "usr_a", "client-pull-2", "two")); err != nil {
		t.Fatalf("send second: %v", err)
	}
	if _, err := messageLogic.SendMessage(ctx, testSendRequest("usr_a", "usr_b", "client-pull-3", "three")); err != nil {
		t.Fatalf("send third: %v", err)
	}

	pulled, err := messageLogic.PullMessages(ctx, logic.PullMessagesRequest{
		UserID:         "usr_b",
		ConversationID: first.Message.ConversationID,
		FromSeq:        2,
		ToSeq:          3,
		Limit:          10,
		Order:          "asc",
	})
	if err != nil {
		t.Fatalf("pull messages: %v", err)
	}
	if len(pulled.Messages) != 2 {
		t.Fatalf("pulled %d messages, want 2: %+v", len(pulled.Messages), pulled.Messages)
	}
	if pulled.Messages[0].Seq != 2 || pulled.Messages[1].Seq != 3 {
		t.Fatalf("messages not pulled in asc seq order: %+v", pulled.Messages)
	}
	if !pulled.IsEnd || pulled.NextSeq != 4 {
		t.Fatalf("unexpected pull cursor: %+v", pulled)
	}
}

func TestMessageSenderReadSeqAdvancesAfterSend(t *testing.T) {
	messageLogic := newMessageTestLogic()
	ctx := context.Background()

	sent, err := messageLogic.SendMessage(ctx, testSendRequest("usr_a", "usr_b", "client-read-sender", "hello"))
	if err != nil {
		t.Fatalf("send: %v", err)
	}

	senderState := mustMessageState(t, messageLogic, "usr_a", sent.Message.ConversationID)
	if senderState.HasReadSeq != sent.Message.Seq || senderState.UnreadCount != 0 {
		t.Fatalf("sender read state did not advance: %+v", senderState)
	}

	receiverState := mustMessageState(t, messageLogic, "usr_b", sent.Message.ConversationID)
	if receiverState.HasReadSeq != 0 || receiverState.UnreadCount != 1 {
		t.Fatalf("receiver should have one unread message: %+v", receiverState)
	}
}

func TestMessageMarkReadRejectsGreaterThanMaxSeq(t *testing.T) {
	messageLogic := newMessageTestLogic()
	ctx := context.Background()

	sent, err := messageLogic.SendMessage(ctx, testSendRequest("usr_a", "usr_b", "client-read-reject", "hello"))
	if err != nil {
		t.Fatalf("send: %v", err)
	}

	_, err = messageLogic.MarkConversationAsRead(ctx, logic.MarkConversationAsReadRequest{
		UserID:         "usr_b",
		ConversationID: sent.Message.ConversationID,
		HasReadSeq:     sent.Message.Seq + 1,
	})
	if err == nil || apperror.From(err).Code != apperror.CodeInvalidArgument {
		t.Fatalf("mark read error = %v, want INVALID_ARGUMENT", err)
	}
}

func TestMessageMarkReadIsMonotonic(t *testing.T) {
	messageLogic := newMessageTestLogic()
	ctx := context.Background()

	first, err := messageLogic.SendMessage(ctx, testSendRequest("usr_a", "usr_b", "client-mono-1", "one"))
	if err != nil {
		t.Fatalf("send first: %v", err)
	}
	if _, err := messageLogic.SendMessage(ctx, testSendRequest("usr_a", "usr_b", "client-mono-2", "two")); err != nil {
		t.Fatalf("send second: %v", err)
	}

	advanced, err := messageLogic.MarkConversationAsRead(ctx, logic.MarkConversationAsReadRequest{
		UserID:         "usr_b",
		ConversationID: first.Message.ConversationID,
		HasReadSeq:     2,
	})
	if err != nil {
		t.Fatalf("mark read to 2: %v", err)
	}
	if !advanced.Updated || advanced.HasReadSeq != 2 || advanced.UnreadCount != 0 {
		t.Fatalf("unexpected advanced read state: %+v", advanced)
	}

	regressed, err := messageLogic.MarkConversationAsRead(ctx, logic.MarkConversationAsReadRequest{
		UserID:         "usr_b",
		ConversationID: first.Message.ConversationID,
		HasReadSeq:     1,
	})
	if err != nil {
		t.Fatalf("mark read to 1: %v", err)
	}
	if regressed.Updated || regressed.HasReadSeq != 2 || regressed.UnreadCount != 0 {
		t.Fatalf("mark read should be monotonic: %+v", regressed)
	}
}

func TestMessageConversationSeqQueryUnreadCount(t *testing.T) {
	messageLogic := newMessageTestLogic()
	ctx := context.Background()

	first, err := messageLogic.SendMessage(ctx, testSendRequest("usr_a", "usr_b", "client-unread-1", "one"))
	if err != nil {
		t.Fatalf("send first: %v", err)
	}
	if _, err := messageLogic.SendMessage(ctx, testSendRequest("usr_a", "usr_b", "client-unread-2", "two")); err != nil {
		t.Fatalf("send second: %v", err)
	}

	state := mustMessageState(t, messageLogic, "usr_b", first.Message.ConversationID)
	if state.MaxSeq != 2 || state.HasReadSeq != 0 || state.UnreadCount != 2 {
		t.Fatalf("unexpected unread state before read: %+v", state)
	}
	if state.LastMessage == nil || state.LastMessage.Seq != 2 {
		t.Fatalf("last message missing from state: %+v", state)
	}

	if _, err := messageLogic.MarkConversationAsRead(ctx, logic.MarkConversationAsReadRequest{
		UserID:         "usr_b",
		ConversationID: first.Message.ConversationID,
		HasReadSeq:     1,
	}); err != nil {
		t.Fatalf("mark read: %v", err)
	}
	state = mustMessageState(t, messageLogic, "usr_b", first.Message.ConversationID)
	if state.MaxSeq != 2 || state.HasReadSeq != 1 || state.UnreadCount != 1 {
		t.Fatalf("unexpected unread state after read: %+v", state)
	}
}

func TestMessageSendCreatesOutboxEvent(t *testing.T) {
	repo := repository.NewMemoryMessageRepository()
	messageLogic := logic.NewMessageLogic(repo)
	ctx := context.Background()

	sent, err := messageLogic.SendMessage(ctx, testSendRequest("usr_a", "usr_b", "client-outbox-1", "hello outbox"))
	if err != nil {
		t.Fatalf("send: %v", err)
	}
	events, err := repo.PollPending(ctx, "memory-worker-1", 10, time.Minute)
	if err != nil {
		t.Fatalf("poll outbox: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("got %d outbox events, want 1: %+v", len(events), events)
	}
	event := events[0]
	if event.EventType != repository.OutboxEventTypeMessageCreated ||
		event.AggregateType != repository.OutboxAggregateTypeMessage ||
		event.AggregateID != sent.Message.ServerMsgID ||
		event.ServerMsgID != sent.Message.ServerMsgID ||
		event.ConversationID != sent.Message.ConversationID ||
		event.Seq != sent.Message.Seq {
		t.Fatalf("unexpected outbox metadata: %+v", event)
	}

	var payload repository.MessageCreatedOutboxPayload
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		t.Fatalf("decode outbox payload: %v", err)
	}
	if payload.Message.ServerMsgID != sent.Message.ServerMsgID || payload.Message.Content != sent.Message.Content {
		t.Fatalf("payload message mismatch: %+v", payload.Message)
	}
	if !containsString(payload.VisibleUserIDs, "usr_a") || !containsString(payload.VisibleUserIDs, "usr_b") {
		t.Fatalf("payload visible users missing sender/receiver: %+v", payload.VisibleUserIDs)
	}

	if err := repo.MarkFailed(ctx, event.EventID, "memory-worker-1", repository.OutboxFailure{
		NextAttemptAt: time.Now().Add(-time.Second),
		LastError:     "temporary failure",
	}); err != nil {
		t.Fatalf("mark failed: %v", err)
	}
	retried, err := repo.PollPending(ctx, "memory-worker-2", 10, time.Minute)
	if err != nil {
		t.Fatalf("poll retried outbox: %v", err)
	}
	if len(retried) != 1 || retried[0].AttemptCount != 1 || retried[0].LockedBy != "memory-worker-2" {
		t.Fatalf("retry poll mismatch: %+v", retried)
	}
	if err := repo.MarkPublished(ctx, retried[0].EventID, "memory-worker-2"); err != nil {
		t.Fatalf("mark published: %v", err)
	}
	remaining, err := repo.PollPending(ctx, "memory-worker-3", 10, time.Minute)
	if err != nil {
		t.Fatalf("poll after publish: %v", err)
	}
	if len(remaining) != 0 {
		t.Fatalf("published event should not be pending: %+v", remaining)
	}
}

func TestMessageOriginAndAgentMetadataPersistAcrossPullAndOutbox(t *testing.T) {
	repo := repository.NewMemoryMessageRepository()
	messageLogic := logic.NewMessageLogic(repo)
	ctx := context.Background()

	human, err := messageLogic.SendMessage(ctx, testSendRequest("usr_a", "agent_1", "client-origin-human", "hello agent"))
	if err != nil {
		t.Fatalf("send human message: %v", err)
	}
	if human.Message.MessageOrigin != logic.MessageOriginHuman {
		t.Fatalf("default origin = %q, want human", human.Message.MessageOrigin)
	}

	ai, err := messageLogic.SendMessage(ctx, logic.SendMessageRequest{
		SenderID:              "agent_1",
		ReceiverID:            "usr_a",
		ChatType:              logic.MessageChatTypeSingle,
		ClientMsgID:           "client-origin-ai",
		ContentType:           logic.MessageContentTypeText,
		Content:               "AI response",
		MessageOrigin:         logic.MessageOriginAI,
		AgentAccountID:        "agent_1",
		TriggerServerMsgID:    human.Message.ServerMsgID,
		AgentRunID:            "run_1",
		AllowRecursiveTrigger: false,
	})
	if err != nil {
		t.Fatalf("send ai message: %v", err)
	}
	if ai.Message.MessageOrigin != logic.MessageOriginAI ||
		ai.Message.AgentAccountID != "agent_1" ||
		ai.Message.TriggerServerMsgID != human.Message.ServerMsgID ||
		ai.Message.AgentRunID != "run_1" ||
		ai.Message.AllowRecursiveTrigger {
		t.Fatalf("ai metadata mismatch: %+v", ai.Message)
	}

	pulled, err := messageLogic.PullMessages(ctx, logic.PullMessagesRequest{
		UserID:         "usr_a",
		ConversationID: human.Message.ConversationID,
		FromSeq:        1,
		Limit:          10,
		Order:          "asc",
	})
	if err != nil {
		t.Fatalf("pull messages: %v", err)
	}
	if len(pulled.Messages) != 2 || pulled.Messages[1].MessageOrigin != logic.MessageOriginAI {
		t.Fatalf("pulled messages missing ai origin: %+v", pulled.Messages)
	}

	events, err := repo.PollPending(ctx, "origin-worker", 10, time.Minute)
	if err != nil {
		t.Fatalf("poll outbox: %v", err)
	}
	var aiPayload repository.MessageCreatedOutboxPayload
	for _, event := range events {
		var payload repository.MessageCreatedOutboxPayload
		if err := json.Unmarshal(event.Payload, &payload); err != nil {
			t.Fatalf("decode outbox payload: %v", err)
		}
		if payload.Message.ServerMsgID == ai.Message.ServerMsgID {
			aiPayload = payload
			break
		}
	}
	if aiPayload.Message.MessageOrigin != logic.MessageOriginAI ||
		aiPayload.Message.AgentAccountID != "agent_1" ||
		aiPayload.Message.TriggerServerMsgID != human.Message.ServerMsgID {
		t.Fatalf("outbox payload missing ai metadata: %+v", aiPayload.Message)
	}
}

func TestAIDirectMessageVisibleToOwnerAndReceiverAndFanoutIncludesBoth(t *testing.T) {
	repo := repository.NewMemoryMessageRepository()
	messageLogic := logic.NewMessageLogic(repo)
	ctx := context.Background()

	trigger, err := messageLogic.SendMessage(ctx, testSendRequest("usr_b", "usr_a", "client-ai-visible-trigger", "please reply"))
	if err != nil {
		t.Fatalf("send trigger: %v", err)
	}
	ai, err := messageLogic.SendMessage(ctx, logic.SendMessageRequest{
		SenderID:           "usr_a",
		ReceiverID:         "usr_b",
		ChatType:           logic.MessageChatTypeSingle,
		ClientMsgID:        "client-ai-visible-response",
		ContentType:        logic.MessageContentTypeText,
		Content:            "AI response on behalf of usr_a",
		MessageOrigin:      logic.MessageOriginAI,
		AgentAccountID:     "usr_a",
		TriggerServerMsgID: trigger.Message.ServerMsgID,
		AgentRunID:         "run_owner_visibility",
	})
	if err != nil {
		t.Fatalf("send ai response: %v", err)
	}

	recipientUserIDs := append([]string(nil), ai.RecipientUserIDs...)
	sort.Strings(recipientUserIDs)
	if !reflect.DeepEqual(recipientUserIDs, []string{"usr_a", "usr_b"}) {
		t.Fatalf("ai live fanout recipients = %+v, want owner and receiver", recipientUserIDs)
	}

	for _, userID := range []string{"usr_a", "usr_b"} {
		pulled, err := messageLogic.PullMessages(ctx, logic.PullMessagesRequest{
			UserID:         userID,
			ConversationID: trigger.Message.ConversationID,
			FromSeq:        ai.Message.Seq,
			ToSeq:          ai.Message.Seq,
			Limit:          10,
			Order:          "asc",
		})
		if err != nil {
			t.Fatalf("pull messages for %s: %v", userID, err)
		}
		if len(pulled.Messages) != 1 || pulled.Messages[0].ServerMsgID != ai.Message.ServerMsgID {
			t.Fatalf("user %s cannot see ai response in history: %+v", userID, pulled.Messages)
		}
		if pulled.Messages[0].MessageOrigin != logic.MessageOriginAI ||
			pulled.Messages[0].AgentAccountID != "usr_a" ||
			pulled.Messages[0].TriggerServerMsgID != trigger.Message.ServerMsgID ||
			pulled.Messages[0].AgentRunID != "run_owner_visibility" {
			t.Fatalf("user %s pulled ai metadata mismatch: %+v", userID, pulled.Messages[0])
		}
	}
}

func TestMessageGroupSendRequiresActiveMembership(t *testing.T) {
	ctx := context.Background()
	userRepo := repository.NewMemoryRepository()
	userLogic := logic.NewUserLogic(userRepo)
	creator := mustCreateUser(t, userLogic, "msg_group_creator")
	member := mustCreateUser(t, userLogic, "msg_group_member")
	outsider := mustCreateUser(t, userLogic, "msg_group_outsider")

	groupsLogic := logic.NewGroupsLogic(
		repository.NewMemoryGroupsRepository(),
		logic.NewUserLogicExistenceChecker(userLogic),
	)
	group, err := groupsLogic.CreateGroup(ctx, logic.CreateGroupRequest{
		CreatorUserID: creator.UserID,
		Name:          "Message Group",
	})
	if err != nil {
		t.Fatalf("create group: %v", err)
	}
	if _, err := groupsLogic.JoinGroup(ctx, logic.JoinGroupRequest{
		GroupID: group.GroupID,
		UserID:  member.UserID,
	}); err != nil {
		t.Fatalf("join member: %v", err)
	}

	messageLogic := logic.NewMessageLogicWithValidators(
		repository.NewMemoryMessageRepository(),
		logic.NewUserLogicExistenceChecker(userLogic),
		groupsLogic,
	)

	_, err = messageLogic.SendMessage(ctx, testGroupSendRequest(outsider.UserID, group.GroupID, "client-group-outsider", "nope"))
	if err == nil || apperror.From(err).Code != apperror.CodeForbidden {
		t.Fatalf("outsider group send error = %v, want FORBIDDEN", err)
	}

	sent, err := messageLogic.SendMessage(ctx, testGroupSendRequest(member.UserID, group.GroupID, "client-group-member", "hello group"))
	if err != nil {
		t.Fatalf("member group send: %v", err)
	}
	if sent.Message.ChatType != logic.MessageChatTypeGroup ||
		sent.Message.GroupID != group.GroupID ||
		sent.Message.ConversationID != repository.GroupConversationID(group.GroupID) {
		t.Fatalf("unexpected group message: %+v", sent.Message)
	}

	creatorState := mustMessageState(t, messageLogic, creator.UserID, sent.Message.ConversationID)
	if creatorState.MaxSeq != 1 || creatorState.UnreadCount != 1 {
		t.Fatalf("creator should see member group message unread: %+v", creatorState)
	}

	memberState := mustMessageState(t, messageLogic, member.UserID, sent.Message.ConversationID)
	if memberState.HasReadSeq != sent.Message.Seq || memberState.UnreadCount != 0 {
		t.Fatalf("sender read state should advance for group send: %+v", memberState)
	}

	_, err = messageLogic.GetConversationSeqs(ctx, logic.GetConversationSeqsRequest{
		UserID:          outsider.UserID,
		ConversationIDs: []string{sent.Message.ConversationID},
	})
	if err == nil || apperror.From(err).Code != apperror.CodeForbidden {
		t.Fatalf("outsider seq query error = %v, want FORBIDDEN", err)
	}
	_, err = messageLogic.PullMessages(ctx, logic.PullMessagesRequest{
		UserID:         outsider.UserID,
		ConversationID: sent.Message.ConversationID,
		FromSeq:        1,
		Limit:          10,
		Order:          repository.MessageStorageOrderAsc,
	})
	if err == nil || apperror.From(err).Code != apperror.CodeForbidden {
		t.Fatalf("outsider pull error = %v, want FORBIDDEN", err)
	}

	if _, err := groupsLogic.LeaveGroup(ctx, logic.LeaveGroupRequest{
		GroupID: group.GroupID,
		UserID:  member.UserID,
	}); err != nil {
		t.Fatalf("member leave: %v", err)
	}
	_, err = messageLogic.SendMessage(ctx, testGroupSendRequest(member.UserID, group.GroupID, "client-group-left", "left"))
	if err == nil || apperror.From(err).Code != apperror.CodeForbidden {
		t.Fatalf("left member group send error = %v, want FORBIDDEN", err)
	}
	_, err = messageLogic.MarkConversationAsRead(ctx, logic.MarkConversationAsReadRequest{
		UserID:         member.UserID,
		ConversationID: sent.Message.ConversationID,
		HasReadSeq:     sent.Message.Seq,
	})
	if err == nil || apperror.From(err).Code != apperror.CodeForbidden {
		t.Fatalf("left member mark read error = %v, want FORBIDDEN", err)
	}
}

func TestMessageHTTPHandlersUseJWTUser(t *testing.T) {
	serviceContext := messagesvc.NewServiceContextWithAuth(repository.NewMemoryMessageRepository(), nil, nil, testJWTAuthConfig())
	mux := newMessageGoZeroRouter(t, serviceContext)

	t.Run("rejects legacy X-User-Id header without bearer token", func(t *testing.T) {
		headerOnlyResp := httptest.NewRecorder()
		headerOnlyReq := newJSONRequest(http.MethodPost, "/messages", `{"receiverId":"usr_receiver","chatType":"single","clientMsgId":"client-http-bypass","contentType":"text","content":"hello"}`)
		setRejectedLegacyXUserIDHeader(t, headerOnlyReq, "usr_sender")
		mux.ServeHTTP(headerOnlyResp, headerOnlyReq)
		if headerOnlyResp.Code != http.StatusUnauthorized {
			t.Fatalf("legacy X-User-Id rejection status = %d", headerOnlyResp.Code)
		}
	})

	invalidTokenResp := httptest.NewRecorder()
	invalidTokenReq := httptest.NewRequest(http.MethodGet, "/conversations/seqs", nil)
	invalidTokenReq.Header.Set("Authorization", "Bearer [REDACTED]")
	mux.ServeHTTP(invalidTokenResp, invalidTokenReq)
	if invalidTokenResp.Code != http.StatusUnauthorized {
		t.Fatalf("invalid token status = %d", invalidTokenResp.Code)
	}

	mismatchResp := httptest.NewRecorder()
	mismatchReq := newJSONRequest(http.MethodPost, "/messages", `{"senderId":"usr_other","receiverId":"usr_receiver","chatType":"single","clientMsgId":"client-http-mismatch","contentType":"text","content":"hello"}`)
	mismatchReq.Header.Set("Authorization", bearerTokenForUser(t, "usr_sender"))
	mux.ServeHTTP(mismatchResp, mismatchReq)
	if mismatchResp.Code != http.StatusBadRequest {
		t.Fatalf("sender mismatch status = %d, body = %s", mismatchResp.Code, mismatchResp.Body.String())
	}

	sendResp := httptest.NewRecorder()
	sendReq := newJSONRequest(http.MethodPost, "/messages", `{"receiverId":"usr_receiver","chatType":"single","clientMsgId":"client-http-send","contentType":"text","content":"hello"}`)
	sendReq.Header.Set("Authorization", bearerTokenForUser(t, "usr_sender"))
	mux.ServeHTTP(sendResp, sendReq)
	if sendResp.Code != http.StatusOK {
		t.Fatalf("send status = %d, body = %s", sendResp.Code, sendResp.Body.String())
	}

	var sent envelope[logic.SendMessageResponse]
	decodeEnvelope(t, sendResp.Body.Bytes(), &sent)
	if sent.Data.Message.SenderID != "usr_sender" {
		t.Fatalf("message sender did not use token user: %+v", sent.Data.Message)
	}

	seqsResp := httptest.NewRecorder()
	seqsReq := httptest.NewRequest(http.MethodGet, "/conversations/seqs?conversationIds="+sent.Data.Message.ConversationID, nil)
	seqsReq.Header.Set("Authorization", bearerTokenForUser(t, "usr_receiver"))
	mux.ServeHTTP(seqsResp, seqsReq)
	if seqsResp.Code != http.StatusOK {
		t.Fatalf("seqs status = %d, body = %s", seqsResp.Code, seqsResp.Body.String())
	}

	pullResp := httptest.NewRecorder()
	pullReq := httptest.NewRequest(http.MethodGet, "/conversations/"+sent.Data.Message.ConversationID+"/messages?fromSeq=1&limit=10", nil)
	pullReq.Header.Set("Authorization", bearerTokenForUser(t, "usr_receiver"))
	mux.ServeHTTP(pullResp, pullReq)
	if pullResp.Code != http.StatusOK {
		t.Fatalf("pull status = %d, body = %s", pullResp.Code, pullResp.Body.String())
	}
	var pulled envelope[logic.PullMessagesResponse]
	decodeEnvelope(t, pullResp.Body.Bytes(), &pulled)
	if len(pulled.Data.Messages) != 1 || pulled.Data.Messages[0].SenderID != "usr_sender" {
		t.Fatalf("unexpected pull response: %+v", pulled.Data.Messages)
	}

	readResp := httptest.NewRecorder()
	readReq := newJSONRequest(http.MethodPost, "/conversations/"+sent.Data.Message.ConversationID+"/read", `{"hasReadSeq":1}`)
	readReq.Header.Set("Authorization", bearerTokenForUser(t, "usr_receiver"))
	mux.ServeHTTP(readResp, readReq)
	if readResp.Code != http.StatusOK {
		t.Fatalf("mark read status = %d, body = %s", readResp.Code, readResp.Body.String())
	}
	var read envelope[logic.MarkConversationAsReadResponse]
	decodeEnvelope(t, readResp.Body.Bytes(), &read)
	if read.Data.UnreadCount != 0 || read.Data.HasReadSeq != 1 {
		t.Fatalf("unexpected read response: %+v", read.Data)
	}
}

func newMessageTestLogic() *logic.MessageLogic {
	return logic.NewMessageLogic(repository.NewMemoryMessageRepository())
}

func testSendRequest(senderID string, receiverID string, clientMsgID string, content string) logic.SendMessageRequest {
	return logic.SendMessageRequest{
		SenderID:    senderID,
		ReceiverID:  receiverID,
		ChatType:    logic.MessageChatTypeSingle,
		ClientMsgID: clientMsgID,
		ContentType: logic.MessageContentTypeText,
		Content:     content,
	}
}

func testGroupSendRequest(senderID string, groupID string, clientMsgID string, content string) logic.SendMessageRequest {
	return logic.SendMessageRequest{
		SenderID:    senderID,
		GroupID:     groupID,
		ChatType:    logic.MessageChatTypeGroup,
		ClientMsgID: clientMsgID,
		ContentType: logic.MessageContentTypeText,
		Content:     content,
	}
}

func mustMessageState(t *testing.T, messageLogic *logic.MessageLogic, userID string, conversationID string) logic.ConversationSeqState {
	t.Helper()

	result, err := messageLogic.GetConversationSeqs(context.Background(), logic.GetConversationSeqsRequest{
		UserID:          userID,
		ConversationIDs: []string{conversationID},
	})
	if err != nil {
		t.Fatalf("get conversation seqs: %v", err)
	}
	if len(result.States) != 1 {
		t.Fatalf("got %d states, want 1", len(result.States))
	}
	return result.States[0]
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
