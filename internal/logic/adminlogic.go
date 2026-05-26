package logic

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/wujunhui99/agents_im/internal/apperror"
	"github.com/wujunhui99/agents_im/internal/domain/agentaudit"
	"github.com/wujunhui99/agents_im/internal/model"
	"github.com/wujunhui99/agents_im/internal/observability"
	"github.com/wujunhui99/agents_im/internal/repository"
)

type AdminLogicConfig struct {
	Accounts    repository.AdminAccountRepository
	Friends     repository.FriendshipRepository
	Messages    repository.AdminMessageRepository
	AgentAudits repository.AdminAgentAuditRepository
	Feedback    repository.FeedbackRepository
	TaskReports repository.TaskReportRepository
}

type AdminLogic struct {
	accounts    repository.AdminAccountRepository
	friends     repository.FriendshipRepository
	messages    repository.AdminMessageRepository
	agentAudits repository.AdminAgentAuditRepository
	feedback    repository.FeedbackRepository
	taskReports repository.TaskReportRepository
}

type AdminDashboardRequest struct {
	Limit int
}

type AdminDashboardResponse struct {
	Totals              AdminDashboardTotals `json:"totals"`
	RecentTraces        []AdminLLMTrace      `json:"recentTraces"`
	RecentConversations []AdminConversation  `json:"recentConversations"`
}

type AdminDashboardTotals struct {
	Users         int64 `json:"users"`
	Conversations int64 `json:"conversations"`
	Messages      int64 `json:"messages"`
	AIRuns        int64 `json:"aiRuns"`
	FailedAIRuns  int64 `json:"failedAiRuns"`
}

type AdminConversationMessagesRequest struct {
	ConversationID string
	FromSeq        int64
	ToSeq          int64
	Limit          int
	Order          string
}

type AdminConversationMessagesResponse struct {
	ConversationID string         `json:"conversationId"`
	Messages       []AdminMessage `json:"messages"`
	IsEnd          bool           `json:"isEnd"`
	NextSeq        int64          `json:"nextSeq"`
}

type AdminMessage struct {
	ServerMsgID           string `json:"serverMsgId"`
	ClientMsgID           string `json:"clientMsgId"`
	ConversationID        string `json:"conversationId"`
	Seq                   int64  `json:"seq"`
	SenderID              string `json:"senderId"`
	ReceiverID            string `json:"receiverId,omitempty"`
	GroupID               string `json:"groupId,omitempty"`
	ChatType              string `json:"chatType"`
	ContentType           string `json:"contentType"`
	Content               string `json:"content"`
	MessageOrigin         string `json:"messageOrigin"`
	AgentAccountID        string `json:"agentAccountId,omitempty"`
	TriggerServerMsgID    string `json:"triggerServerMsgId,omitempty"`
	AgentRunID            string `json:"agentRunId,omitempty"`
	AllowRecursiveTrigger bool   `json:"allowRecursiveTrigger,omitempty"`
	SendTime              int64  `json:"sendTime"`
	CreatedAt             int64  `json:"createdAt"`
}

type AdminUserSearchRequest struct {
	Query string
	Limit int
}

type AdminUserSearchResponse struct {
	Users []AdminUser `json:"users"`
}

type AdminUserDetailRequest struct {
	AccountID string
}

type AdminUserDetailResponse struct {
	User AdminUser `json:"user"`
}

type AdminUserFriendsRequest struct {
	AccountID string
}

type AdminUserFriendsResponse struct {
	Friends []AdminFriend `json:"friends"`
}

type AdminUserConversationsRequest struct {
	AccountID string
}

type AdminUserConversationsResponse struct {
	Conversations []AdminConversation `json:"conversations"`
}

type AdminUser struct {
	UserID        string `json:"userId"`
	AccountID     string `json:"accountId,omitempty"`
	Identifier    string `json:"identifier"`
	DisplayName   string `json:"displayName"`
	Name          string `json:"name"`
	Gender        string `json:"gender"`
	BirthDate     string `json:"birthDate"`
	Region        string `json:"region"`
	AccountType   string `json:"accountType"`
	AvatarMediaID string `json:"avatarMediaId,omitempty"`
	AvatarURL     string `json:"avatarUrl,omitempty"`
	CreatedAt     string `json:"createdAt"`
	UpdatedAt     string `json:"updatedAt"`
}

type AdminFriend struct {
	UserID    string     `json:"userId"`
	FriendID  string     `json:"friendId"`
	Status    string     `json:"status"`
	IsFriend  bool       `json:"isFriend"`
	Friend    *AdminUser `json:"friend,omitempty"`
	CreatedAt string     `json:"createdAt"`
	UpdatedAt string     `json:"updatedAt"`
}

