// Package feedbackstore owns the feedback data layer for admin-rpc: the feedback
// table (migration 015). admin is the feedback owner (#463 moved CreateFeedback
// into admin-rpc), so this replaces the keystone internal/repository.FeedbackRepository
// + internal/logic.FeedbackLogic outright (issue #678, split from #618/#394/#344) —
// admin-rpc no longer imports internal/ feedback.
//
// Store is the data interface (goctl model backed in prod via ModelStore, in-memory
// in tests/demo via MemoryStore). admin-rpc feedback Logic depends on this interface.
package feedbackstore

import (
	"context"

	"github.com/wujunhui99/agents_im/pkg/model"
)

// ListFilter narrows ListFeedback（query spec, not a domain entity）。
type ListFilter struct {
	Status model.FeedbackStatus
	Limit  int
	Offset int
}

// Store 是 feedback 表的数据访问接口：用户提交（CreateFeedback，经 msg-api BFF gRPC 调入）+
// admin triage 只读/更新（List/Get/UpdateFeedback）。prod 注入 ModelStore（goctl），单测注入 MemoryStore。
type Store interface {
	CreateFeedback(ctx context.Context, feedback model.Feedback) (model.Feedback, error)
	ListFeedback(ctx context.Context, filter ListFilter) ([]model.Feedback, error)
	GetFeedback(ctx context.Context, feedbackID string) (model.Feedback, error)
	UpdateFeedback(ctx context.Context, feedback model.Feedback) (model.Feedback, error)
}
