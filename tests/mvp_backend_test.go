package tests

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	authlogic "github.com/wujunhui99/agents_im/internal/auth/logic"
	authrepo "github.com/wujunhui99/agents_im/internal/auth/repository"
	authsvc "github.com/wujunhui99/agents_im/internal/auth/svc"
	"github.com/wujunhui99/agents_im/internal/auth/token"
	"github.com/wujunhui99/agents_im/internal/auth/useradapter"
	"github.com/wujunhui99/agents_im/internal/gateway"
	"github.com/wujunhui99/agents_im/internal/gateway/delivery"
	"github.com/wujunhui99/agents_im/internal/logic"
	"github.com/wujunhui99/agents_im/internal/model"
	"github.com/wujunhui99/agents_im/internal/repository"
	"github.com/wujunhui99/agents_im/internal/svc"
)

func TestMVPBackendAuthProfileSmoke(t *testing.T) {
	authConfig := testJWTAuthConfig()
	userRepo := repository.NewMemoryRepository()
	userLogic := logic.NewUserLogic(userRepo)
	authServiceContext := authsvc.NewServiceContext(
		authrepo.NewMemoryRepository(),
		useradapter.NewLogicClient(userLogic),
		token.NewHMACTokenManager(authConfig.AccessSecret, time.Duration(authConfig.AccessExpire)*time.Second),
	)
	authMux := newAuthGoZeroRouter(t, authServiceContext)
	userMux := newUserGoZeroRouter(t, svc.NewServiceContextWithAuth(userRepo, authConfig))

	registerResp := httptest.NewRecorder()
	registerReq := newJSONRequest(http.MethodPost, "/auth/register", `{"identifier":"mvp_alice","password":"local-demo-password","display_name":"MVP Alice","gender":"female","birth_date":"1996-05-02","region":"Shanghai"}`)
	authMux.ServeHTTP(registerResp, registerReq)
	if registerResp.Code != http.StatusOK {
		t.Fatalf("register status = %d, body = %s", registerResp.Code, registerResp.Body.String())
	}
	assertNoSecretFields(t, registerResp.Body.String())

	var registered envelope[authlogic.AuthResponse]
	decodeEnvelope(t, registerResp.Body.Bytes(), &registered)
	if registered.Data.UserID == "" || registered.Data.Token == "" || registered.Data.Identifier != "mvp_alice" {
		t.Fatalf("unexpected register response: %+v", registered.Data)
	}

	meResp := httptest.NewRecorder()
	meReq := httptest.NewRequest(http.MethodGet, "/me", nil)
	meReq.Header.Set("Authorization", "Bearer "+registered.Data.Token)
	userMux.ServeHTTP(meResp, meReq)
	if meResp.Code != http.StatusOK {
		t.Fatalf("/me status = %d, body = %s", meResp.Code, meResp.Body.String())
	}
	var me envelope[logic.UserProfile]
	decodeEnvelope(t, meResp.Body.Bytes(), &me)
	if me.Data.UserID != registered.Data.UserID || me.Data.DisplayName != "MVP Alice" {
		t.Fatalf("unexpected /me profile: %+v", me.Data)
	}

	patchResp := httptest.NewRecorder()
	patchReq := newJSONRequest(http.MethodPatch, "/me", `{"display_name":"MVP Alice Updated","region":"Hangzhou"}`)
	patchReq.Header.Set("Authorization", "Bearer "+registered.Data.Token)
	userMux.ServeHTTP(patchResp, patchReq)
	if patchResp.Code != http.StatusOK {
		t.Fatalf("patch /me status = %d, body = %s", patchResp.Code, patchResp.Body.String())
	}
	var patched envelope[logic.UserProfile]
	decodeEnvelope(t, patchResp.Body.Bytes(), &patched)
	if patched.Data.DisplayName != "MVP Alice Updated" || patched.Data.Region != "Hangzhou" {
		t.Fatalf("unexpected patched profile: %+v", patched.Data)
	}

	existsResp := httptest.NewRecorder()
	existsReq := httptest.NewRequest(http.MethodGet, "/users/exists?identifier=MVP_ALICE", nil)
	userMux.ServeHTTP(existsResp, existsReq)
	if existsResp.Code != http.StatusOK {
		t.Fatalf("exists status = %d, body = %s", existsResp.Code, existsResp.Body.String())
	}
	var exists envelope[logic.ExistsByIdentifierResponse]
	decodeEnvelope(t, existsResp.Body.Bytes(), &exists)
	if !exists.Data.Exists || exists.Data.Identifier != "mvp_alice" {
		t.Fatalf("unexpected exists response: %+v", exists.Data)
	}

	publicResp := httptest.NewRecorder()
	publicReq := httptest.NewRequest(http.MethodGet, "/users/mvp_alice", nil)
	userMux.ServeHTTP(publicResp, publicReq)
	if publicResp.Code != http.StatusOK {
		t.Fatalf("public profile status = %d, body = %s", publicResp.Code, publicResp.Body.String())
	}
	assertNoSecretFields(t, publicResp.Body.String())
}

