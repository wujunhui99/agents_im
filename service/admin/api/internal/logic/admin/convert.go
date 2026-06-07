package admin

import (
	"github.com/wujunhui99/agents_im/pkg/apperror"
	"github.com/wujunhui99/agents_im/service/admin/api/internal/types"
	adminpb "github.com/wujunhui99/agents_im/service/admin/rpc/admin"
)

// admin-api 是纯 BFF：把 admin-rpc 返回的 pb 转成对外 HTTP 契约类型（internal/types）。

var (
	codeOK    = string(apperror.CodeOK)
	messageOK = "ok"
)

func adminDashboardData(data *adminpb.DashboardResponse) types.AdminDashboardData {
	return types.AdminDashboardData{
		Totals:              adminDashboardTotals(data.GetTotals()),
		RecentTraces:        adminTraces(data.GetRecentTraces()),
		RecentConversations: adminConversations(data.GetRecentConversations()),
	}
}

func adminDashboardTotals(t *adminpb.AdminDashboardTotals) types.AdminDashboardTotals {
	return types.AdminDashboardTotals{
		Users:         t.GetUsers(),
		Conversations: t.GetConversations(),
		Messages:      t.GetMessages(),
		AIRuns:        t.GetAiRuns(),
		FailedAIRuns:  t.GetFailedAiRuns(),
	}
}

func adminMessages(messages []*adminpb.AdminMessage) []types.AdminMessage {
	out := make([]types.AdminMessage, 0, len(messages))
	for _, message := range messages {
		out = append(out, adminMessage(message))
	}
	return out
}

func adminMessage(message *adminpb.AdminMessage) types.AdminMessage {
	return types.AdminMessage{
		ServerMsgID:           message.GetServerMsgId(),
		ClientMsgID:           message.GetClientMsgId(),
		ConversationID:        message.GetConversationId(),
		Seq:                   message.GetSeq(),
		SenderID:              message.GetSenderId(),
		ReceiverID:            message.GetReceiverId(),
		GroupID:               message.GetGroupId(),
		ChatType:              message.GetChatType(),
		ContentType:           message.GetContentType(),
		Content:               message.GetContent(),
		MessageOrigin:         message.GetMessageOrigin(),
		AgentAccountID:        message.GetAgentAccountId(),
		TriggerServerMsgID:    message.GetTriggerServerMsgId(),
		AgentRunID:            message.GetAgentRunId(),
		AllowRecursiveTrigger: message.GetAllowRecursiveTrigger(),
		SendTime:              message.GetSendTime(),
		CreatedAt:             message.GetCreatedAt(),
	}
}

func adminMessagePtr(message *adminpb.AdminMessage) *types.AdminMessage {
	if message == nil {
		return nil
	}
	view := adminMessage(message)
	return &view
}

func adminUsers(users []*adminpb.AdminUser) []types.AdminUser {
	out := make([]types.AdminUser, 0, len(users))
	for _, user := range users {
		out = append(out, adminUser(user))
	}
	return out
}

func adminUser(user *adminpb.AdminUser) types.AdminUser {
	return types.AdminUser{
		UserID:        user.GetUserId(),
		AccountID:     user.GetAccountId(),
		Identifier:    user.GetIdentifier(),
		DisplayName:   user.GetDisplayName(),
		Name:          user.GetName(),
		Gender:        user.GetGender(),
		BirthDate:     user.GetBirthDate(),
		Region:        user.GetRegion(),
		AccountType:   user.GetAccountType(),
		AvatarMediaID: user.GetAvatarMediaId(),
		AvatarURL:     user.GetAvatarUrl(),
		CreatedAt:     user.GetCreatedAt(),
		UpdatedAt:     user.GetUpdatedAt(),
	}
}

func adminUserPtr(user *adminpb.AdminUser) *types.AdminUser {
	if user == nil {
		return nil
	}
	view := adminUser(user)
	return &view
}

