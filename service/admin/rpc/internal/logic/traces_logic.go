package logic

import (
	"context"
	"strings"

	"github.com/wujunhui99/agents_im/pkg/apperror"
	"github.com/wujunhui99/agents_im/pkg/rpcerror"
	"github.com/wujunhui99/agents_im/service/admin/rpc/admin"
	"github.com/wujunhui99/agents_im/service/admin/rpc/internal/svc"
	"github.com/wujunhui99/agents_im/service/agent/rpc/agent"

	"github.com/zeromicro/go-zero/core/logx"
)

// ---- ListLLMTraces ----

type ListLLMTracesLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListLLMTracesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListLLMTracesLogic {
	return &ListLLMTracesLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

func (l *ListLLMTracesLogic) ListLLMTraces(in *admin.LLMTraceListRequest) (*admin.LLMTraceListResponse, error) {
	if l.svcCtx.AgentRPC == nil {
		return nil, rpcerror.ToStatus(apperror.Internal("admin agent rpc client is not configured"))
	}
	resp, err := l.svcCtx.AgentRPC.ListAgentRuns(l.ctx, &agent.ListAgentRunsRequest{
		Status: strings.TrimSpace(in.GetStatus()),
		Limit:  int64(normalizeAdminLimit(int(in.GetLimit()), 20, 100)),
		Offset: int64(in.GetOffset()),
	})
	if err != nil {
		return nil, rpcerror.ToStatus(rpcerror.FromStatus(err))
	}
	traces := make([]*admin.AdminLLMTrace, 0, len(resp.GetRuns()))
	for _, run := range resp.GetRuns() {
		traces = append(traces, adminTracePB(agentRunFromPB(run)))
	}
	return &admin.LLMTraceListResponse{Traces: traces}, nil
}

// ---- GetLLMTraceDetail ----

type GetLLMTraceDetailLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetLLMTraceDetailLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetLLMTraceDetailLogic {
	return &GetLLMTraceDetailLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

func (l *GetLLMTraceDetailLogic) GetLLMTraceDetail(in *admin.LLMTraceDetailRequest) (*admin.LLMTraceDetailResponse, error) {
	if l.svcCtx.AgentRPC == nil {
		return nil, rpcerror.ToStatus(apperror.Internal("admin agent rpc client is not configured"))
	}
	traceID := strings.TrimSpace(in.GetTraceId())
	if traceID == "" {
		return nil, rpcerror.ToStatus(apperror.InvalidArgument("trace_id is required"))
	}
	runPB, err := l.svcCtx.AgentRPC.GetAgentRunByTraceID(l.ctx, &agent.GetAgentRunByTraceIDRequest{TraceId: traceID})
	if err != nil {
		appErr := rpcerror.FromStatus(err)
		if apperror.From(appErr).Code != apperror.CodeNotFound {
			return nil, rpcerror.ToStatus(appErr)
		}
		// 兜底：trace_id 缺失时按 run_id 直查（旧 internal/repository 同语义）。
		runPB, err = l.svcCtx.AgentRPC.GetAgentRun(l.ctx, &agent.GetAgentRunRequest{RunId: traceID})
		if err != nil {
			return nil, rpcerror.ToStatus(rpcerror.FromStatus(err))
		}
	}
	run := agentRunFromPB(runPB)
	toolCalls, err := l.svcCtx.AgentRPC.ListAgentToolCallsByRunID(l.ctx, &agent.ListAuditByRunIDRequest{RunId: run.RunID})
	if err != nil {
		return nil, rpcerror.ToStatus(rpcerror.FromStatus(err))
	}
	fileReads, err := l.svcCtx.AgentRPC.ListAgentFileReadsByRunID(l.ctx, &agent.ListAuditByRunIDRequest{RunId: run.RunID})
	if err != nil {
		return nil, rpcerror.ToStatus(rpcerror.FromStatus(err))
	}
	pythonExecs, err := l.svcCtx.AgentRPC.ListAgentPythonExecsByRunID(l.ctx, &agent.ListAuditByRunIDRequest{RunId: run.RunID})
	if err != nil {
		return nil, rpcerror.ToStatus(rpcerror.FromStatus(err))
	}
	return &admin.LLMTraceDetailResponse{
		Trace:       adminTracePB(run),
		ToolCalls:   adminToolCallsPB(agentToolCallsFromPB(toolCalls.GetToolCalls())),
		FileReads:   adminFileReadsPB(agentFileReadsFromPB(fileReads.GetFileReads())),
		PythonExecs: adminPythonExecsPB(agentPythonExecsFromPB(pythonExecs.GetPythonExecs())),
	}, nil
}