type AdminConversation struct {
	ConversationID string        `json:"conversationId"`
	MaxSeq         int64         `json:"maxSeq"`
	HasReadSeq     int64         `json:"hasReadSeq,omitempty"`
	UnreadCount    int64         `json:"unreadCount,omitempty"`
	MaxSeqTime     int64         `json:"maxSeqTime,omitempty"`
	LastMessage    *AdminMessage `json:"lastMessage,omitempty"`
}

type AdminFeedbackListRequest struct {
	Status string
	Limit  int
	Offset int
}

type AdminFeedbackDetailRequest struct {
	FeedbackID string
}

type AdminFeedbackUpdateRequest struct {
	FeedbackID string
	Status     string
	AdminNote  string
}

type AdminFeedbackListResponse struct {
	Items []AdminFeedback `json:"items"`
}

type AdminFeedbackDetailResponse struct {
	Feedback AdminFeedback `json:"feedback"`
}

type AdminFeedbackUpdateResponse struct {
	Feedback AdminFeedback `json:"feedback"`
}

type AdminFeedback struct {
	FeedbackID string         `json:"feedbackId"`
	UserID     string         `json:"userId"`
	Category   string         `json:"category"`
	Status     string         `json:"status"`
	Title      string         `json:"title"`
	Content    string         `json:"content"`
	Contact    string         `json:"contact,omitempty"`
	PageURL    string         `json:"pageUrl,omitempty"`
	UserAgent  string         `json:"userAgent,omitempty"`
	ClientMeta map[string]any `json:"clientMeta,omitempty"`
	AdminNote  string         `json:"adminNote,omitempty"`
	CreatedAt  string         `json:"createdAt"`
	UpdatedAt  string         `json:"updatedAt"`
}

type AdminTaskReportListRequest struct {
	Outcome string
	Limit   int
	Offset  int
}

type AdminTaskReportUpsertRequest struct {
	Report AdminTaskReport
}

type AdminTaskReportListResponse struct {
	Items []AdminTaskReport `json:"items"`
}

type AdminTaskReportDetailResponse struct {
	Report AdminTaskReport `json:"report"`
}

type AdminTaskReport struct {
	TaskID                  string   `json:"taskId"`
	Agent                   string   `json:"agent"`
	CodexSessionID          string   `json:"codexSessionId,omitempty"`
	IssueNumber             int64    `json:"issueNumber,omitempty"`
	IssueURL                string   `json:"issueUrl,omitempty"`
	Repo                    string   `json:"repo"`
	Branch                  string   `json:"branch,omitempty"`
	Worktree                string   `json:"worktree,omitempty"`
	Commit                  string   `json:"commit,omitempty"`
	Outcome                 string   `json:"outcome"`
	StartedAt               string   `json:"startedAt,omitempty"`
	EndedAt                 string   `json:"endedAt,omitempty"`
	DurationSeconds         int64    `json:"durationSeconds,omitempty"`
	TokensUsed              int64    `json:"tokensUsed,omitempty"`
	PRURL                   string   `json:"prUrl,omitempty"`
	Evidence                []string `json:"evidence"`
	Blockers                []string `json:"blockers"`
	MajorTimeSinks          []string `json:"majorTimeSinks"`
	WouldMorePermissionHelp string   `json:"wouldMorePermissionHelp,omitempty"`
	CandidatePermissions    []string `json:"candidatePermissions"`
	PermissionReason        string   `json:"permissionReason,omitempty"`
	PitfallsOrLessons       []string `json:"pitfallsOrLessons"`
	Notes                   string   `json:"notes,omitempty"`
	RecordedAt              string   `json:"recordedAt"`
}

type AdminLLMTraceListRequest struct {
	Status string
	Limit  int
	Offset int
}

type AdminLLMTraceListResponse struct {
	Traces []AdminLLMTrace `json:"traces"`
}

type AdminLLMTraceDetailRequest struct {
	TraceID string
}

type AdminLLMTraceDetailResponse struct {
	Trace       AdminLLMTrace          `json:"trace"`
	ToolCalls   []AdminAgentToolCall   `json:"toolCalls"`
	FileReads   []AdminAgentFileRead   `json:"fileReads"`
	PythonExecs []AdminAgentPythonExec `json:"pythonExecs"`
}

type AdminLLMTrace struct {
	TraceID           string `json:"traceId"`
	TraceURL          string `json:"traceUrl,omitempty"`
	RunID             string `json:"runId"`
	AgentID           string `json:"agentId"`
	ConversationID    string `json:"conversationId,omitempty"`
	TriggerMessageID  string `json:"triggerMessageId,omitempty"`
	ResponseMessageID string `json:"responseMessageId,omitempty"`
	RequestingUserID  string `json:"requestingUserId,omitempty"`
	Status            string `json:"status"`
	Provider          string `json:"provider,omitempty"`
	Model             string `json:"model,omitempty"`
	PromptHash        string `json:"promptHash,omitempty"`
	PromptVersion     string `json:"promptVersion,omitempty"`
	LatencyMs         int64  `json:"latencyMs,omitempty"`
	TotalTokens       int64  `json:"totalTokens,omitempty"`
	ErrorCode         string `json:"errorCode,omitempty"`
	ErrorMessage      string `json:"errorMessage,omitempty"`
	StartedAt         string `json:"startedAt,omitempty"`
	FinishedAt        string `json:"finishedAt,omitempty"`
	CreatedAt         string `json:"createdAt,omitempty"`
}