func adminFriends(friends []*adminpb.AdminFriend) []types.AdminFriend {
	out := make([]types.AdminFriend, 0, len(friends))
	for _, friend := range friends {
		out = append(out, types.AdminFriend{
			UserID:    friend.GetUserId(),
			FriendID:  friend.GetFriendId(),
			Status:    friend.GetStatus(),
			IsFriend:  friend.GetIsFriend(),
			Friend:    adminUserPtr(friend.GetFriend()),
			CreatedAt: friend.GetCreatedAt(),
			UpdatedAt: friend.GetUpdatedAt(),
		})
	}
	return out
}

func adminConversations(conversations []*adminpb.AdminConversation) []types.AdminConversation {
	out := make([]types.AdminConversation, 0, len(conversations))
	for _, conversation := range conversations {
		out = append(out, types.AdminConversation{
			ConversationID: conversation.GetConversationId(),
			MaxSeq:         conversation.GetMaxSeq(),
			HasReadSeq:     conversation.GetHasReadSeq(),
			UnreadCount:    conversation.GetUnreadCount(),
			MaxSeqTime:     conversation.GetMaxSeqTime(),
			LastMessage:    adminMessagePtr(conversation.GetLastMessage()),
		})
	}
	return out
}

func adminTraces(traces []*adminpb.AdminLLMTrace) []types.AdminLLMTrace {
	out := make([]types.AdminLLMTrace, 0, len(traces))
	for _, trace := range traces {
		out = append(out, adminTrace(trace))
	}
	return out
}

func adminTrace(trace *adminpb.AdminLLMTrace) types.AdminLLMTrace {
	return types.AdminLLMTrace{
		TraceID:           trace.GetTraceId(),
		TraceURL:          trace.GetTraceUrl(),
		RunID:             trace.GetRunId(),
		AgentID:           trace.GetAgentId(),
		ConversationID:    trace.GetConversationId(),
		TriggerMessageID:  trace.GetTriggerMessageId(),
		ResponseMessageID: trace.GetResponseMessageId(),
		RequestingUserID:  trace.GetRequestingUserId(),
		Status:            trace.GetStatus(),
		Provider:          trace.GetProvider(),
		Model:             trace.GetModel(),
		PromptHash:        trace.GetPromptHash(),
		PromptVersion:     trace.GetPromptVersion(),
		LatencyMs:         trace.GetLatencyMs(),
		TotalTokens:       trace.GetTotalTokens(),
		ErrorCode:         trace.GetErrorCode(),
		ErrorMessage:      trace.GetErrorMessage(),
		StartedAt:         trace.GetStartedAt(),
		FinishedAt:        trace.GetFinishedAt(),
		CreatedAt:         trace.GetCreatedAt(),
	}
}

func adminToolCalls(calls []*adminpb.AdminAgentToolCall) []types.AdminAgentToolCall {
	out := make([]types.AdminAgentToolCall, 0, len(calls))
	for _, call := range calls {
		out = append(out, types.AdminAgentToolCall{
			ToolCallID:   call.GetToolCallId(),
			RunID:        call.GetRunId(),
			ToolName:     call.GetToolName(),
			Status:       call.GetStatus(),
			DurationMs:   call.GetDurationMs(),
			ErrorCode:    call.GetErrorCode(),
			ErrorMessage: call.GetErrorMessage(),
			StartedAt:    call.GetStartedAt(),
			FinishedAt:   call.GetFinishedAt(),
			CreatedAt:    call.GetCreatedAt(),
		})
	}
	return out
}

func adminFileReads(reads []*adminpb.AdminAgentFileRead) []types.AdminAgentFileRead {
	out := make([]types.AdminAgentFileRead, 0, len(reads))
	for _, read := range reads {
		out = append(out, types.AdminAgentFileRead{
			FileReadID:   read.GetFileReadId(),
			RunID:        read.GetRunId(),
			SkillID:      read.GetSkillId(),
			FileID:       read.GetFileId(),
			Status:       read.GetStatus(),
			ByteCount:    read.GetByteCount(),
			ErrorCode:    read.GetErrorCode(),
			ErrorMessage: read.GetErrorMessage(),
			StartedAt:    read.GetStartedAt(),
			FinishedAt:   read.GetFinishedAt(),
			CreatedAt:    read.GetCreatedAt(),
		})
	}
	return out
}

