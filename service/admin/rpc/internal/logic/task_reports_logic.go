package logic

import (
	"context"
	"strings"
	"time"

	"github.com/wujunhui99/agents_im/pkg/apperror"
	"github.com/wujunhui99/agents_im/pkg/rpcerror"
	"github.com/wujunhui99/agents_im/service/admin/rpc/admin"
	dbmodel "github.com/wujunhui99/agents_im/service/admin/rpc/internal/model"
	"github.com/wujunhui99/agents_im/service/admin/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

// ---- ListTaskReports ----

type ListTaskReportsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListTaskReportsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListTaskReportsLogic {
	return &ListTaskReportsLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

func (l *ListTaskReportsLogic) ListTaskReports(in *admin.TaskReportListRequest) (*admin.TaskReportListResponse, error) {
	rows, err := l.svcCtx.TaskReportModel.ListTaskReports(l.ctx, dbmodel.TaskReportListFilter{
		Outcome: strings.TrimSpace(in.GetOutcome()),
		Limit:   int(in.GetLimit()),
		Offset:  int(in.GetOffset()),
	})
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	items := make([]*admin.AdminTaskReport, 0, len(rows))
	for _, row := range rows {
		report, err := adminTaskReportPB(row)
		if err != nil {
			return nil, rpcerror.ToStatus(err)
		}
		items = append(items, report)
	}
	return &admin.TaskReportListResponse{Items: items}, nil
}

// ---- UpsertTaskReport ----

type UpsertTaskReportLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUpsertTaskReportLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpsertTaskReportLogic {
	return &UpsertTaskReportLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

func (l *UpsertTaskReportLogic) UpsertTaskReport(in *admin.TaskReportUpsertRequest) (*admin.TaskReportDetailResponse, error) {
	row, err := taskReportRowFromPB(in.GetReport(), time.Now())
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	if row.TaskId == "" {
		return nil, rpcerror.ToStatus(apperror.InvalidArgument("task_id is required"))
	}
	stored, err := l.svcCtx.TaskReportModel.UpsertTaskReport(l.ctx, row)
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	report, err := adminTaskReportPB(stored)
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	return &admin.TaskReportDetailResponse{Report: report}, nil
}