type AdminAgentToolCall struct {
	ToolCallID   string `json:"toolCallId"`
	RunID        string `json:"runId"`
	ToolName     string `json:"toolName"`
	Status       string `json:"status"`
	DurationMs   int64  `json:"durationMs,omitempty"`
	ErrorCode    string `json:"errorCode,omitempty"`
	ErrorMessage string `json:"errorMessage,omitempty"`
	StartedAt    string `json:"startedAt,omitempty"`
	FinishedAt   string `json:"finishedAt,omitempty"`
	CreatedAt    string `json:"createdAt,omitempty"`
}

type AdminAgentFileRead struct {
	FileReadID   string `json:"fileReadId"`
	RunID        string `json:"runId"`
	SkillID      string `json:"skillId,omitempty"`
	FileID       string `json:"fileId,omitempty"`
	Status       string `json:"status"`
	ByteCount    int64  `json:"byteCount,omitempty"`
	ErrorCode    string `json:"errorCode,omitempty"`
	ErrorMessage string `json:"errorMessage,omitempty"`
	StartedAt    string `json:"startedAt,omitempty"`
	FinishedAt   string `json:"finishedAt,omitempty"`
	CreatedAt    string `json:"createdAt,omitempty"`
}

type AdminAgentPythonExec struct {
	PythonExecID string `json:"pythonExecId"`
	RunID        string `json:"runId"`
	Status       string `json:"status"`
	ErrorCode    string `json:"errorCode,omitempty"`
	ErrorMessage string `json:"errorMessage,omitempty"`
	StartedAt    string `json:"startedAt,omitempty"`
	FinishedAt   string `json:"finishedAt,omitempty"`
	CreatedAt    string `json:"createdAt,omitempty"`
}

func NewAdminLogic(config AdminLogicConfig) *AdminLogic {
	return &AdminLogic{
		accounts:    config.Accounts,
		friends:     config.Friends,
		messages:    config.Messages,
		agentAudits: config.AgentAudits,
		feedback:    config.Feedback,
		taskReports: config.TaskReports,
	}
}

func (l *AdminLogic) GetDashboard(ctx context.Context, req AdminDashboardRequest) (AdminDashboardResponse, error) {
	if l == nil || l.accounts == nil || l.messages == nil || l.agentAudits == nil {
		return AdminDashboardResponse{}, apperror.Internal("admin repositories are not configured")
	}
	users, err := l.accounts.CountAccounts(ctx)
	if err != nil {
		return AdminDashboardResponse{}, err
	}
	conversations, err := l.messages.CountConversations(ctx)
	if err != nil {
		return AdminDashboardResponse{}, err
	}
	messages, err := l.messages.CountMessages(ctx)
	if err != nil {
		return AdminDashboardResponse{}, err
	}
	aiRuns, err := l.agentAudits.CountAgentRuns(ctx, "")
	if err != nil {
		return AdminDashboardResponse{}, err
	}
	failedRuns, err := l.agentAudits.CountAgentRuns(ctx, string(agentaudit.StatusFailed))
	if err != nil {
		return AdminDashboardResponse{}, err
	}
	limit := normalizeAdminLogicLimit(req.Limit, 10, 100)
	traceList, err := l.ListLLMTraces(ctx, AdminLLMTraceListRequest{Limit: limit})
	if err != nil {
		return AdminDashboardResponse{}, err
	}
	recentStates, err := l.messages.ListRecentConversationStates(ctx, limit)
	if err != nil {
		return AdminDashboardResponse{}, err
	}
	return AdminDashboardResponse{
		Totals: AdminDashboardTotals{
			Users:         users,
			Conversations: conversations,
			Messages:      messages,
			AIRuns:        aiRuns,
			FailedAIRuns:  failedRuns,
		},
		RecentTraces:        traceList.Traces,
		RecentConversations: adminConversationsFromStates(recentStates),
	}, nil
}

func (l *AdminLogic) GetConversationMessages(ctx context.Context, req AdminConversationMessagesRequest) (AdminConversationMessagesResponse, error) {
	if l == nil || l.messages == nil {
		return AdminConversationMessagesResponse{}, apperror.Internal("admin message repository is not configured")
	}
	conversationID, err := normalizeAdminConversationID(req.ConversationID)
	if err != nil {
		return AdminConversationMessagesResponse{}, err
	}
	limit := normalizeAdminLogicLimit(req.Limit, 50, 500)
	order := strings.ToLower(strings.TrimSpace(req.Order))
	if order == "" {
		order = repository.MessageStorageOrderAsc
	}
	messages, isEnd, nextSeq, err := l.messages.GetMessages(ctx, conversationID, req.FromSeq, req.ToSeq, limit, order)
	if err != nil {
		return AdminConversationMessagesResponse{}, err
	}
	return AdminConversationMessagesResponse{
		ConversationID: conversationID,
		Messages:       adminMessagesFromRepository(messages),
		IsEnd:          isEnd,
		NextSeq:        nextSeq,
	}, nil
}

