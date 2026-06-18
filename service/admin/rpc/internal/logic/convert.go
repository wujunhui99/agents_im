package logic

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/wujunhui99/agents_im/internal/repository"
	"github.com/wujunhui99/agents_im/pkg/agentaudit"
	"github.com/wujunhui99/agents_im/pkg/apperror"
	"github.com/wujunhui99/agents_im/pkg/model"
	"github.com/wujunhui99/agents_im/pkg/observability"
	"github.com/wujunhui99/agents_im/service/admin/rpc/admin"
	dbmodel "github.com/wujunhui99/agents_im/service/admin/rpc/internal/model"
	userpb "github.com/wujunhui99/agents_im/service/user/rpc/user"
	"google.golang.org/protobuf/types/known/structpb"
)

// ---- 跨域只读 → pb 转换（内容做敏感词脱敏，时间统一 RFC3339Nano） ----

// adminUserPB 把 user-rpc UserEntity 直接转成 admin.AdminUser（不再经 model.User 过渡态）。
// UserEntity 只有 user_id，admin 的 user_id/account_id 同源；时间是 UnixMilli。
func adminUserPB(u *userpb.UserEntity) *admin.AdminUser {
	if u == nil {
		return &admin.AdminUser{}
	}
	return &admin.AdminUser{
		UserId:        u.GetUserId(),
		AccountId:     u.GetUserId(),
		Identifier:    u.GetIdentifier(),
		DisplayName:   u.GetDisplayName(),
		Name:          u.GetName(),
		Gender:        u.GetGender(),
		BirthDate:     u.GetBirthDate(),
		Region:        u.GetRegion(),
		AccountType:   u.GetAccountType(),
		AvatarMediaId: u.GetAvatarMediaId(),
		AvatarUrl:     u.GetAvatarUrl(),
		CreatedAt:     formatAdminTimeMillis(u.GetCreatedAt()),
		UpdatedAt:     formatAdminTimeMillis(u.GetUpdatedAt()),
	}
}

func adminMessagePB(message repository.Message) *admin.AdminMessage {
	return &admin.AdminMessage{
		ServerMsgId:           message.ServerMsgID,
		ClientMsgId:           message.ClientMsgID,
		ConversationId:        message.ConversationID,
		Seq:                   message.Seq,
		SenderId:              message.SenderID,
		ReceiverId:            message.ReceiverID,
		GroupId:               message.GroupID,
		ChatType:              message.ChatType,
		ContentType:           message.ContentType,
		Content:               sanitizeAdminText(message.Content),
		MessageOrigin:         message.MessageOrigin,
		AgentAccountId:        message.AgentAccountID,
		TriggerServerMsgId:    message.TriggerServerMsgID,
		AgentRunId:            message.AgentRunID,
		AllowRecursiveTrigger: message.AllowRecursiveTrigger,
		SendTime:              message.SendTime,
		CreatedAt:             message.CreatedAt,
	}
}

func adminMessagesPB(messages []repository.Message) []*admin.AdminMessage {
	out := make([]*admin.AdminMessage, 0, len(messages))
	for _, message := range messages {
		out = append(out, adminMessagePB(message))
	}
	return out
}

func adminConversationsPB(states []repository.ConversationSeqState) []*admin.AdminConversation {
	out := make([]*admin.AdminConversation, 0, len(states))
	for _, state := range states {
		conversation := &admin.AdminConversation{
			ConversationId: state.ConversationID,
			MaxSeq:         state.MaxSeq,
			HasReadSeq:     state.HasReadSeq,
			UnreadCount:    state.UnreadCount,
			MaxSeqTime:     state.MaxSeqTime,
		}
		if state.LastMessage != nil {
			conversation.LastMessage = adminMessagePB(*state.LastMessage)
		}
		out = append(out, conversation)
	}
	return out
}