func adminPythonExecs(execs []*adminpb.AdminAgentPythonExec) []types.AdminAgentPythonExec {
	out := make([]types.AdminAgentPythonExec, 0, len(execs))
	for _, exec := range execs {
		out = append(out, types.AdminAgentPythonExec{
			PythonExecID: exec.GetPythonExecId(),
			RunID:        exec.GetRunId(),
			Status:       exec.GetStatus(),
			ErrorCode:    exec.GetErrorCode(),
			ErrorMessage: exec.GetErrorMessage(),
			StartedAt:    exec.GetStartedAt(),
			FinishedAt:   exec.GetFinishedAt(),
			CreatedAt:    exec.GetCreatedAt(),
		})
	}
	return out
}

func adminFeedbackItems(items []*adminpb.AdminFeedback) []types.AdminFeedback {
	out := make([]types.AdminFeedback, 0, len(items))
	for _, item := range items {
		out = append(out, adminFeedback(item))
	}
	return out
}

func adminFeedback(item *adminpb.AdminFeedback) types.AdminFeedback {
	var clientMeta map[string]any
	if item.GetClientMeta() != nil {
		clientMeta = item.GetClientMeta().AsMap()
	}
	return types.AdminFeedback{
		FeedbackID: item.GetFeedbackId(),
		UserID:     item.GetUserId(),
		Category:   item.GetCategory(),
		Status:     item.GetStatus(),
		Title:      item.GetTitle(),
		Content:    item.GetContent(),
		Contact:    item.GetContact(),
		PageURL:    item.GetPageUrl(),
		UserAgent:  item.GetUserAgent(),
		ClientMeta: clientMeta,
		AdminNote:  item.GetAdminNote(),
		CreatedAt:  item.GetCreatedAt(),
		UpdatedAt:  item.GetUpdatedAt(),
	}
}

func adminTaskReports(items []*adminpb.AdminTaskReport) []types.AdminTaskReport {
	out := make([]types.AdminTaskReport, 0, len(items))
	for _, item := range items {
		out = append(out, adminTaskReport(item))
	}
	return out
}

func adminTaskReport(report *adminpb.AdminTaskReport) types.AdminTaskReport {
	return types.AdminTaskReport{
		TaskID:                  report.GetTaskId(),
		Agent:                   report.GetAgent(),
		CodexSessionID:          report.GetCodexSessionId(),
		IssueNumber:             report.GetIssueNumber(),
		IssueURL:                report.GetIssueUrl(),
		Repo:                    report.GetRepo(),
		Branch:                  report.GetBranch(),
		Worktree:                report.GetWorktree(),
		Commit:                  report.GetCommit(),
		Outcome:                 report.GetOutcome(),
		StartedAt:               report.GetStartedAt(),
		EndedAt:                 report.GetEndedAt(),
		DurationSeconds:         report.GetDurationSeconds(),
		TokensUsed:              report.GetTokensUsed(),
		PRURL:                   report.GetPrUrl(),
		Evidence:                report.GetEvidence(),
		Blockers:                report.GetBlockers(),
		MajorTimeSinks:          report.GetMajorTimeSinks(),
		WouldMorePermissionHelp: report.GetWouldMorePermissionHelp(),
		CandidatePermissions:    report.GetCandidatePermissions(),
		PermissionReason:        report.GetPermissionReason(),
		PitfallsOrLessons:       report.GetPitfallsOrLessons(),
		Notes:                   report.GetNotes(),
		RecordedAt:              report.GetRecordedAt(),
	}
}