func (l *AdminLogic) SearchUsers(ctx context.Context, req AdminUserSearchRequest) (AdminUserSearchResponse, error) {
	if l == nil || l.accounts == nil {
		return AdminUserSearchResponse{}, apperror.Internal("admin account repository is not configured")
	}
	users, err := l.accounts.SearchAccounts(ctx, repository.AccountSearchFilter{
		Query: strings.TrimSpace(req.Query),
		Limit: normalizeAdminLogicLimit(req.Limit, 20, 100),
	})
	if err != nil {
		return AdminUserSearchResponse{}, err
	}
	resp := AdminUserSearchResponse{Users: make([]AdminUser, 0, len(users))}
	for _, user := range users {
		resp.Users = append(resp.Users, adminUserFromModel(user))
	}
	return resp, nil
}

func (l *AdminLogic) GetUserDetail(ctx context.Context, req AdminUserDetailRequest) (AdminUserDetailResponse, error) {
	if l == nil || l.accounts == nil {
		return AdminUserDetailResponse{}, apperror.Internal("admin account repository is not configured")
	}
	accountID, err := normalizeAdminAccountID(req.AccountID)
	if err != nil {
		return AdminUserDetailResponse{}, err
	}
	user, err := l.accounts.GetByID(ctx, accountID)
	if err != nil {
		return AdminUserDetailResponse{}, err
	}
	return AdminUserDetailResponse{User: adminUserFromModel(user)}, nil
}

func (l *AdminLogic) GetUserFriends(ctx context.Context, req AdminUserFriendsRequest) (AdminUserFriendsResponse, error) {
	if l == nil || l.accounts == nil || l.friends == nil {
		return AdminUserFriendsResponse{}, apperror.Internal("admin friend repositories are not configured")
	}
	accountID, err := normalizeAdminAccountID(req.AccountID)
	if err != nil {
		return AdminUserFriendsResponse{}, err
	}
	if _, err := l.accounts.GetByID(ctx, accountID); err != nil {
		return AdminUserFriendsResponse{}, err
	}
	friendships, err := l.friends.ListFriends(ctx, accountID)
	if err != nil {
		return AdminUserFriendsResponse{}, err
	}
	resp := AdminUserFriendsResponse{Friends: make([]AdminFriend, 0, len(friendships))}
	for _, friendship := range friendships {
		view := AdminFriend{
			UserID:    friendship.UserID,
			FriendID:  friendship.FriendID,
			Status:    friendship.Status,
			IsFriend:  friendship.Status == model.FriendshipStatusAccepted,
			CreatedAt: formatAdminTime(friendship.CreatedAt),
			UpdatedAt: formatAdminTime(friendship.UpdatedAt),
		}
		if friend, err := l.accounts.GetByID(ctx, friendship.FriendID); err == nil {
			friendView := adminUserFromModel(friend)
			view.Friend = &friendView
		} else {
			return AdminUserFriendsResponse{}, err
		}
		resp.Friends = append(resp.Friends, view)
	}
	return resp, nil
}

func (l *AdminLogic) GetUserConversations(ctx context.Context, req AdminUserConversationsRequest) (AdminUserConversationsResponse, error) {
	if l == nil || l.accounts == nil || l.messages == nil {
		return AdminUserConversationsResponse{}, apperror.Internal("admin conversation repositories are not configured")
	}
	accountID, err := normalizeAdminAccountID(req.AccountID)
	if err != nil {
		return AdminUserConversationsResponse{}, err
	}
	if _, err := l.accounts.GetByID(ctx, accountID); err != nil {
		return AdminUserConversationsResponse{}, err
	}
	states, err := l.messages.GetConversationSeqStates(ctx, accountID, nil)
	if err != nil {
		return AdminUserConversationsResponse{}, err
	}
	return AdminUserConversationsResponse{Conversations: adminConversationsFromStates(states)}, nil
}

func (l *AdminLogic) ListLLMTraces(ctx context.Context, req AdminLLMTraceListRequest) (AdminLLMTraceListResponse, error) {
	if l == nil || l.agentAudits == nil {
		return AdminLLMTraceListResponse{}, apperror.Internal("admin agent audit repository is not configured")
	}
	runs, err := l.agentAudits.ListAgentRuns(ctx, repository.AgentRunFilter{
		Status: strings.TrimSpace(req.Status),
		Limit:  normalizeAdminLogicLimit(req.Limit, 20, 100),
		Offset: req.Offset,
	})
	if err != nil {
		return AdminLLMTraceListResponse{}, err
	}
	resp := AdminLLMTraceListResponse{Traces: make([]AdminLLMTrace, 0, len(runs))}
	for _, run := range runs {
		resp.Traces = append(resp.Traces, adminTraceFromRun(run))
	}
	return resp, nil
}