func adminTracePB(run agentaudit.AgentRun) *admin.AdminLLMTrace {
	traceID := strings.TrimSpace(run.TraceID)
	if traceID == "" {
		traceID = run.RunID
	}
	return &admin.AdminLLMTrace{
		TraceId:           traceID,
		TraceUrl:          observability.TraceUIURL(observability.TraceUIBaseURLFromEnv(), traceID),
		RunId:             run.RunID,
		AgentId:           run.AgentID,
		ConversationId:    run.ConversationID,
		TriggerMessageId:  run.TriggerMessageID,
		ResponseMessageId: run.OutputMessageID,
		RequestingUserId:  run.RequestingUserID,
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

func adminToolCallsPB(calls []agentaudit.AgentToolCall) []*admin.AdminAgentToolCall {
	out := make([]*admin.AdminAgentToolCall, 0, len(calls))
	for _, call := range calls {
		out = append(out, &admin.AdminAgentToolCall{
			ToolCallId:   call.ToolCallID,
			RunId:        call.RunID,
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

func adminFileReadsPB(reads []agentaudit.AgentFileRead) []*admin.AdminAgentFileRead {
	out := make([]*admin.AdminAgentFileRead, 0, len(reads))
	for _, read := range reads {
		out = append(out, &admin.AdminAgentFileRead{
			FileReadId:   read.FileReadID,
			RunId:        read.RunID,
			SkillId:      read.SkillID,
			FileId:       read.FileID,
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

func adminPythonExecsPB(execs []agentaudit.AgentPythonExec) []*admin.AdminAgentPythonExec {
	out := make([]*admin.AdminAgentPythonExec, 0, len(execs))
	for _, exec := range execs {
		out = append(out, &admin.AdminAgentPythonExec{
			PythonExecId: exec.PythonExecID,
			RunId:        exec.RunID,
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

func adminFeedbackPB(feedback model.Feedback) (*admin.AdminFeedback, error) {
	var clientMeta *structpb.Struct
	if len(feedback.ClientMeta) > 0 {
		st, err := structpb.NewStruct(feedback.ClientMeta)
		if err != nil {
			return nil, apperror.Internal("encode feedback client_meta: " + err.Error())
		}
		clientMeta = st
	}
	return &admin.AdminFeedback{
		FeedbackId: feedback.FeedbackID,
		UserId:     feedback.UserID,
		Category:   string(feedback.Category),
		Status:     string(feedback.Status),
		Title:      feedback.Title,
		Content:    feedback.Content,
		Contact:    feedback.Contact,
		PageUrl:    feedback.PageURL,
		UserAgent:  feedback.UserAgent,
		ClientMeta: clientMeta,
		AdminNote:  feedback.AdminNote,
		CreatedAt:  formatAdminTime(feedback.CreatedAt),
		UpdatedAt:  formatAdminTime(feedback.UpdatedAt),
	}, nil
}

// ---- task_reports：goctl 行 ↔ pb（JSONB ↔ []string、Null* ↔ 标量、时间 ↔ RFC3339Nano） ----

func adminTaskReportPB(row *dbmodel.TaskReports) (*admin.AdminTaskReport, error) {
	if row == nil {
		return &admin.AdminTaskReport{}, nil
	}
	report := &admin.AdminTaskReport{
		TaskId:                  row.TaskId,
		Agent:                   row.Agent,
		CodexSessionId:          row.CodexSessionId,
		IssueNumber:             row.IssueNumber.Int64,
		IssueUrl:                row.IssueUrl,
		Repo:                    row.Repo,
		Branch:                  row.Branch,
		Worktree:                row.Worktree,
		Commit:                  row.CommitSha,
		Outcome:                 row.Outcome,
		StartedAt:               formatAdminNullTime(row.StartedAt),
		EndedAt:                 formatAdminNullTime(row.EndedAt),
		DurationSeconds:         row.DurationSeconds.Int64,
		TokensUsed:              row.TokensUsed.Int64,
		PrUrl:                   row.PrUrl,
		WouldMorePermissionHelp: row.WouldMorePermissionHelp,
		PermissionReason:        row.PermissionReason,
		Notes:                   row.Notes,
		RecordedAt:              formatAdminTime(row.RecordedAt),
	}
	var err error
	if report.Evidence, err = decodeStringSlice(row.Evidence); err != nil {
		return nil, err
	}
	if report.Blockers, err = decodeStringSlice(row.Blockers); err != nil {
		return nil, err
	}
	if report.MajorTimeSinks, err = decodeStringSlice(row.MajorTimeSinks); err != nil {
		return nil, err
	}
	if report.CandidatePermissions, err = decodeStringSlice(row.CandidatePermissions); err != nil {
		return nil, err
	}
	if report.PitfallsOrLessons, err = decodeStringSlice(row.PitfallsOrLessons); err != nil {
		return nil, err
	}
	return report, nil
}

func taskReportRowFromPB(report *admin.AdminTaskReport, now time.Time) (*dbmodel.TaskReports, error) {
	if report == nil {
		return nil, apperror.InvalidArgument("task report is required")
	}
	evidence, err := encodeStringSlice(report.GetEvidence())
	if err != nil {
		return nil, err
	}
	blockers, err := encodeStringSlice(report.GetBlockers())
	if err != nil {
		return nil, err
	}
	timeSinks, err := encodeStringSlice(report.GetMajorTimeSinks())
	if err != nil {
		return nil, err
	}
	candidatePermissions, err := encodeStringSlice(report.GetCandidatePermissions())
	if err != nil {
		return nil, err
	}
	lessons, err := encodeStringSlice(report.GetPitfallsOrLessons())
	if err != nil {
		return nil, err
	}
	startedAt, err := parseAdminNullTime(report.GetStartedAt())
	if err != nil {
		return nil, err
	}
	endedAt, err := parseAdminNullTime(report.GetEndedAt())
	if err != nil {
		return nil, err
	}
	recordedAt := now.UTC()
	if trimmed := strings.TrimSpace(report.GetRecordedAt()); trimmed != "" {
		parsed, perr := time.Parse(time.RFC3339Nano, trimmed)
		if perr != nil {
			return nil, apperror.InvalidArgument("recorded_at must be RFC3339")
		}
		recordedAt = parsed.UTC()
	}
	return &dbmodel.TaskReports{
		TaskId:                  strings.TrimSpace(report.GetTaskId()),
		Agent:                   strings.TrimSpace(report.GetAgent()),
		CodexSessionId:          strings.TrimSpace(report.GetCodexSessionId()),
		IssueNumber:             nullInt64(report.GetIssueNumber()),
		IssueUrl:                strings.TrimSpace(report.GetIssueUrl()),
		Repo:                    strings.TrimSpace(report.GetRepo()),
		Branch:                  strings.TrimSpace(report.GetBranch()),
		Worktree:                strings.TrimSpace(report.GetWorktree()),
		CommitSha:               strings.TrimSpace(report.GetCommit()),
		Outcome:                 strings.TrimSpace(report.GetOutcome()),
		StartedAt:               startedAt,
		EndedAt:                 endedAt,
		DurationSeconds:         nullInt64(report.GetDurationSeconds()),
		TokensUsed:              nullInt64(report.GetTokensUsed()),
		PrUrl:                   strings.TrimSpace(report.GetPrUrl()),
		Evidence:                evidence,
		Blockers:                blockers,
		MajorTimeSinks:          timeSinks,
		WouldMorePermissionHelp: strings.TrimSpace(report.GetWouldMorePermissionHelp()),
		CandidatePermissions:    candidatePermissions,
		PermissionReason:        strings.TrimSpace(report.GetPermissionReason()),
		PitfallsOrLessons:       lessons,
		Notes:                   strings.TrimSpace(report.GetNotes()),
		RecordedAt:              recordedAt,
	}, nil
}

// ---- 校验与小工具 ----

func validateRequiredAdminID(value, field string, maxLen int) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", apperror.InvalidArgument(field + " is required")
	}
	if len([]rune(value)) > maxLen {
		return "", apperror.InvalidArgument(fmt.Sprintf("%s must be %d characters or fewer", field, maxLen))
	}
	if strings.Contains(value, "\x00") {
		return "", apperror.InvalidArgument(field + " cannot contain NUL")
	}
	return value, nil
}

func normalizeAdminLimit(value, fallback, max int) int {
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

// formatAdminTimeMillis 把 user-rpc 的 UnixMilli 时间戳渲染成 admin pb 的 RFC3339Nano(UTC) 串；
// 0 → 空串（与 formatAdminTime 零值行为一致）。
func formatAdminTimeMillis(ms int64) string {
	if ms == 0 {
		return ""
	}
	return time.UnixMilli(ms).UTC().Format(time.RFC3339Nano)
}

func formatAdminNullTime(value sql.NullTime) string {
	if !value.Valid {
		return ""
	}
	return formatAdminTime(value.Time)
}

func parseAdminNullTime(value string) (sql.NullTime, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return sql.NullTime{}, nil
	}
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return sql.NullTime{}, apperror.InvalidArgument("timestamp must be RFC3339")
	}
	return sql.NullTime{Time: parsed.UTC(), Valid: true}, nil
}

func nullInt64(value int64) sql.NullInt64 {
	return sql.NullInt64{Int64: value, Valid: value != 0}
}

func decodeStringSlice(raw string) ([]string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return []string{}, nil
	}
	out := []string{}
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil, apperror.Internal("decode task report list: " + err.Error())
	}
	return out, nil
}

func encodeStringSlice(values []string) (string, error) {
	if values == nil {
		values = []string{}
	}
	raw, err := json.Marshal(values)
	if err != nil {
		return "", apperror.Internal("encode task report list: " + err.Error())
	}
	return string(raw), nil
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
