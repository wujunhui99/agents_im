package logic

import (
	"context"
	"testing"

	"github.com/wujunhui99/agents_im/service/admin/rpc/admin"
	dbmodel "github.com/wujunhui99/agents_im/service/admin/rpc/internal/model"
	"github.com/wujunhui99/agents_im/service/admin/rpc/internal/svc"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// fakeTaskReportsModel 实现 model.TaskReportsModel，仅记录 upsert 入参并回放，供 logic 单测（无需 PG）。
type fakeTaskReportsModel struct {
	dbmodel.TaskReportsModel
	upserted *dbmodel.TaskReports
	listed   []*dbmodel.TaskReports
}

func (m *fakeTaskReportsModel) UpsertTaskReport(_ context.Context, data *dbmodel.TaskReports) (*dbmodel.TaskReports, error) {
	m.upserted = data
	return data, nil
}

func (m *fakeTaskReportsModel) ListTaskReports(_ context.Context, _ dbmodel.TaskReportListFilter) ([]*dbmodel.TaskReports, error) {
	return m.listed, nil
}

func (m *fakeTaskReportsModel) WithSession(sqlx.Session) dbmodel.TaskReportsModel { return m }

func TestUpsertTaskReportMapsPBToRowAndBack(t *testing.T) {
	fake := &fakeTaskReportsModel{}
	svcCtx := &svc.ServiceContext{TaskReportModel: fake}
	l := NewUpsertTaskReportLogic(context.Background(), svcCtx)

	resp, err := l.UpsertTaskReport(&admin.TaskReportUpsertRequest{Report: &admin.AdminTaskReport{
		TaskId:               "task-1",
		Agent:                "claude",
		IssueNumber:          448,
		Outcome:              "success",
		Commit:               "abc123",
		Evidence:             []string{"pr#448", "ci-green"},
		CandidatePermissions: []string{"gh:pr"},
		StartedAt:            "2026-06-06T01:02:03Z",
	}})
	if err != nil {
		t.Fatalf("UpsertTaskReport: %v", err)
	}

	// pb → goctl 行：标量进 NULLIF 语义的 sql.Null*，JSONB 列编码成字符串，commit→commit_sha。
	if fake.upserted == nil {
		t.Fatal("model.UpsertTaskReport was not called")
	}
	if !fake.upserted.IssueNumber.Valid || fake.upserted.IssueNumber.Int64 != 448 {
		t.Fatalf("issue_number not mapped: %+v", fake.upserted.IssueNumber)
	}
	if fake.upserted.CommitSha != "abc123" {
		t.Fatalf("commit_sha = %q, want abc123", fake.upserted.CommitSha)
	}
	if !fake.upserted.StartedAt.Valid {
		t.Fatal("started_at should be a valid time")
	}
	if fake.upserted.RecordedAt.IsZero() {
		t.Fatal("recorded_at should default to now when empty")
	}

	// goctl 行 → pb：JSONB 字符串解回 []string，Null* 解回标量。
	got := resp.GetReport()
	if got.GetIssueNumber() != 448 || got.GetCommit() != "abc123" {
		t.Fatalf("round-trip scalars mismatch: issue=%d commit=%q", got.GetIssueNumber(), got.GetCommit())
	}
	if len(got.GetEvidence()) != 2 || got.GetEvidence()[0] != "pr#448" {
		t.Fatalf("evidence round-trip mismatch: %v", got.GetEvidence())
	}
	if len(got.GetCandidatePermissions()) != 1 || got.GetCandidatePermissions()[0] != "gh:pr" {
		t.Fatalf("candidate_permissions round-trip mismatch: %v", got.GetCandidatePermissions())
	}
}

func TestUpsertTaskReportRequiresTaskID(t *testing.T) {
	svcCtx := &svc.ServiceContext{TaskReportModel: &fakeTaskReportsModel{}}
	l := NewUpsertTaskReportLogic(context.Background(), svcCtx)
	if _, err := l.UpsertTaskReport(&admin.TaskReportUpsertRequest{Report: &admin.AdminTaskReport{}}); err == nil {
		t.Fatal("expected error when task_id is empty")
	}
}

func TestListTaskReportsEncodesEmptyJSONBAsList(t *testing.T) {
	// 行中 JSONB 为空字符串时应解成空切片而非 nil/报错。
	fake := &fakeTaskReportsModel{listed: []*dbmodel.TaskReports{{TaskId: "t", Evidence: "", Blockers: "[]"}}}
	svcCtx := &svc.ServiceContext{TaskReportModel: fake}
	l := NewListTaskReportsLogic(context.Background(), svcCtx)
	resp, err := l.ListTaskReports(&admin.TaskReportListRequest{})
	if err != nil {
		t.Fatalf("ListTaskReports: %v", err)
	}
	if len(resp.GetItems()) != 1 {
		t.Fatalf("want 1 item, got %d", len(resp.GetItems()))
	}
	if resp.GetItems()[0].GetEvidence() == nil {
		t.Fatal("evidence should decode to empty slice, not nil")
	}
}