func (l *AdminLogic) GetLLMTraceDetail(ctx context.Context, req AdminLLMTraceDetailRequest) (AdminLLMTraceDetailResponse, error) {
	if l == nil || l.agentAudits == nil {
		return AdminLLMTraceDetailResponse{}, apperror.Internal("admin agent audit repository is not configured")
	}
	traceID := strings.TrimSpace(req.TraceID)
	if traceID == "" {
		return AdminLLMTraceDetailResponse{}, apperror.InvalidArgument("trace_id is required")
	}
	run, err := l.agentAudits.GetAgentRunByTraceID(ctx, traceID)
	if err != nil && apperror.From(err).Code == apperror.CodeNotFound {
		run, err = l.agentAudits.GetAgentRun(ctx, traceID)
	}
	if err != nil {
		return AdminLLMTraceDetailResponse{}, err
	}
	toolCalls, err := l.agentAudits.ListAgentToolCallsByRunID(ctx, run.RunID)
	if err != nil {
		return AdminLLMTraceDetailResponse{}, err
	}
	fileReads, err := l.agentAudits.ListAgentFileReadsByRunID(ctx, run.RunID)
	if err != nil {
		return AdminLLMTraceDetailResponse{}, err
	}
	pythonExecs, err := l.agentAudits.ListAgentPythonExecsByRunID(ctx, run.RunID)
	if err != nil {
		return AdminLLMTraceDetailResponse{}, err
	}
	return AdminLLMTraceDetailResponse{
		Trace:       adminTraceFromRun(run),
		ToolCalls:   adminToolCallsFromAudit(toolCalls),
		FileReads:   adminFileReadsFromAudit(fileReads),
		PythonExecs: adminPythonExecsFromAudit(pythonExecs),
	}, nil
}

func adminMessagesFromRepository(messages []repository.Message) []AdminMessage {
	out := make([]AdminMessage, 0, len(messages))
	for _, message := range messages {
		out = append(out, adminMessageFromRepository(message))
	}
	return out
}

func adminMessageFromRepository(message repository.Message) AdminMessage {
	return AdminMessage{
		ServerMsgID:           message.ServerMsgID,
		ClientMsgID:           message.ClientMsgID,
		ConversationID:        message.ConversationID,
		Seq:                   message.Seq,
		SenderID:              message.SenderID,
		ReceiverID:            message.ReceiverID,
		GroupID:               message.GroupID,
		ChatType:              message.ChatType,
		ContentType:           message.ContentType,
		Content:               sanitizeAdminText(message.Content),
		MessageOrigin:         message.MessageOrigin,
		AgentAccountID:        message.AgentAccountID,
		TriggerServerMsgID:    message.TriggerServerMsgID,
		AgentRunID:            message.AgentRunID,
		AllowRecursiveTrigger: message.AllowRecursiveTrigger,
		SendTime:              message.SendTime,
		CreatedAt:             message.CreatedAt,
	}
}

func adminUserFromModel(user model.User) AdminUser {
	user = user.Clone()
	return AdminUser{
		UserID:        user.UserID,
		AccountID:     user.AccountID,
		Identifier:    user.Identifier,
		DisplayName:   user.DisplayName,
		Name:          user.Name,
		Gender:        user.Gender,
		BirthDate:     user.BirthDate,
		Region:        user.Region,
		AccountType:   string(user.AccountType),
		AvatarMediaID: user.AvatarMediaID,
		AvatarURL:     user.AvatarURL,
		CreatedAt:     formatAdminTime(user.CreatedAt),
		UpdatedAt:     formatAdminTime(user.UpdatedAt),
	}
}

func adminConversationsFromStates(states []repository.ConversationSeqState) []AdminConversation {
	out := make([]AdminConversation, 0, len(states))
	for _, state := range states {
		conversation := AdminConversation{
			ConversationID: state.ConversationID,
			MaxSeq:         state.MaxSeq,
			HasReadSeq:     state.HasReadSeq,
			UnreadCount:    state.UnreadCount,
			MaxSeqTime:     state.MaxSeqTime,
		}
		if state.LastMessage != nil {
			lastMessage := adminMessageFromRepository(*state.LastMessage)
			conversation.LastMessage = &lastMessage
		}
		out = append(out, conversation)
	}
	return out
}

