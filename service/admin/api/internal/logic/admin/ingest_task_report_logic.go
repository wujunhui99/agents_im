// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package admin

import (
	"context"

	"github.com/wujunhui99/agents_im/common/share/rpcerror"
	"github.com/wujunhui99/agents_im/service/admin/api/internal/svc"
	"github.com/wujunhui99/agents_im/service/admin/api/internal/types"
	adminpb "github.com/wujunhui99/agents_im/service/admin/rpc/admin"

	"github.com/zeromicro/go-zero/core/logx"
)

type IngestTaskReportLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewIngestTaskReportLogic(ctx context.Context, svcCtx *svc.ServiceContext) *IngestTaskReportLogic {
	return &IngestTaskReportLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *IngestTaskReportLogic) IngestTaskReport(req *types.AdminTaskReportIngestReq) (resp *types.AdminTaskReportDetailResp, err error) {
	out, err := l.svcCtx.AdminRPC.UpsertTaskReport(l.ctx, &adminpb.TaskReportUpsertRequest{Report: &adminpb.AdminTaskReport{
		TaskId:                  req.TaskID,
		Agent:                   req.Agent,
		CodexSessionId:          req.CodexSessionID,
		IssueNumber:             req.Issue.Number,
		IssueUrl:                req.Issue.URL,
		Repo:                    req.Repo,
		Branch:                  req.Branch,
		Worktree:                req.Worktree,
		Commit:                  req.Commit,
		Outcome:                 req.Outcome,
		StartedAt:               req.StartedAt,
		EndedAt:                 req.EndedAt,
		DurationSeconds:         req.DurationSeconds,
		TokensUsed:              req.TokensUsed,
		PrUrl:                   req.PRURL,
		Evidence:                req.Evidence,
		Blockers:                req.Blockers,
		MajorTimeSinks:          req.MajorTimeSinks,
		WouldMorePermissionHelp: req.PermissionAnalysis.WouldMorePermissionHelp,
		CandidatePermissions:    req.PermissionAnalysis.CandidatePermissions,
		PermissionReason:        req.PermissionAnalysis.Reason,
		PitfallsOrLessons:       req.PitfallsOrLessons,
		Notes:                   req.Notes,
		RecordedAt:              req.RecordedAt,
	}})
	if err != nil {
		return nil, rpcerror.FromStatus(err)
	}
	return &types.AdminTaskReportDetailResp{Code: codeOK, Message: messageOK, Data: types.AdminTaskReportDetailData{Report: adminTaskReport(out.GetReport())}}, nil
}
