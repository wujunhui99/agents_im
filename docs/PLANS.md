# PLANS.md

本文档定义 Planner、Generator、Evaluator 的协作方式，以及执行计划的存放和维护规则。

## Planner

Planner 对单个需求生成三类文档：

1. 需求文档：只描述业务逻辑，不涉及具体技术实现。
2. 实现文档：描述技术方案、架构设计、接口设计、数据模型等实现细节。
3. 执行文档：将需求拆分为多个 Task，说明执行顺序、验收标准和验证方式。

## Generator

Generator 根据 Planner 的文档完成实现，并进行自测。自测应覆盖：

- 单元测试
- 集成测试
- 接口测试
- 本地启动验证
- 关键链路验证

## Evaluator

Evaluator 检查 Generator 的实现结果，包括：

- 功能是否满足需求
- 代码是否符合架构和质量要求
- 测试是否充分
- 文档是否同步更新
- 是否存在安全、可靠性或可维护性问题

## GitHub Issue / Project 驱动

新的复杂需求默认不再只依赖本地执行计划。GitHub Issue 是需求规格和验收标准的单一事实源，GitHub Project 是状态和调度中心。完整规则见 [`docs/AGENTIC_DEVELOPMENT_WORKFLOW.md`](./AGENTIC_DEVELOPMENT_WORKFLOW.md)。

Planner/Codex Spec Mode 的输出必须进入 Issue，并包含 Background、User Story、Goals、Non-goals、Functional Scope、Interaction Flow、Product Usability Requirements、Data/API Impact、Edge Cases、Acceptance Criteria、Test Plan 和 Dependencies。

Generator/Codex Dev Mode 只能从 `Ready for Dev` Issue 开发，并在完成后回写 Branch、PR、Acceptance Criteria Check、Tests、Risks 和 Remaining Work。

Evaluator/Hermes 对照 Issue 验收标准检查。PR merge 之后仍需必要 CI/CD 和生产验证，才能把 Issue 推到 Done。

## 执行计划模板

> 执行计划已统一改用 GitHub Issue 管理，不再维护本地 `docs/exec-plans/` 目录。以下模板可作为 Issue 正文的结构参考。

```md
# <task-name>

状态：Active | Completed | Blocked

## 背景

## 目标

## 非目标

## 任务拆分

- [ ] Task 1
- [ ] Task 2

## 决策日志

| 时间 | 决策 | 原因 |
| --- | --- | --- |

## 验证方式

## 风险与回滚

## 结果记录
```
