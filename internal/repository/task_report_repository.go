package repository

import (
	"context"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/wujunhui99/agents_im/internal/apperror"
)

type TaskReport struct {
	TaskID                  string
	Agent                   string
	CodexSessionID          string
	IssueNumber             int64
	IssueURL                string
	Repo                    string
	Branch                  string
	Worktree                string
	Commit                  string
	Outcome                 string
	StartedAt               string
	EndedAt                 string
	DurationSeconds         int64
	TokensUsed              int64
	PRURL                   string
	Evidence                []string
	Blockers                []string
	MajorTimeSinks          []string
	WouldMorePermissionHelp string
	CandidatePermissions    []string
	PermissionReason        string
	PitfallsOrLessons       []string
	Notes                   string
	RecordedAt              string
}

type TaskReportListFilter struct {
	Outcome string
	Limit   int
	Offset  int
}

type TaskReportRepository interface {
	UpsertTaskReport(ctx context.Context, report TaskReport) (TaskReport, error)
	ListTaskReports(ctx context.Context, filter TaskReportListFilter) ([]TaskReport, error)
}

type MemoryTaskReportRepository struct {
	mu      sync.RWMutex
	reports map[string]TaskReport
}

func NewMemoryTaskReportRepository() *MemoryTaskReportRepository {
	return &MemoryTaskReportRepository{reports: map[string]TaskReport{}}
}

func (r *MemoryTaskReportRepository) UpsertTaskReport(_ context.Context, report TaskReport) (TaskReport, error) {
	if r == nil {
		return TaskReport{}, apperror.Internal("task report repository is not configured")
	}
	report.TaskID = strings.TrimSpace(report.TaskID)
	if report.TaskID == "" {
		return TaskReport{}, apperror.InvalidArgument("task_id is required")
	}
	if strings.TrimSpace(report.RecordedAt) == "" {
		report.RecordedAt = time.Now().UTC().Format(time.RFC3339)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.reports[report.TaskID] = cloneTaskReport(report)
	return cloneTaskReport(report), nil
}

func (r *MemoryTaskReportRepository) ListTaskReports(_ context.Context, filter TaskReportListFilter) ([]TaskReport, error) {
	if r == nil {
		return nil, apperror.Internal("task report repository is not configured")
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]TaskReport, 0, len(r.reports))
	outcome := strings.TrimSpace(filter.Outcome)
	for _, report := range r.reports {
		if outcome != "" && report.Outcome != outcome {
			continue
		}
		out = append(out, cloneTaskReport(report))
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].RecordedAt == out[j].RecordedAt {
			return out[i].TaskID > out[j].TaskID
		}
		return out[i].RecordedAt > out[j].RecordedAt
	})
	start := filter.Offset
	if start < 0 {
		start = 0
	}
	if start >= len(out) {
		return []TaskReport{}, nil
	}
	end := len(out)
	if filter.Limit > 0 && start+filter.Limit < end {
		end = start + filter.Limit
	}
	return out[start:end], nil
}

func cloneTaskReport(report TaskReport) TaskReport {
	report.Evidence = append([]string(nil), report.Evidence...)
	report.Blockers = append([]string(nil), report.Blockers...)
	report.MajorTimeSinks = append([]string(nil), report.MajorTimeSinks...)
	report.CandidatePermissions = append([]string(nil), report.CandidatePermissions...)
	report.PitfallsOrLessons = append([]string(nil), report.PitfallsOrLessons...)
	return report
}
