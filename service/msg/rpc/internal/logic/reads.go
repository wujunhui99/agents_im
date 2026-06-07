package logic

import (
	"context"
	"errors"

	"github.com/wujunhui99/agents_im/pkg/apperror"
	"github.com/wujunhui99/agents_im/service/msg/rpc/internal/model"
	"github.com/wujunhui99/agents_im/service/msg/rpc/internal/svc"
)

// loadConversationSeqStates 移植自 internal/repository.GetConversationSeqStates：
// 指定会话列表时逐个定向修复；未指定时取该用户全部会话（必要时全量修复）。
func loadConversationSeqStates(ctx context.Context, svcCtx *svc.ServiceContext, userID string, conversationIDs []string) ([]model.ConversationSeqState, error) {
	ids := conversationIDs
	needsTargetedRepair := len(ids) > 0
	if len(ids) == 0 {
		listed, err := svcCtx.States.ListConversationIDs(ctx, userID)
		if err != nil {
			return nil, err
		}
		ids = listed
		if len(ids) == 0 {
			if err := svcCtx.States.RepairAllDirect(ctx, userID); err != nil {
				return nil, err
			}
			ids, err = svcCtx.States.ListConversationIDs(ctx, userID)
			if err != nil {
				return nil, err
			}
		}
	}

	states := make([]model.ConversationSeqState, 0, len(ids))
	for _, conversationID := range ids {
		if needsTargetedRepair {
			if err := svcCtx.States.RepairDirect(ctx, userID, conversationID); err != nil {
				return nil, err
			}
		}
		state, err := svcCtx.States.QuerySeqState(ctx, userID, conversationID)
		if err != nil {
			if errors.Is(err, model.ErrNotFound) {
				return nil, apperror.NotFound("conversation not found")
			}
			return nil, err
		}
		states = append(states, state.Clone())
	}
	return states, nil
}

// loadMessagesForUser 移植自 internal/repository.GetMessagesForUser：按用户可见边界裁剪 seq 区间后分页。
func loadMessagesForUser(ctx context.Context, svcCtx *svc.ServiceContext, userID, conversationID string, fromSeq, toSeq int64, limit int, order string) ([]*model.Messages, bool, int64, error) {
	if err := svcCtx.States.RepairDirect(ctx, userID, conversationID); err != nil {
		return nil, false, 0, err
	}
	bounds, err := svcCtx.States.UserScopedBounds(ctx, userID, conversationID)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return nil, false, 0, apperror.NotFound("conversation not found")
		}
		return nil, false, 0, err
	}
	if fromSeq <= bounds.VisibleStartSeq {
		fromSeq = bounds.VisibleStartSeq + 1
	}
	if toSeq <= 0 || toSeq > bounds.MaxSeq {
		toSeq = bounds.MaxSeq
	}
	return svcCtx.Messages.GetMessagesInRange(ctx, conversationID, fromSeq, toSeq, bounds.MaxSeq, limit, order)
}