func adminTraceFromRun(run agentaudit.AgentRun) AdminLLMTrace {
	traceID := strings.TrimSpace(run.TraceID)
	if traceID == "" {
		traceID = run.RunID
	}
	return AdminLLMTrace{
		TraceID:           traceID,
		TraceURL:          observability.TraceUIURL(observability.TraceUIBaseURLFromEnv(), traceID),
		RunID:             run.RunID,
		AgentID:           run.AgentID,
		ConversationID:    run.ConversationID,
		TriggerMessageID:  run.TriggerMessageID,
		ResponseMessageID: run.OutputMessageID,
		RequestingUserID:  run.RequestingUserID,
		Status:            string(run.Status),
		Provider:          summaryString(run.InputSummary, "provider"),
		Model:             summaryString(run.InputSummary, "model"),
		PromptHash:        summaryString(run.InputSummary, "prompt_hash", "promptHash"),
		PromptVersion:     summaryString(run.InputSummary, "prompt_version", "promptVersion", "prompt_ver"),
		LatencyMs:         summaryInt64(run.OutputSummary, "latency_ms", "latencyMs"),
		TotalTokens:       summaryInt64(run.OutputSummary, "total_tokens", "totalTokens"),
		ErrorCode:         sanitizeAdminText(run.ErrorCode),
		ErrorMessage:      sanitizeAdminText(run.ErrorMessage),
		StartedAt:         formatAdminTime(run.StartedAt),
		FinishedAt:        formatAdminTime(run.FinishedAt),
		CreatedAt:         formatAdminTime(run.CreatedAt),
	}
}

func adminToolCallsFromAudit(calls []agentaudit.AgentToolCall) []AdminAgentToolCall {
	out := make([]AdminAgentToolCall, 0, len(calls))
	for _, call := range calls {
		out = append(out, AdminAgentToolCall{
			ToolCallID:   call.ToolCallID,
			RunID:        call.RunID,
			ToolName:     call.ToolName,
			Status:       string(call.Status),
			DurationMs:   call.DurationMs,
			ErrorCode:    sanitizeAdminText(call.ErrorCode),
			ErrorMessage: sanitizeAdminText(call.ErrorMessage),
			StartedAt:    formatAdminTime(call.StartedAt),
			FinishedAt:   formatAdminTime(call.FinishedAt),
			CreatedAt:    formatAdminTime(call.CreatedAt),
		})
	}
	return out
}

func adminFileReadsFromAudit(reads []agentaudit.AgentFileRead) []AdminAgentFileRead {
	out := make([]AdminAgentFileRead, 0, len(reads))
	for _, read := range reads {
		out = append(out, AdminAgentFileRead{
			FileReadID:   read.FileReadID,
			RunID:        read.RunID,
			SkillID:      read.SkillID,
			FileID:       read.FileID,
			Status:       string(read.Status),
			ByteCount:    read.ByteCount,
			ErrorCode:    sanitizeAdminText(read.ErrorCode),
			ErrorMessage: sanitizeAdminText(read.ErrorMessage),
			StartedAt:    formatAdminTime(read.StartedAt),
			FinishedAt:   formatAdminTime(read.FinishedAt),
			CreatedAt:    formatAdminTime(read.CreatedAt),
		})
	}
	return out
}

func adminPythonExecsFromAudit(execs []agentaudit.AgentPythonExec) []AdminAgentPythonExec {
	out := make([]AdminAgentPythonExec, 0, len(execs))
	for _, exec := range execs {
		out = append(out, AdminAgentPythonExec{
			PythonExecID: exec.PythonExecID,
			RunID:        exec.RunID,
			Status:       string(exec.Status),
			ErrorCode:    sanitizeAdminText(exec.ErrorCode),
			ErrorMessage: sanitizeAdminText(exec.ErrorMessage),
			StartedAt:    formatAdminTime(exec.StartedAt),
			FinishedAt:   formatAdminTime(exec.FinishedAt),
			CreatedAt:    formatAdminTime(exec.CreatedAt),
		})
	}
	return out
}

func (l *AdminLogic) ListFeedback(ctx context.Context, req AdminFeedbackListRequest) (AdminFeedbackListResponse, error) {
	if l == nil || l.feedback == nil {
		return AdminFeedbackListResponse{}, apperror.Internal("feedback repository is not configured")
	}
	var status model.FeedbackStatus
	if strings.TrimSpace(req.Status) != "" {
		parsed, ok := model.NormalizeFeedbackStatus(strings.TrimSpace(req.Status))
		if !ok {
			return AdminFeedbackListResponse{}, apperror.InvalidArgument("feedback status is invalid")
		}
		status = parsed
	}
	items, err := l.feedback.ListFeedback(ctx, repository.FeedbackListFilter{
		Status: status,
		Limit:  normalizeAdminLogicLimit(req.Limit, 50, 200),
		Offset: req.Offset,
	})
	if err != nil {
		return AdminFeedbackListResponse{}, err
	}
	resp := AdminFeedbackListResponse{Items: make([]AdminFeedback, 0, len(items))}
	for _, item := range items {
		resp.Items = append(resp.Items, adminFeedback(item))
	}
	return resp, nil
}

