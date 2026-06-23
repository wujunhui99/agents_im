package convhosting

import (
	"context"
	"errors"

	"github.com/zeromicro/go-zero/core/stores/postgres"
	"github.com/zeromicro/go-zero/core/stores/sqlx"

	"github.com/wujunhui99/agents_im/pkg/apperror"
	"github.com/wujunhui99/agents_im/service/agent/rpc/internal/model"
)

// ModelStore 是 Store 的 goctl model 实现（背靠 conversation_ai_hosting_settings 表）。
type ModelStore struct {
	model model.ConversationAiHostingSettingsModel
}

var _ Store = (*ModelStore)(nil)

// NewModelStore 用数据源构建 model-backed Store。
func NewModelStore(dataSource string) *ModelStore {
	return NewModelStoreFromConn(postgres.New(dataSource))
}

// NewModelStoreFromConn 用已建连接构建 model-backed Store。
func NewModelStoreFromConn(conn sqlx.SqlConn) *ModelStore {
	return &ModelStore{model: model.NewConversationAiHostingSettingsModel(conn)}
}

func (s *ModelStore) GetConversationAIHostingSetting(ctx context.Context, ownerAccountID string, conversationID string) (Setting, error) {
	row, err := s.model.FindOneByOwnerAccountIdConversationId(ctx, ownerAccountID, conversationID)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return Setting{}, apperror.NotFound("conversation AI hosting setting not found")
		}
		return Setting{}, err
	}
	return settingFromModel(row), nil
}

func (s *ModelStore) GetEnabledConversationAIHosting(ctx context.Context, conversationID string) (Setting, error) {
	row, err := s.model.FindEnabledByConversationId(ctx, conversationID)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return Setting{}, apperror.NotFound("enabled conversation AI hosting setting not found")
		}
		return Setting{}, err
	}
	return settingFromModel(row), nil
}

func (s *ModelStore) SetConversationAIHostingEnabled(ctx context.Context, input Update) (Setting, error) {
	row, err := s.model.Upsert(ctx, &model.ConversationAiHostingSettings{
		OwnerAccountId:    input.OwnerAccountID,
		ConversationId:    input.ConversationID,
		Enabled:           input.Enabled,
		Mode:              modeAutoReply,
		MaxRecentMessages: int64(clampRecentMessages(input.MaxRecentMessages)),
		SummaryEnabled:    input.SummaryEnabled,
	})
	if err != nil {
		if model.IsUniqueViolation(err) {
			return Setting{}, conflictError()
		}
		if model.IsCheckViolation(err) {
			return Setting{}, apperror.InvalidArgument("invalid conversation AI hosting setting")
		}
		return Setting{}, err
	}
	return settingFromModel(row), nil
}

func settingFromModel(row *model.ConversationAiHostingSettings) Setting {
	return Setting{
		OwnerAccountID:    row.OwnerAccountId,
		ConversationID:    row.ConversationId,
		Enabled:           row.Enabled,
		Mode:              row.Mode,
		MaxRecentMessages: int(row.MaxRecentMessages),
		SummaryEnabled:    row.SummaryEnabled,
		CreatedAt:         row.CreatedAt.UTC(),
		UpdatedAt:         row.UpdatedAt.UTC(),
	}
}
