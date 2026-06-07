package logic

import (
	"context"
	"strings"

	"github.com/wujunhui99/agents_im/common/share/rpcerror"
	"github.com/wujunhui99/agents_im/internal/repository"
	"github.com/wujunhui99/agents_im/pkg/apperror"
	"github.com/wujunhui99/agents_im/service/admin/rpc/admin"
	"github.com/wujunhui99/agents_im/service/admin/rpc/internal/svc"

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
	if l.svcCtx.AgentAudits == nil {
		return nil, rpcerror.ToStatus(apperror.Internal("admin agent audit repository is not configured"))
	}
	runs, err := l.svcCtx.AgentAudits.ListAgentRuns(l.ctx, repository.AgentRunFilter{
		Status: strings.TrimSpace(in.GetStatus()),
		Limit:  normalizeAdminLimit(int(in.GetLimit()), 20, 100),
		Offset: int(in.GetOffset()),
	})
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	traces := make([]*admin.AdminLLMTrace, 0, len(runs))
	for _, run := range runs {
		traces = append(traces, adminTracePB(run))
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
	if l.svcCtx.AgentAudits == nil {
		return nil, rpcerror.ToStatus(apperror.Internal("admin agent audit repository is not configured"))
	}
	traceID := strings.TrimSpace(in.GetTraceId())
	if traceID == "" {
		return nil, rpcerror.ToStatus(apperror.InvalidArgument("trace_id is required"))
	}
	run, err := l.svcCtx.AgentAudits.GetAgentRunByTraceID(l.ctx, traceID)
	if err != nil && apperror.From(err).Code == apperror.CodeNotFound {
		run, err = l.svcCtx.AgentAudits.GetAgentRun(l.ctx, traceID)
	}
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	toolCalls, err := l.svcCtx.AgentAudits.ListAgentToolCallsByRunID(l.ctx, run.RunID)
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	fileReads, err := l.svcCtx.AgentAudits.ListAgentFileReadsByRunID(l.ctx, run.RunID)
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	pythonExecs, err := l.svcCtx.AgentAudits.ListAgentPythonExecsByRunID(l.ctx, run.RunID)
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	return &admin.LLMTraceDetailResponse{
		Trace:       adminTracePB(run),
		ToolCalls:   adminToolCallsPB(toolCalls),
		FileReads:   adminFileReadsPB(fileReads),
		PythonExecs: adminPythonExecsPB(pythonExecs),
	}, nil
}