func TestMVPBackendFriendGroupMessageSmoke(t *testing.T) {
	ctx := context.Background()
	userRepo := repository.NewMemoryRepository()
	userLogic := logic.NewUserLogic(userRepo)
	alice := mustCreateUser(t, userLogic, "mvp_friend_alice")
	bob := mustCreateUser(t, userLogic, "mvp_friend_bob")

	friendsLogic := logic.NewFriendsLogic(userRepo, userLogic)
	added, err := friendsLogic.AddFriend(ctx, logic.AddFriendRequest{UserID: alice.UserID, FriendID: bob.UserID})
	if err != nil {
		t.Fatalf("add friend: %v", err)
	}
	if !added.Created || !added.Friendship.IsFriend || added.Friendship.Status != model.FriendshipStatusActive {
		t.Fatalf("unexpected add friend result: %+v", added)
	}
	friends, err := friendsLogic.ListFriends(ctx, logic.ListFriendsRequest{UserID: alice.UserID})
	if err != nil {
		t.Fatalf("list friends: %v", err)
	}
	if len(friends.Friends) != 1 || friends.Friends[0].FriendID != bob.UserID {
		t.Fatalf("unexpected friends list: %+v", friends.Friends)
	}

	groupsLogic := logic.NewGroupsLogic(repository.NewMemoryGroupsRepository(), logic.NewUserLogicExistenceChecker(userLogic))
	group, err := groupsLogic.CreateGroup(ctx, logic.CreateGroupRequest{
		CreatorUserID: alice.UserID,
		Name:          "MVP Smoke Group",
		Description:   "acceptance smoke",
	})
	if err != nil {
		t.Fatalf("create group: %v", err)
	}
	if group.GroupID == "" || group.CreatorUserID != alice.UserID {
		t.Fatalf("unexpected group: %+v", group)
	}
	joined, err := groupsLogic.JoinGroup(ctx, logic.JoinGroupRequest{GroupID: group.GroupID, UserID: bob.UserID})
	if err != nil {
		t.Fatalf("join group: %v", err)
	}
	if joined.AlreadyMember || joined.Member.State != model.MemberStateActive {
		t.Fatalf("unexpected join result: %+v", joined)
	}
	members, err := groupsLogic.ListMembers(ctx, logic.ListMembersRequest{GroupID: group.GroupID})
	if err != nil {
		t.Fatalf("list members: %v", err)
	}
	if len(members.Members) != 2 {
		t.Fatalf("unexpected group members: %+v", members.Members)
	}

	messageLogic := logic.NewMessageLogicWithValidators(
		repository.NewMemoryMessageRepository(),
		logic.NewUserLogicExistenceChecker(userLogic),
		groupsLogic,
	)
	sentSingle, err := messageLogic.SendMessage(ctx, logic.SendMessageRequest{
		SenderID:    alice.UserID,
		ReceiverID:  bob.UserID,
		ChatType:    logic.MessageChatTypeSingle,
		ClientMsgID: "mvp-single-1",
		ContentType: logic.MessageContentTypeText,
		Content:     "hello bob",
	})
	if err != nil {
		t.Fatalf("send single message: %v", err)
	}
	if sentSingle.Message.MessageOrigin != logic.MessageOriginHuman {
		t.Fatalf("single message origin = %q, want human", sentSingle.Message.MessageOrigin)
	}
	bobSingleState := mustMessageState(t, messageLogic, bob.UserID, sentSingle.Message.ConversationID)
	if bobSingleState.MaxSeq != 1 || bobSingleState.UnreadCount != 1 {
		t.Fatalf("unexpected bob single state: %+v", bobSingleState)
	}
	pulled, err := messageLogic.PullMessages(ctx, logic.PullMessagesRequest{
		UserID:         bob.UserID,
		ConversationID: sentSingle.Message.ConversationID,
		FromSeq:        1,
		Limit:          10,
		Order:          "asc",
	})
	if err != nil {
		t.Fatalf("pull single message: %v", err)
	}
	if len(pulled.Messages) != 1 || pulled.Messages[0].ServerMsgID != sentSingle.Message.ServerMsgID {
		t.Fatalf("unexpected pulled single messages: %+v", pulled.Messages)
	}
	read, err := messageLogic.MarkConversationAsRead(ctx, logic.MarkConversationAsReadRequest{
		UserID:         bob.UserID,
		ConversationID: sentSingle.Message.ConversationID,
		HasReadSeq:     sentSingle.Message.Seq,
	})
	if err != nil {
		t.Fatalf("mark single read: %v", err)
	}
	if read.UnreadCount != 0 || read.HasReadSeq != sentSingle.Message.Seq {
		t.Fatalf("unexpected read state: %+v", read)
	}

	sentGroup, err := messageLogic.SendMessage(ctx, logic.SendMessageRequest{
		SenderID:    bob.UserID,
		GroupID:     group.GroupID,
		ChatType:    logic.MessageChatTypeGroup,
		ClientMsgID: "mvp-group-1",
		ContentType: logic.MessageContentTypeText,
		Content:     "hello group",
	})
	if err != nil {
		t.Fatalf("send group message: %v", err)
	}
	if sentGroup.Message.ConversationID != repository.GroupConversationID(group.GroupID) || sentGroup.Message.Seq != 1 {
		t.Fatalf("unexpected group message: %+v", sentGroup.Message)
	}
	aliceGroupState := mustMessageState(t, messageLogic, alice.UserID, sentGroup.Message.ConversationID)
	if aliceGroupState.MaxSeq != 1 || aliceGroupState.UnreadCount != 1 {
		t.Fatalf("unexpected alice group unread state: %+v", aliceGroupState)
	}
}

