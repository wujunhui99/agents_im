package model

import "github.com/zeromicro/go-zero/core/stores/sqlx"

var ErrNotFound = sqlx.ErrNotFound

// agent_trigger_idempotency.status 的 DB 取值（单一来源）：running 占用中、succeeded/failed
// 为终态。TryStart 的 TTL 抢占与 Finish 的终态推进 SQL 引用这些字面量，domain 层（aghosting）
// 复用为 owner 视图常量。
const (
	AgentTriggerStatusRunning   = "running"
	AgentTriggerStatusSucceeded = "succeeded"
	AgentTriggerStatusFailed    = "failed"
)
