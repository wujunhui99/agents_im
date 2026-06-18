package logic

import (
	"context"
	"errors"

	"github.com/wujunhui99/agents_im/pkg/rpcerror"
	"github.com/wujunhui99/agents_im/pkg/apperror"
	"github.com/wujunhui99/agents_im/service/msg/rpc/internal/model"
	"github.com/wujunhui99/agents_im/service/msg/rpc/internal/svc"
	"github.com/wujunhui99/agents_im/service/msg/rpc/msg"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type MarkConversationAsReadLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewMarkConversationAsReadLogic(ctx context.Context, svcCtx *svc.ServiceContext) *MarkConversationAsReadLogic {
	return &MarkConversationAsReadLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// MarkConversationAsRead 把整会话已读推进到 has_read_seq（移植自 messagelogic + repo.SetUserHasReadSeqMax）。
func (l *MarkConversationAsReadLogic) MarkConversationAsRead(in *msg.MarkConversationAsReadRequest) (*msg.MarkConversationAsReadResponse, error) {
	userID, err := normalizeMessageRequiredID(in.GetUserId(), "user_id")
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	conversationID, err := normalizeConversationID(in.GetConversationId())
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	if err := ensureConversationReadAccess(l.ctx, l.svcCtx.Groups, userID, conversationID); err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	if in.GetHasReadSeq() < 0 {
		return nil, rpcerror.ToStatus(apperror.InvalidArgument("has_read_seq must be greater than or equal to 0"))
	}

	state, updated, err := l.setHasReadSeqMax(userID, conversationID, in.GetHasReadSeq())
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	return &msg.MarkConversationAsReadResponse{
		ConversationId: state.ConversationID,
		HasReadSeq:     state.HasReadSeq,
		MaxSeq:         state.MaxSeq,
		UnreadCount:    state.UnreadCount,
		Updated:        updated,
	}, nil
}

// setHasReadSeqMax 在 FOR UPDATE 事务内推进已读 seq（移植自 internal/repository.SetUserHasReadSeqMax）。
func (l *MarkConversationAsReadLogic) setHasReadSeqMax(userID, conversationID string, seq int64) (model.ConversationSeqState, bool, error) {
	if err := l.svcCtx.States.RepairDirect(l.ctx, userID, conversationID); err != nil {
		return model.ConversationSeqState{}, false, err
	}

	var state model.ConversationSeqState
	updated := false
	err := l.svcCtx.States.Transact(l.ctx, func(ctx context.Context, session sqlx.Session) error {
		states := l.svcCtx.States.WithSession(session)
		bounds, err := states.LockReadState(ctx, userID, conversationID)
		if err != nil {
			return err
		}
		if seq > bounds.MaxSeq {
			return apperror.InvalidArgument("has_read_seq cannot exceed max_seq")
		}
		if seq < bounds.VisibleStartSeq {
			seq = bounds.VisibleStartSeq
		}
		updated = seq > bounds.HasReadSeq
		if err := states.AdvanceLastReadSeq(ctx, userID, conversationID, seq); err != nil {
			return err
		}
		next, err := states.QuerySeqState(ctx, userID, conversationID)
		if err != nil {
			return err
		}
		state = next
		return nil
	})
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return model.ConversationSeqState{}, false, apperror.NotFound("conversation not found")
		}
		return model.ConversationSeqState{}, false, err
	}
	return state.Clone(), updated, nil
}
