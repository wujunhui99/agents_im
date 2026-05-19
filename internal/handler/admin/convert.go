package admin

import (
	"github.com/wujunhui99/agents_im/internal/apperror"
	business "github.com/wujunhui99/agents_im/internal/logic"
	"github.com/wujunhui99/agents_im/internal/types"
)

func adminDashboardResp(data business.AdminDashboardResponse) *types.AdminDashboardResp {
	return &types.AdminDashboardResp{
		Code:    string(apperror.CodeOK),
		Message: "ok",
		Data: types.AdminDashboardData{
			Totals:              adminDashboardTotals(data.Totals),
			RecentTraces:        adminTraces(data.RecentTraces),
			RecentConversations: adminConversations(data.RecentConversations),
		},
	}
}

func adminConversationMessagesResp(data business.AdminConversationMessagesResponse) *types.AdminConversationMessagesResp {
	return &types.AdminConversationMessagesResp{
		Code:    string(apperror.CodeOK),
		Message: "ok",
		Data: types.AdminConversationMessagesData{
			ConversationID: data.ConversationID,
			Messages:       adminMessages(data.Messages),
			IsEnd:          data.IsEnd,
			NextSeq:        data.NextSeq,
		},
	}
}

func adminUserSearchResp(data business.AdminUserSearchResponse) *types.AdminUserSearchResp {
	return &types.AdminUserSearchResp{
		Code:    string(apperror.CodeOK),
		Message: "ok",
		Data:    types.AdminUserSearchData{Users: adminUsers(data.Users)},
	}
}

func adminUserDetailResp(data business.AdminUserDetailResponse) *types.AdminUserDetailResp {
	return &types.AdminUserDetailResp{
		Code:    string(apperror.CodeOK),
		Message: "ok",
		Data:    types.AdminUserDetailData{User: adminUser(data.User)},
	}
}

func adminUserFriendsResp(data business.AdminUserFriendsResponse) *types.AdminUserFriendsResp {
	return &types.AdminUserFriendsResp{
		Code:    string(apperror.CodeOK),
		Message: "ok",
		Data:    types.AdminUserFriendsData{Friends: adminFriends(data.Friends)},
	}
}

func adminUserConversationsResp(data business.AdminUserConversationsResponse) *types.AdminUserConversationsResp {
	return &types.AdminUserConversationsResp{
		Code:    string(apperror.CodeOK),
		Message: "ok",
		Data:    types.AdminUserConversationsData{Conversations: adminConversations(data.Conversations)},
	}
}

func adminTraceListResp(data business.AdminLLMTraceListResponse) *types.AdminLLMTraceListResp {
	return &types.AdminLLMTraceListResp{
		Code:    string(apperror.CodeOK),
		Message: "ok",
		Data:    types.AdminLLMTraceListData{Traces: adminTraces(data.Traces)},
	}
}

func adminTraceDetailResp(data business.AdminLLMTraceDetailResponse) *types.AdminLLMTraceDetailResp {
	return &types.AdminLLMTraceDetailResp{
		Code:    string(apperror.CodeOK),
		Message: "ok",
		Data: types.AdminLLMTraceDetailData{
			Trace:       adminTrace(data.Trace),
			ToolCalls:   adminToolCalls(data.ToolCalls),
			FileReads:   adminFileReads(data.FileReads),
			PythonExecs: adminPythonExecs(data.PythonExecs),
		},
	}
}

func adminDashboardTotals(t business.AdminDashboardTotals) types.AdminDashboardTotals {
	return types.AdminDashboardTotals{
		Users:         t.Users,
		Conversations: t.Conversations,
		Messages:      t.Messages,
		AIRuns:        t.AIRuns,
		FailedAIRuns:  t.FailedAIRuns,
	}
}

func adminMessages(messages []business.AdminMessage) []types.AdminMessage {
	out := make([]types.AdminMessage, 0, len(messages))
	for _, message := range messages {
		out = append(out, adminMessage(message))
	}
	return out
}

