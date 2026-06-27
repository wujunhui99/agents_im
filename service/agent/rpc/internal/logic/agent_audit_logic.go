// agent_audit_logic.go 实现 agent 审计只读 gRPC 面（#616）：agent 域自有 append-only 审计四表的
// owner 只读 API，供 admin-rpc BFF 聚合 traces/dashboard（admin 不再直读 internal/repository
// agent_audit）。数据层走 agent 自有 goctl Store（svcCtx.Hosting.AgentAudit）。
// summary jsonb 经 *_summary_json 串行携带；时间为 RFC3339Nano（零值空串）。
package logic

import (
	"context"
	"encoding/json"
	"time"

	"github.com/wujunhui99/agents_im/pkg/agentaudit"
	"github.com/wujunhui99/agents_im/pkg/apperror"
	"github.com/wujunhui99/agents_im/pkg/rpcerror"
	"github.com/wujunhui99/agents_im/service/agent/rpc/agent"
	"github.com/wujunhui99/agents_im/service/agent/rpc/internal/agaudit"
	"github.com/wujunhui99/agents_im/service/agent/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

// auditStore 取出 agent 审计 Store；未装配返回 nil（worker/单测路径可能不接线）。
func auditStore(svcCtx *svc.ServiceContext) agaudit.Store {
	if svcCtx == nil || svcCtx.Hosting == nil {
		return nil
	}
	return svcCtx.Hosting.AgentAudit
}

func auditNotConfigured() error {
	return rpcerror.ToStatus(apperror.Internal("agent audit store is not configured"))
}

// ---- ListAgentRuns ----

type ListAgentRunsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListAgentRunsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListAgentRunsLogic {
	return &ListAgentRunsLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

func (l *ListAgentRunsLogic) ListAgentRuns(in *agent.ListAgentRunsRequest) (*agent.ListAgentRunsResponse, error) {
	store := auditStore(l.svcCtx)
	if store == nil {
		return nil, auditNotConfigured()
	}
	runs, err := store.ListAgentRuns(l.ctx, agaudit.RunFilter{
		Status: in.GetStatus(),
		Limit:  int(in.GetLimit()),
		Offset: int(in.GetOffset()),
	})
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	out := make([]*agent.AgentRunAudit, 0, len(runs))
	for _, run := range runs {
		out = append(out, agentRunAuditToPB(run))
	}
	return &agent.ListAgentRunsResponse{Runs: out}, nil
}

// ---- CountAgentRuns ----

type CountAgentRunsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCountAgentRunsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CountAgentRunsLogic {
	return &CountAgentRunsLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

func (l *CountAgentRunsLogic) CountAgentRuns(in *agent.CountAgentRunsRequest) (*agent.CountAgentRunsResponse, error) {
	store := auditStore(l.svcCtx)
	if store == nil {
		return nil, auditNotConfigured()
	}
	count, err := store.CountAgentRuns(l.ctx, in.GetStatus())
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	return &agent.CountAgentRunsResponse{Count: count}, nil
}

// ---- GetAgentRun ----

type GetAgentRunLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetAgentRunLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetAgentRunLogic {
	return &GetAgentRunLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

func (l *GetAgentRunLogic) GetAgentRun(in *agent.GetAgentRunRequest) (*agent.AgentRunAudit, error) {
	store := auditStore(l.svcCtx)
	if store == nil {
		return nil, auditNotConfigured()
	}
	run, err := store.GetAgentRun(l.ctx, in.GetRunId())
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	return agentRunAuditToPB(run), nil
}

// ---- GetAgentRunByTraceID ----

type GetAgentRunByTraceIDLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetAgentRunByTraceIDLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetAgentRunByTraceIDLogic {
	return &GetAgentRunByTraceIDLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

func (l *GetAgentRunByTraceIDLogic) GetAgentRunByTraceID(in *agent.GetAgentRunByTraceIDRequest) (*agent.AgentRunAudit, error) {
	store := auditStore(l.svcCtx)
	if store == nil {
		return nil, auditNotConfigured()
	}
	run, err := store.GetAgentRunByTraceID(l.ctx, in.GetTraceId())
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	return agentRunAuditToPB(run), nil
}

// ---- ListAgentToolCallsByRunID ----

type ListAgentToolCallsByRunIDLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListAgentToolCallsByRunIDLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListAgentToolCallsByRunIDLogic {
	return &ListAgentToolCallsByRunIDLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

func (l *ListAgentToolCallsByRunIDLogic) ListAgentToolCallsByRunID(in *agent.ListAuditByRunIDRequest) (*agent.ListAgentToolCallsResponse, error) {
	store := auditStore(l.svcCtx)
	if store == nil {
		return nil, auditNotConfigured()
	}
	calls, err := store.ListAgentToolCallsByRunID(l.ctx, in.GetRunId())
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	out := make([]*agent.AgentToolCallAudit, 0, len(calls))
	for _, call := range calls {
		out = append(out, agentToolCallAuditToPB(call))
	}
	return &agent.ListAgentToolCallsResponse{ToolCalls: out}, nil
}

// ---- ListAgentFileReadsByRunID ----

type ListAgentFileReadsByRunIDLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListAgentFileReadsByRunIDLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListAgentFileReadsByRunIDLogic {
	return &ListAgentFileReadsByRunIDLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

func (l *ListAgentFileReadsByRunIDLogic) ListAgentFileReadsByRunID(in *agent.ListAuditByRunIDRequest) (*agent.ListAgentFileReadsResponse, error) {
	store := auditStore(l.svcCtx)
	if store == nil {
		return nil, auditNotConfigured()
	}
	reads, err := store.ListAgentFileReadsByRunID(l.ctx, in.GetRunId())
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	out := make([]*agent.AgentFileReadAudit, 0, len(reads))
	for _, read := range reads {
		out = append(out, agentFileReadAuditToPB(read))
	}
	return &agent.ListAgentFileReadsResponse{FileReads: out}, nil
}

// ---- ListAgentPythonExecsByRunID ----

type ListAgentPythonExecsByRunIDLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListAgentPythonExecsByRunIDLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListAgentPythonExecsByRunIDLogic {
	return &ListAgentPythonExecsByRunIDLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

func (l *ListAgentPythonExecsByRunIDLogic) ListAgentPythonExecsByRunID(in *agent.ListAuditByRunIDRequest) (*agent.ListAgentPythonExecsResponse, error) {
	store := auditStore(l.svcCtx)
	if store == nil {
		return nil, auditNotConfigured()
	}
	execs, err := store.ListAgentPythonExecsByRunID(l.ctx, in.GetRunId())
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	out := make([]*agent.AgentPythonExecAudit, 0, len(execs))
	for _, exec := range execs {
		out = append(out, agentPythonExecAuditToPB(exec))
	}
	return &agent.ListAgentPythonExecsResponse{PythonExecs: out}, nil
}

// ---- agentaudit.* → pb 映射（summary jsonb → JSON 串、时间 → RFC3339Nano） ----

func agentRunAuditToPB(run agentaudit.AgentRun) *agent.AgentRunAudit {
	return &agent.AgentRunAudit{
		RunId:             run.RunID,
		AgentId:           run.AgentID,
		ConversationId:    run.ConversationID,
		TriggerMessageId:  run.TriggerMessageID,
		RequestingUserId:  run.RequestingUserID,
		Status:            string(run.Status),
		InputSummaryJson:  auditSummaryJSON(run.InputSummary),
		OutputSummaryJson: auditSummaryJSON(run.OutputSummary),
		OutputMessageId:   run.OutputMessageID,
		ErrorCode:         run.ErrorCode,
		ErrorMessage:      run.ErrorMessage,
		TraceId:           run.TraceID,
		RequestId:         run.RequestID,
		StartedAt:         auditTimeRFC3339(run.StartedAt),
		FinishedAt:        auditTimeRFC3339(run.FinishedAt),
		CreatedAt:         auditTimeRFC3339(run.CreatedAt),
	}
}

func agentToolCallAuditToPB(call agentaudit.AgentToolCall) *agent.AgentToolCallAudit {
	return &agent.AgentToolCallAudit{
		ToolCallId:        call.ToolCallID,
		RunId:             call.RunID,
		AgentId:           call.AgentID,
		ToolId:            call.ToolID,
		ToolName:          call.ToolName,
		Status:            string(call.Status),
		InputSummaryJson:  auditSummaryJSON(call.InputSummary),
		OutputSummaryJson: auditSummaryJSON(call.OutputSummary),
		DurationMs:        call.DurationMs,
		ErrorCode:         call.ErrorCode,
		ErrorMessage:      call.ErrorMessage,
		TraceId:           call.TraceID,
		RequestId:         call.RequestID,
		StartedAt:         auditTimeRFC3339(call.StartedAt),
		FinishedAt:        auditTimeRFC3339(call.FinishedAt),
		CreatedAt:         auditTimeRFC3339(call.CreatedAt),
	}
}

func agentFileReadAuditToPB(read agentaudit.AgentFileRead) *agent.AgentFileReadAudit {
	return &agent.AgentFileReadAudit{
		FileReadId:         read.FileReadID,
		RunId:              read.RunID,
		AgentId:            read.AgentID,
		SkillId:            read.SkillID,
		FileId:             read.FileID,
		ObjectKey:          read.ObjectKey,
		Sha256:             read.SHA256,
		Status:             string(read.Status),
		ByteCount:          read.ByteCount,
		ContentSummaryJson: auditSummaryJSON(read.ContentSummary),
		ErrorCode:          read.ErrorCode,
		ErrorMessage:       read.ErrorMessage,
		TraceId:            read.TraceID,
		RequestId:          read.RequestID,
		StartedAt:          auditTimeRFC3339(read.StartedAt),
		FinishedAt:         auditTimeRFC3339(read.FinishedAt),
		CreatedAt:          auditTimeRFC3339(read.CreatedAt),
	}
}

func agentPythonExecAuditToPB(exec agentaudit.AgentPythonExec) *agent.AgentPythonExecAudit {
	return &agent.AgentPythonExecAudit{
		PythonExecId:        exec.PythonExecID,
		RunId:               exec.RunID,
		AgentId:             exec.AgentID,
		SandboxRequestId:    exec.SandboxRequestID,
		Status:              string(exec.Status),
		CodeSummaryJson:     auditSummaryJSON(exec.CodeSummary),
		ResourceSummaryJson: auditSummaryJSON(exec.ResourceSummary),
		StdoutSummaryJson:   auditSummaryJSON(exec.StdoutSummary),
		StderrSummaryJson:   auditSummaryJSON(exec.StderrSummary),
		ResultSummaryJson:   auditSummaryJSON(exec.ResultSummary),
		ErrorCode:           exec.ErrorCode,
		ErrorMessage:        exec.ErrorMessage,
		TraceId:             exec.TraceID,
		RequestId:           exec.RequestID,
		StartedAt:           auditTimeRFC3339(exec.StartedAt),
		FinishedAt:          auditTimeRFC3339(exec.FinishedAt),
		CreatedAt:           auditTimeRFC3339(exec.CreatedAt),
	}
}

// auditSummaryJSON 把 Summary（jsonb 对象）串成紧凑 JSON；空对象返回 "{}"。
func auditSummaryJSON(summary agentaudit.Summary) string {
	if len(summary) == 0 {
		return "{}"
	}
	encoded, err := json.Marshal(summary)
	if err != nil {
		return "{}"
	}
	return string(encoded)
}

// auditTimeRFC3339 把时间格式化为 RFC3339Nano；零值返回空串（admin 侧空串 → 零值）。
func auditTimeRFC3339(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.UTC().Format(time.RFC3339Nano)
}
