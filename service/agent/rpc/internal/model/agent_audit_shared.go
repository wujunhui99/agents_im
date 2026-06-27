package model

// agent_audit_shared.go 是 agent 审计四表（agent_runs / agent_tool_calls / agent_file_reads /
// agent_python_execs）custom 方法的共享底座：summary jsonb 编解码、时间归一、bigint keystone
// 的 ID 生成，以及插入冲突 → apperror 的映射。审计表是 append-only（migration 002 触发器拒绝
// UPDATE/DELETE），故只暴露 insert + 只读，不复用 goctl 生成的 Update/Delete。
// bigint 列 ↔ string ID 沿用 #013/#550 的 keystone `::text` 约定（写 `$n::bigint`、读 `col::text`）。

import (
	"database/sql"
	"encoding/json"
	"strconv"
	"time"

	"github.com/wujunhui99/agents_im/pkg/agentaudit"
	"github.com/wujunhui99/agents_im/pkg/apperror"
	"github.com/wujunhui99/agents_im/pkg/idgen"
)

func marshalAgentAuditSummary(summary agentaudit.Summary) ([]byte, error) {
	redacted, err := agentaudit.RedactSummary(summary)
	if err != nil {
		return nil, err
	}
	if redacted == nil {
		redacted = agentaudit.Summary{}
	}
	return json.Marshal(redacted)
}

func unmarshalAgentAuditSummary(raw []byte) (agentaudit.Summary, error) {
	if len(raw) == 0 {
		return agentaudit.Summary{}, nil
	}
	var summary map[string]any
	if err := json.Unmarshal(raw, &summary); err != nil {
		return nil, err
	}
	return agentaudit.Summary(summary), nil
}

func defaultAuditTime(value time.Time, fallback time.Time) time.Time {
	if value.IsZero() {
		return fallback
	}
	return value.UTC()
}

func nullableAuditTime(value time.Time) any {
	if value.IsZero() {
		return nil
	}
	return value.UTC()
}

func nullTimeUTC(value sql.NullTime) time.Time {
	if !value.Valid {
		return time.Time{}
	}
	return value.Time.UTC()
}

// mapAgentAuditInsertError 把 append-only 插入的 pg 约束冲突映射成业务错误：唯一 → AlreadyExists、
// 外键 → NotFound（run 不存在）、check → InvalidArgument，其余原样返回。
func mapAgentAuditInsertError(err error, alreadyExistsMessage string, notFoundMessage string) error {
	switch {
	case IsUniqueViolation(err):
		return apperror.AlreadyExists(alreadyExistsMessage)
	case IsForeignKeyViolation(err):
		return apperror.NotFound(notFoundMessage)
	case IsCheckViolation(err):
		return apperror.InvalidArgument("invalid agent audit record")
	default:
		return err
	}
}

// normalizeAgentAuditLimit 把 admin 列表 limit 收敛到 [1, max]，0/负数取 fallback。
func normalizeAgentAuditLimit(value int, fallback int, max int) int {
	if value <= 0 {
		return fallback
	}
	if value > max {
		return max
	}
	return value
}

func agentAuditItoa(value int) string {
	return strconv.Itoa(value)
}

// newAgentAuditID 生成审计行的 bigint keystone ID（snowflake string，写库时 `$n::bigint` 转入）。
func newAgentAuditID() (string, error) {
	return idgen.NewString()
}