func adminMessage(message business.AdminMessage) types.AdminMessage {
	return types.AdminMessage{
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

func adminUsers(users []business.AdminUser) []types.AdminUser {
	out := make([]types.AdminUser, 0, len(users))
	for _, user := range users {
		out = append(out, adminUser(user))
	}
	return out
}

func adminUser(user business.AdminUser) types.AdminUser {
	return types.AdminUser{
		UserID:        user.UserID,
		AccountID:     user.AccountID,
		Identifier:    user.Identifier,
		DisplayName:   user.DisplayName,
		Name:          user.Name,
		Gender:        user.Gender,
		BirthDate:     user.BirthDate,
		Region:        user.Region,
		AccountType:   user.AccountType,
		AvatarMediaID: user.AvatarMediaID,
		AvatarURL:     user.AvatarURL,
		CreatedAt:     user.CreatedAt,
		UpdatedAt:     user.UpdatedAt,
	}
}

func adminFriends(friends []business.AdminFriend) []types.AdminFriend {
	out := make([]types.AdminFriend, 0, len(friends))
	for _, friend := range friends {
		view := types.AdminFriend{
			UserID:    friend.UserID,
			FriendID:  friend.FriendID,
			Status:    friend.Status,
			IsFriend:  friend.IsFriend,
			CreatedAt: friend.CreatedAt,
			UpdatedAt: friend.UpdatedAt,
		}
		if friend.Friend != nil {
			profile := adminUser(*friend.Friend)
			view.Friend = &profile
		}
		out = append(out, view)
	}
	return out
}

func adminConversations(conversations []business.AdminConversation) []types.AdminConversation {
	out := make([]types.AdminConversation, 0, len(conversations))
	for _, conversation := range conversations {
		view := types.AdminConversation{
			ConversationID: conversation.ConversationID,
			MaxSeq:         conversation.MaxSeq,
			HasReadSeq:     conversation.HasReadSeq,
			UnreadCount:    conversation.UnreadCount,
			MaxSeqTime:     conversation.MaxSeqTime,
		}
		if conversation.LastMessage != nil {
			lastMessage := adminMessage(*conversation.LastMessage)
			view.LastMessage = &lastMessage
		}
		out = append(out, view)
	}
	return out
}

func adminTraces(traces []business.AdminLLMTrace) []types.AdminLLMTrace {
	out := make([]types.AdminLLMTrace, 0, len(traces))
	for _, trace := range traces {
		out = append(out, adminTrace(trace))
	}
	return out
}

func adminTrace(trace business.AdminLLMTrace) types.AdminLLMTrace {
	return types.AdminLLMTrace{
		TraceID:           trace.TraceID,
		RunID:             trace.RunID,
		AgentID:           trace.AgentID,
		ConversationID:    trace.ConversationID,
		TriggerMessageID:  trace.TriggerMessageID,
		ResponseMessageID: trace.ResponseMessageID,
		RequestingUserID:  trace.RequestingUserID,
		Status:            trace.Status,
		Provider:          trace.Provider,
		Model:             trace.Model,
		PromptHash:        trace.PromptHash,
		PromptVersion:     trace.PromptVersion,
		LatencyMs:         trace.LatencyMs,
		TotalTokens:       trace.TotalTokens,
		ErrorCode:         trace.ErrorCode,
		ErrorMessage:      trace.ErrorMessage,
		StartedAt:         trace.StartedAt,
		FinishedAt:        trace.FinishedAt,
		CreatedAt:         trace.CreatedAt,
	}
}

func adminToolCalls(calls []business.AdminAgentToolCall) []types.AdminAgentToolCall {
	out := make([]types.AdminAgentToolCall, 0, len(calls))
	for _, call := range calls {
		out = append(out, types.AdminAgentToolCall{
			ToolCallID:   call.ToolCallID,
			RunID:        call.RunID,
			ToolName:     call.ToolName,
			Status:       call.Status,
			DurationMs:   call.DurationMs,
			ErrorCode:    call.ErrorCode,
			ErrorMessage: call.ErrorMessage,
			StartedAt:    call.StartedAt,
			FinishedAt:   call.FinishedAt,
			CreatedAt:    call.CreatedAt,
		})
	}
	return out
}

func adminFileReads(reads []business.AdminAgentFileRead) []types.AdminAgentFileRead {
	out := make([]types.AdminAgentFileRead, 0, len(reads))
	for _, read := range reads {
		out = append(out, types.AdminAgentFileRead{
			FileReadID:   read.FileReadID,
			RunID:        read.RunID,
			SkillID:      read.SkillID,
			FileID:       read.FileID,
			Status:       read.Status,
			ByteCount:    read.ByteCount,
			ErrorCode:    read.ErrorCode,
			ErrorMessage: read.ErrorMessage,
			StartedAt:    read.StartedAt,
			FinishedAt:   read.FinishedAt,
			CreatedAt:    read.CreatedAt,
		})
	}
	return out
}

func adminPythonExecs(execs []business.AdminAgentPythonExec) []types.AdminAgentPythonExec {
	out := make([]types.AdminAgentPythonExec, 0, len(execs))
	for _, exec := range execs {
		out = append(out, types.AdminAgentPythonExec{
			PythonExecID: exec.PythonExecID,
			RunID:        exec.RunID,
			Status:       exec.Status,
			ErrorCode:    exec.ErrorCode,
			ErrorMessage: exec.ErrorMessage,
			StartedAt:    exec.StartedAt,
			FinishedAt:   exec.FinishedAt,
			CreatedAt:    exec.CreatedAt,
		})
	}
	return out
}