func (l *AdminLogic) GetFeedback(ctx context.Context, req AdminFeedbackDetailRequest) (AdminFeedbackDetailResponse, error) {
	if l == nil || l.feedback == nil {
		return AdminFeedbackDetailResponse{}, apperror.Internal("feedback repository is not configured")
	}
	feedbackID, err := normalizeAdminFeedbackID(req.FeedbackID)
	if err != nil {
		return AdminFeedbackDetailResponse{}, err
	}
	feedback, err := l.feedback.GetFeedback(ctx, feedbackID)
	if err != nil {
		return AdminFeedbackDetailResponse{}, err
	}
	return AdminFeedbackDetailResponse{Feedback: adminFeedback(feedback)}, nil
}

func (l *AdminLogic) UpdateFeedback(ctx context.Context, req AdminFeedbackUpdateRequest) (AdminFeedbackUpdateResponse, error) {
	if l == nil || l.feedback == nil {
		return AdminFeedbackUpdateResponse{}, apperror.Internal("feedback repository is not configured")
	}
	feedbackID, err := normalizeAdminFeedbackID(req.FeedbackID)
	if err != nil {
		return AdminFeedbackUpdateResponse{}, err
	}
	status, ok := model.NormalizeFeedbackStatus(strings.TrimSpace(req.Status))
	if !ok {
		return AdminFeedbackUpdateResponse{}, apperror.InvalidArgument("feedback status is invalid")
	}
	updated, err := l.feedback.UpdateFeedback(ctx, model.Feedback{
		FeedbackID: feedbackID,
		Status:     status,
		AdminNote:  strings.TrimSpace(req.AdminNote),
	})
	if err != nil {
		return AdminFeedbackUpdateResponse{}, err
	}
	return AdminFeedbackUpdateResponse{Feedback: adminFeedback(updated)}, nil
}

func adminFeedback(feedback model.Feedback) AdminFeedback {
	return AdminFeedback{
		FeedbackID: feedback.FeedbackID,
		UserID:     feedback.UserID,
		Category:   string(feedback.Category),
		Status:     string(feedback.Status),
		Title:      feedback.Title,
		Content:    feedback.Content,
		Contact:    feedback.Contact,
		PageURL:    feedback.PageURL,
		UserAgent:  feedback.UserAgent,
		ClientMeta: feedback.ClientMeta,
		AdminNote:  feedback.AdminNote,
		CreatedAt:  formatAdminTime(feedback.CreatedAt),
		UpdatedAt:  formatAdminTime(feedback.UpdatedAt),
	}
}

func (l *AdminLogic) ListTaskReports(ctx context.Context, req AdminTaskReportListRequest) (AdminTaskReportListResponse, error) {
	if l == nil || l.taskReports == nil {
		return AdminTaskReportListResponse{}, apperror.Internal("task report repository is not configured")
	}
	reports, err := l.taskReports.ListTaskReports(ctx, repository.TaskReportListFilter{
		Outcome: strings.TrimSpace(req.Outcome),
		Limit:   normalizeAdminLogicLimit(req.Limit, 50, 200),
		Offset:  req.Offset,
	})
	if err != nil {
		return AdminTaskReportListResponse{}, err
	}
	items := make([]AdminTaskReport, 0, len(reports))
	for _, report := range reports {
		items = append(items, adminTaskReportFromRepository(report))
	}
	return AdminTaskReportListResponse{Items: items}, nil
}

func (l *AdminLogic) UpsertTaskReport(ctx context.Context, req AdminTaskReportUpsertRequest) (AdminTaskReportDetailResponse, error) {
	if l == nil || l.taskReports == nil {
		return AdminTaskReportDetailResponse{}, apperror.Internal("task report repository is not configured")
	}
	report, err := l.taskReports.UpsertTaskReport(ctx, adminTaskReportToRepository(req.Report))
	if err != nil {
		return AdminTaskReportDetailResponse{}, err
	}
	return AdminTaskReportDetailResponse{Report: adminTaskReportFromRepository(report)}, nil
}

func adminTaskReportFromRepository(report repository.TaskReport) AdminTaskReport {
	return AdminTaskReport{
		TaskID:                  report.TaskID,
		Agent:                   report.Agent,
		CodexSessionID:          report.CodexSessionID,
		IssueNumber:             report.IssueNumber,
		IssueURL:                report.IssueURL,
		Repo:                    report.Repo,
		Branch:                  report.Branch,
		Worktree:                report.Worktree,
		Commit:                  report.Commit,
		Outcome:                 report.Outcome,
		StartedAt:               report.StartedAt,
		EndedAt:                 report.EndedAt,
		DurationSeconds:         report.DurationSeconds,
		TokensUsed:              report.TokensUsed,
		PRURL:                   report.PRURL,
		Evidence:                report.Evidence,
		Blockers:                report.Blockers,
		MajorTimeSinks:          report.MajorTimeSinks,
		WouldMorePermissionHelp: report.WouldMorePermissionHelp,
		CandidatePermissions:    report.CandidatePermissions,
		PermissionReason:        report.PermissionReason,
		PitfallsOrLessons:       report.PitfallsOrLessons,
		Notes:                   report.Notes,
		RecordedAt:              report.RecordedAt,
	}
}