func TestMVPBackendWebSocketSendPullMarkReadSmoke(t *testing.T) {
	_, server, cleanup := newGatewayWSAppTestServer(t)
	defer cleanup()

	senderConn := dialGatewayWS(t, server.URL, "usr_mvp_ws_sender")
	defer senderConn.Close()
	receiverConn := dialGatewayWS(t, server.URL, "usr_mvp_ws_receiver")
	defer receiverConn.Close()

	writeCommand(t, senderConn, map[string]interface{}{
		"requestId": "mvp-ws-send",
		"command":   gateway.CommandSendMessage,
		"payload": map[string]interface{}{
			"chatType":    "single",
			"receiverId":  "usr_mvp_ws_receiver",
			"clientMsgId": "mvp-ws-client-1",
			"contentType": "text",
			"content":     "hello over mvp websocket",
		},
	})
	sendResp := readWSResponse(t, senderConn)
	if sendResp.RequestID != "mvp-ws-send" || sendResp.Type != gateway.CommandSendMessage || sendResp.Status != gateway.AckStatusOK {
		t.Fatalf("unexpected send ack: %+v", sendResp)
	}
	var sent gateway.SendMessageCommandResponse
	decodeRaw(t, sendResp.Data, &sent)
	if sent.Message.ServerMsgID == "" ||
		sent.Message.SenderID != "usr_mvp_ws_sender" ||
		sent.Message.ReceiverID != "usr_mvp_ws_receiver" ||
		sent.Message.MessageOrigin != logic.MessageOriginHuman {
		t.Fatalf("unexpected sent websocket message: %+v", sent.Message)
	}

	push := readWSPushEvent(t, receiverConn)
	if push.Type != delivery.EventMessageReceived || push.Data.ServerMsgID != sent.Message.ServerMsgID {
		t.Fatalf("unexpected live push: %+v, sent=%+v", push, sent.Message)
	}

	writeCommand(t, receiverConn, map[string]interface{}{
		"requestId": "mvp-ws-seqs",
		"command":   gateway.CommandGetConversationSeqs,
		"payload": map[string]interface{}{
			"conversationIds": []string{sent.Message.ConversationID},
		},
	})
	seqsResp := readWSResponse(t, receiverConn)
	if seqsResp.Type != gateway.CommandGetConversationSeqs || seqsResp.Status != gateway.AckStatusOK {
		t.Fatalf("unexpected seqs ack: %+v", seqsResp)
	}
	var seqs gateway.GetConversationSeqsCommandResponse
	decodeRaw(t, seqsResp.Data, &seqs)
	if len(seqs.States) != 1 || seqs.States[0].MaxSeq != 1 || seqs.States[0].UnreadCount != 1 {
		t.Fatalf("unexpected receiver seqs: %+v", seqs.States)
	}

	writeCommand(t, receiverConn, map[string]interface{}{
		"requestId": "mvp-ws-pull",
		"command":   gateway.CommandPullMessages,
		"payload": map[string]interface{}{
			"conversationId": sent.Message.ConversationID,
			"fromSeq":        1,
			"toSeq":          0,
			"limit":          10,
			"order":          "asc",
		},
	})
	pullResp := readWSResponse(t, receiverConn)
	if pullResp.Type != gateway.CommandPullMessages || pullResp.Status != gateway.AckStatusOK {
		t.Fatalf("unexpected pull ack: %+v", pullResp)
	}
	var pulled gateway.PullMessagesCommandResponse
	decodeRaw(t, pullResp.Data, &pulled)
	if len(pulled.Messages) != 1 ||
		pulled.Messages[0].ServerMsgID != sent.Message.ServerMsgID ||
		pulled.Messages[0].MessageOrigin != logic.MessageOriginHuman {
		t.Fatalf("unexpected pulled websocket messages: %+v", pulled.Messages)
	}

	writeCommand(t, receiverConn, map[string]interface{}{
		"requestId": "mvp-ws-read",
		"command":   gateway.CommandMarkConversationRead,
		"payload": map[string]interface{}{
			"conversationId": sent.Message.ConversationID,
			"hasReadSeq":     sent.Message.Seq,
		},
	})
	readResp := readWSResponse(t, receiverConn)
	if readResp.Type != gateway.CommandMarkConversationRead || readResp.Status != gateway.AckStatusOK {
		t.Fatalf("unexpected mark-read ack: %+v", readResp)
	}
	var read gateway.MarkConversationReadCommandResponse
	decodeRaw(t, readResp.Data, &read)
	if read.HasReadSeq != sent.Message.Seq || read.UnreadCount != 0 {
		t.Fatalf("unexpected websocket read state: %+v", read)
	}
}
