package agenteval

import (
	"context"
	"fmt"
	"strings"
)

const PythonGoPerformanceCaseID = "ai_hosting.python_go_performance.v1"

type EvaluationRequest struct {
	CaseID     string
	Input      string
	Output     string
	TraceID    string
	AgentRunID string
}

type EvaluationResult struct {
	CaseID     string  `json:"case_id"`
	Score      float64 `json:"score"`
	Pass       bool    `json:"pass"`
	Reason     string  `json:"reason"`
	TraceID    string  `json:"trace_id,omitempty"`
	AgentRunID string  `json:"agent_run_id,omitempty"`
}

type Judge interface {
	Evaluate(ctx context.Context, req EvaluationRequest) (EvaluationResult, error)
}

type RuleJudge struct {
	PassThreshold float64
}

func DefaultJudge() RuleJudge {
	return RuleJudge{PassThreshold: 0.7}
}

func (j RuleJudge) Evaluate(_ context.Context, req EvaluationRequest) (EvaluationResult, error) {
	caseID := strings.TrimSpace(req.CaseID)
	if caseID == "" {
		caseID = PythonGoPerformanceCaseID
	}
	if caseID != PythonGoPerformanceCaseID {
		return EvaluationResult{}, fmt.Errorf("unsupported eval case %q", caseID)
	}
	output := strings.TrimSpace(req.Output)
	if output == "" {
		return EvaluationResult{}, fmt.Errorf("output is required")
	}
	score, reason := scorePythonGoPerformance(output)
	threshold := j.PassThreshold
	if threshold <= 0 {
		threshold = 0.7
	}
	return EvaluationResult{
		CaseID:     caseID,
		Score:      score,
		Pass:       score >= threshold,
		Reason:     reason,
		TraceID:    strings.TrimSpace(req.TraceID),
		AgentRunID: strings.TrimSpace(req.AgentRunID),
	}, nil
}

func scorePythonGoPerformance(output string) (float64, string) {
	text := strings.ToLower(output)
	criteria := []struct {
		name   string
		groups [][]string
	}{
		{
			name: "execution speed and compiled-vs-interpreted behavior",
			groups: [][]string{
				{"快", "性能", "speed", "faster"},
				{"编译", "机器码", "compiled", "native"},
				{"解释", "解释器", "vm", "interpreter", "cpython", "pypy"},
			},
		},
		{
			name: "concurrency tradeoffs",
			groups: [][]string{
				{"goroutine", "协程", "channel"},
				{"gil", "multiprocessing", "多进程", "asyncio", "native extension", "c 扩展", "原生扩展"},
			},
		},
		{
			name: "startup and deployment",
			groups: [][]string{
				{"启动", "startup", "部署", "deployment", "单个", "二进制", "binary", "容器", "依赖环境", "虚拟环境"},
			},
		},
		{
			name: "memory and runtime overhead",
			groups: [][]string{
				{"内存", "memory", "runtime", "运行时", "gc", "对象模型", "开销"},
			},
		},
		{
			name: "ecosystem and suitable scenarios",
			groups: [][]string{
				{"生态", "ecosystem", "数据科学", "ai", "脚本", "后端", "基础设施", "云原生", "cli", "场景"},
			},
		},
		{
			name: "use-case dependent conclusion",
			groups: [][]string{
				{"取决", "视", "depends", "场景", "业务", "选择"},
			},
		},
	}
	hit := 0
	missing := make([]string, 0)
	for _, criterion := range criteria {
		if matchesAllGroups(text, criterion.groups) {
			hit++
		} else {
			missing = append(missing, criterion.name)
		}
	}
	score := float64(hit) / float64(len(criteria))
	if score >= 0.7 {
		return score, fmt.Sprintf("covers %d/%d required comparison dimensions", hit, len(criteria))
	}
	return score, "missing: " + strings.Join(missing, "; ")
}

func matchesAllGroups(text string, groups [][]string) bool {
	for _, group := range groups {
		if !containsAny(text, group) {
			return false
		}
	}
	return true
}

func containsAny(text string, values []string) bool {
	for _, value := range values {
		if strings.Contains(text, strings.ToLower(value)) {
			return true
		}
	}
	return false
}
