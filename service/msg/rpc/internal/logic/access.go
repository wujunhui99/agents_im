package logic

import (
	"context"

	business "github.com/wujunhui99/agents_im/internal/logic"
	"github.com/wujunhui99/agents_im/pkg/apperror"
	"github.com/wujunhui99/agents_im/service/msg/rpc/internal/model"
)

// ensureConversationReadAccess 校验调用者可读该会话（移植自 messagelogic.go）：
// 单聊看 conversation_id 里的双方；群聊调 internal GroupsLogic（keystone 例外）。
func ensureConversationReadAccess(ctx context.Context, groups business.GroupMemberLister, userID, conversationID string) error {
	userA, userB, ok := singleConversationParticipants(conversationID)
	if ok {
		if userID != userA && userID != userB {
			return apperror.Forbidden("caller is not a conversation participant")
		}
		return nil
	}

	groupID, ok := groupIDFromConversationID(conversationID)
	if !ok {
		return nil
	}
	if groups == nil {
		return apperror.Internal("group membership validator is not configured")
	}
	_, err := groups.ListMembers(ctx, business.ListMembersRequest{
		GroupID:         groupID,
		RequesterUserID: userID,
	})
	return err
}

// filterReadableConversationStates 丢弃调用者无权访问的会话（Forbidden/NotFound），其余错误上抛。
func filterReadableConversationStates(ctx context.Context, groups business.GroupMemberLister, userID string, states []model.ConversationSeqState) ([]model.ConversationSeqState, error) {
	filtered := make([]model.ConversationSeqState, 0, len(states))
	for _, state := range states {
		err := ensureConversationReadAccess(ctx, groups, userID, state.ConversationID)
		if err == nil {
			filtered = append(filtered, state)
			continue
		}
		code := apperror.From(err).Code
		if code == apperror.CodeForbidden || code == apperror.CodeNotFound {
			continue
		}
		return nil, err
	}
	return filtered, nil
}
