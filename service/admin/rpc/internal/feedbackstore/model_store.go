package feedbackstore

import (
	"context"

	"github.com/zeromicro/go-zero/core/stores/postgres"
	"github.com/zeromicro/go-zero/core/stores/sqlx"

	domain "github.com/wujunhui99/agents_im/pkg/model"
	"github.com/wujunhui99/agents_im/service/admin/rpc/internal/model"
)

// ModelStore 是 Store 的 goctl model 实现（背靠 feedback 表）。各方法直接委托 model.FeedbackModel
// 的 custom 方法（bigint↔string `::text` 约定 + jsonb client_meta 编解码在 model 层完成）。
type ModelStore struct {
	feedback model.FeedbackModel
}

var _ Store = (*ModelStore)(nil)

// NewModelStore 用数据源构建 model-backed Store。
func NewModelStore(dataSource string) *ModelStore {
	return NewModelStoreFromConn(postgres.New(dataSource))
}

// NewModelStoreFromConn 用已建连接构建 model-backed Store。
func NewModelStoreFromConn(conn sqlx.SqlConn) *ModelStore {
	return &ModelStore{feedback: model.NewFeedbackModel(conn)}
}

func (s *ModelStore) CreateFeedback(ctx context.Context, feedback domain.Feedback) (domain.Feedback, error) {
	return s.feedback.CreateFeedback(ctx, feedback)
}

func (s *ModelStore) ListFeedback(ctx context.Context, filter ListFilter) ([]domain.Feedback, error) {
	return s.feedback.ListFeedback(ctx, model.FeedbackListFilter{
		Status: filter.Status,
		Limit:  filter.Limit,
		Offset: filter.Offset,
	})
}

func (s *ModelStore) GetFeedback(ctx context.Context, feedbackID string) (domain.Feedback, error) {
	return s.feedback.GetFeedback(ctx, feedbackID)
}

func (s *ModelStore) UpdateFeedback(ctx context.Context, feedback domain.Feedback) (domain.Feedback, error) {
	return s.feedback.UpdateFeedback(ctx, feedback)
}
