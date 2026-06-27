// agent_audit_convert.go 把 agent-rpc 审计 gRPC 面的 pb 反向映射回 agentaudit 域类型，复用既有
// adminTracePB/adminToolCallsPB/... 转 admin pb（#616：admin traces/dashboard 经 agent-rpc 读，
// 不再直读 internal/repository agent_audit）。summary jsonb 由 *_summary_json 解析；时间为 RFC3339Nano。
package logic

import (
	"encoding/json"
	"time"

	"github.com/wujunhui99/agents_im/pkg/agentaudit"
	"github.com/wujunhui99/agents_im/service/agent/rpc/agent"
)

func agentRunFromPB(run *agent.AgentRunAudit) agentaudit.AgentRun {
	if run == nil {
		return agentaudit.AgentRun{}
	}
	return agentaudit.AgentRun{
		RunID:            run.GetRunId(),
		AgentID:          run.GetAgentId(),
		ConversationID:   run.GetConversationId(),
		TriggerMessageID: run.GetTriggerMessageId(),
		RequestingUserID: run.GetRequestingUserId(),
		Status:           agentaudit.Status(run.GetStatus()),
		InputSummary:     auditSummaryFromJSON(run.GetInputSummaryJson()),
		OutputSummary:    auditSummaryFromJSON(run.GetOutputSummaryJson()),
		OutputMessageID:  run.GetOutputMessageId(),
		ErrorCode:        run.GetErrorCode(),
		ErrorMessage:     run.GetErrorMessage(),
		TraceID:          run.GetTraceId(),
		RequestID:        run.GetRequestId(),
		StartedAt:        auditTimeFromRFC3339(run.GetStartedAt()),
		FinishedAt:       auditTimeFromRFC3339(run.GetFinishedAt()),
		CreatedAt:        auditTimeFromRFC3339(run.GetCreatedAt()),
	}
}

func agentToolCallsFromPB(calls []*agent.AgentToolCallAudit) []agentaudit.AgentToolCall {
	out := make([]agentaudit.AgentToolCall, 0, len(calls))
	for _, call := range calls {
		if call == nil {
			continue
		}
		out = append(out, agentaudit.AgentToolCall{
			ToolCallID:    call.GetToolCallId(),
			RunID:         call.GetRunId(),
			AgentID:       call.GetAgentId(),
			ToolID:        call.GetToolId(),
			ToolName:      call.GetToolName(),
			Status:        agentaudit.Status(call.GetStatus()),
			InputSummary:  auditSummaryFromJSON(call.GetInputSummaryJson()),
			OutputSummary: auditSummaryFromJSON(call.GetOutputSummaryJson()),
			DurationMs:    call.GetDurationMs(),
			ErrorCode:     call.GetErrorCode(),
			ErrorMessage:  call.GetErrorMessage(),
			TraceID:       call.GetTraceId(),
			RequestID:     call.GetRequestId(),
			StartedAt:     auditTimeFromRFC3339(call.GetStartedAt()),
			FinishedAt:    auditTimeFromRFC3339(call.GetFinishedAt()),
			CreatedAt:     auditTimeFromRFC3339(call.GetCreatedAt()),
		})
	}
	return out
}

func agentFileReadsFromPB(reads []*agent.AgentFileReadAudit) []agentaudit.AgentFileRead {
	out := make([]agentaudit.AgentFileRead, 0, len(reads))
	for _, read := range reads {
		if read == nil {
			continue
		}
		out = append(out, agentaudit.AgentFileRead{
			FileReadID:     read.GetFileReadId(),
			RunID:          read.GetRunId(),
			AgentID:        read.GetAgentId(),
			SkillID:        read.GetSkillId(),
			FileID:         read.GetFileId(),
			ObjectKey:      read.GetObjectKey(),
			SHA256:         read.GetSha256(),
			Status:         agentaudit.Status(read.GetStatus()),
			ByteCount:      read.GetByteCount(),
			ContentSummary: auditSummaryFromJSON(read.GetContentSummaryJson()),
			ErrorCode:      read.GetErrorCode(),
			ErrorMessage:   read.GetErrorMessage(),
			TraceID:        read.GetTraceId(),
			RequestID:      read.GetRequestId(),
			StartedAt:      auditTimeFromRFC3339(read.GetStartedAt()),
			FinishedAt:     auditTimeFromRFC3339(read.GetFinishedAt()),
			CreatedAt:      auditTimeFromRFC3339(read.GetCreatedAt()),
		})
	}
	return out
}

func agentPythonExecsFromPB(execs []*agent.AgentPythonExecAudit) []agentaudit.AgentPythonExec {
	out := make([]agentaudit.AgentPythonExec, 0, len(execs))
	for _, exec := range execs {
		if exec == nil {
			continue
		}
		out = append(out, agentaudit.AgentPythonExec{
			PythonExecID:     exec.GetPythonExecId(),
			RunID:            exec.GetRunId(),
			AgentID:          exec.GetAgentId(),
			SandboxRequestID: exec.GetSandboxRequestId(),
			Status:           agentaudit.Status(exec.GetStatus()),
			CodeSummary:      auditSummaryFromJSON(exec.GetCodeSummaryJson()),
			ResourceSummary:  auditSummaryFromJSON(exec.GetResourceSummaryJson()),
			StdoutSummary:    auditSummaryFromJSON(exec.GetStdoutSummaryJson()),
			StderrSummary:    auditSummaryFromJSON(exec.GetStderrSummaryJson()),
			ResultSummary:    auditSummaryFromJSON(exec.GetResultSummaryJson()),
			ErrorCode:        exec.GetErrorCode(),
			ErrorMessage:     exec.GetErrorMessage(),
			TraceID:          exec.GetTraceId(),
			RequestID:        exec.GetRequestId(),
			StartedAt:        auditTimeFromRFC3339(exec.GetStartedAt()),
			FinishedAt:       auditTimeFromRFC3339(exec.GetFinishedAt()),
			CreatedAt:        auditTimeFromRFC3339(exec.GetCreatedAt()),
		})
	}
	return out
}

func auditSummaryFromJSON(raw string) agentaudit.Summary {
	if raw == "" {
		return agentaudit.Summary{}
	}
	var summary map[string]any
	if err := json.Unmarshal([]byte(raw), &summary); err != nil {
		return agentaudit.Summary{}
	}
	return agentaudit.Summary(summary)
}

func auditTimeFromRFC3339(value string) time.Time {
	if value == "" {
		return time.Time{}
	}
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return time.Time{}
	}
	return parsed.UTC()
}
