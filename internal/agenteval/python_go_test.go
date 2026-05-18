package agenteval

import (
	"context"
	"testing"
)

func TestPythonGoPerformanceJudgeFailsVagueReply(t *testing.T) {
	result, err := DefaultJudge().Evaluate(context.Background(), EvaluationRequest{
		CaseID: PythonGoPerformanceCaseID,
		Input:  "你能帮我对比一下 Python 和 Go 语言的性能吗？",
		Output: "可以，你简单说说吧。",
	})
	if err != nil {
		t.Fatalf("evaluate vague reply: %v", err)
	}
	if result.Pass {
		t.Fatalf("vague reply should fail: %+v", result)
	}
	if result.Score >= 0.7 {
		t.Fatalf("vague reply score = %.2f, want below passing threshold", result.Score)
	}
	if result.Reason == "" {
		t.Fatalf("result should explain failure: %+v", result)
	}
}

func TestPythonGoPerformanceJudgePassesUsefulComparison(t *testing.T) {
	answer := `一般来说，Go 在 CPU 密集型和服务端场景里执行速度更快，因为 Go 编译成本机机器码，运行时开销较小；Python 主要由解释器/VM 执行，热点性能通常依赖 C 扩展、PyPy 或向量化库。
并发方面，Go 的 goroutine 和 channel 适合大量网络并发；Python 线程受 GIL 影响，CPU 并行通常用 multiprocessing、asyncio 处理 I/O，或把重计算交给 NumPy 等原生扩展。
启动和部署上，Go 常能交付单个静态二进制，启动快；Python 需要解释器和依赖环境，部署更依赖虚拟环境或容器。内存方面 Go runtime 有 GC 和调度器开销，但长期服务通常比 Python 进程更省；Python 对象模型内存占用较高。
生态和场景上，Python 在数据科学、脚本、AI 生态很强，Go 更适合高并发后端、基础设施、CLI 和云原生服务。结论是性能敏感服务优先考虑 Go，数据分析和快速迭代优先 Python，最终取决于业务场景。`
	result, err := DefaultJudge().Evaluate(context.Background(), EvaluationRequest{
		CaseID: PythonGoPerformanceCaseID,
		Input:  "你能帮我对比一下 Python 和 Go 语言的性能吗？",
		Output: answer,
	})
	if err != nil {
		t.Fatalf("evaluate useful reply: %v", err)
	}
	if !result.Pass {
		t.Fatalf("useful comparison should pass: %+v", result)
	}
	if result.Score < 0.7 {
		t.Fatalf("useful comparison score = %.2f, want passing score", result.Score)
	}
}

func TestJudgeResultContainsCaseScoreReasonAndTraceLinkage(t *testing.T) {
	result, err := DefaultJudge().Evaluate(context.Background(), EvaluationRequest{
		CaseID:     PythonGoPerformanceCaseID,
		Input:      "你能帮我对比一下 Python 和 Go 语言的性能吗？",
		Output:     "Go 编译成本机码，通常比解释执行的 Python 快；goroutine 适合并发，Python 有 GIL 但可用 multiprocessing 和 C 扩展；Go 单二进制部署启动快，Python 依赖环境更灵活；Go 内存开销通常更低，Python 生态适合数据科学。选择取决于场景。",
		TraceID:    "trace_eval_1",
		AgentRunID: "run_eval_1",
	})
	if err != nil {
		t.Fatalf("evaluate with linkage: %v", err)
	}
	if result.CaseID != PythonGoPerformanceCaseID || result.Score == 0 || result.Reason == "" {
		t.Fatalf("result missing required fields: %+v", result)
	}
	if result.TraceID != "trace_eval_1" || result.AgentRunID != "run_eval_1" {
		t.Fatalf("result missing trace/run linkage: %+v", result)
	}
}