func adminTaskReportToRepository(report AdminTaskReport) repository.TaskReport {
	return repository.TaskReport{
		TaskID:                  strings.TrimSpace(report.TaskID),
		Agent:                   strings.TrimSpace(report.Agent),
		CodexSessionID:          strings.TrimSpace(report.CodexSessionID),
		IssueNumber:             report.IssueNumber,
		IssueURL:                strings.TrimSpace(report.IssueURL),
		Repo:                    strings.TrimSpace(report.Repo),
		Branch:                  strings.TrimSpace(report.Branch),
		Worktree:                strings.TrimSpace(report.Worktree),
		Commit:                  strings.TrimSpace(report.Commit),
		Outcome:                 strings.TrimSpace(report.Outcome),
		StartedAt:               strings.TrimSpace(report.StartedAt),
		EndedAt:                 strings.TrimSpace(report.EndedAt),
		DurationSeconds:         report.DurationSeconds,
		TokensUsed:              report.TokensUsed,
		PRURL:                   strings.TrimSpace(report.PRURL),
		Evidence:                report.Evidence,
		Blockers:                report.Blockers,
		MajorTimeSinks:          report.MajorTimeSinks,
		WouldMorePermissionHelp: strings.TrimSpace(report.WouldMorePermissionHelp),
		CandidatePermissions:    report.CandidatePermissions,
		PermissionReason:        strings.TrimSpace(report.PermissionReason),
		PitfallsOrLessons:       report.PitfallsOrLessons,
		Notes:                   strings.TrimSpace(report.Notes),
		RecordedAt:              strings.TrimSpace(report.RecordedAt),
	}
}

func normalizeAdminFeedbackID(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", apperror.InvalidArgument("feedback_id is required")
	}
	if len([]rune(value)) > 128 {
		return "", apperror.InvalidArgument("feedback_id must be 128 characters or fewer")
	}
	if strings.Contains(value, "\x00") {
		return "", apperror.InvalidArgument("feedback_id cannot contain NUL")
	}
	return value, nil
}

func normalizeAdminConversationID(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", apperror.InvalidArgument("conversation_id is required")
	}
	if len([]rune(value)) > 256 {
		return "", apperror.InvalidArgument("conversation_id must be 256 characters or fewer")
	}
	if strings.Contains(value, "\x00") {
		return "", apperror.InvalidArgument("conversation_id cannot contain NUL")
	}
	return value, nil
}

func normalizeAdminAccountID(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", apperror.InvalidArgument("account_id is required")
	}
	if len([]rune(value)) > 128 {
		return "", apperror.InvalidArgument("account_id must be 128 characters or fewer")
	}
	if strings.Contains(value, "\x00") {
		return "", apperror.InvalidArgument("account_id cannot contain NUL")
	}
	return value, nil
}

func normalizeAdminLogicLimit(value int, fallback int, max int) int {
	if value <= 0 {
		value = fallback
	}
	if value > max {
		value = max
	}
	return value
}

func formatAdminTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.UTC().Format(time.RFC3339Nano)
}

func summaryString(summary agentaudit.Summary, keys ...string) string {
	for _, key := range keys {
		value, ok := summary[key]
		if !ok {
			continue
		}
		text := strings.TrimSpace(fmt.Sprint(value))
		if text != "" && text != agentaudit.RedactedValue {
			return sanitizeAdminText(text)
		}
	}
	return ""
}

func summaryInt64(summary agentaudit.Summary, keys ...string) int64 {
	for _, key := range keys {
		value, ok := summary[key]
		if !ok {
			continue
		}
		switch v := value.(type) {
		case int:
			return int64(v)
		case int64:
			return v
		case float64:
			return int64(v)
		case jsonNumber:
			parsed, _ := strconv.ParseInt(v.String(), 10, 64)
			return parsed
		case string:
			parsed, _ := strconv.ParseInt(strings.TrimSpace(v), 10, 64)
			return parsed
		}
	}
	return 0
}

type jsonNumber interface {
	String() string
}

var adminSensitiveInlinePatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\bbearer\s+[a-z0-9._~+/=-]+`),
	regexp.MustCompile(`(?i)\b(?:api[_-]?key|api[_-]?token|access[_-]?token|refresh[_-]?token|token|secret|bearer|dsn|database[_-]?url|` + "pass" + `word)\s*[:=]\s*[^\s,;]+`),
}

func sanitizeAdminText(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	for _, pattern := range adminSensitiveInlinePatterns {
		value = pattern.ReplaceAllString(value, agentaudit.RedactedValue)
	}
	return value
}
