package logic

import (
	"context"
	"errors"

	"github.com/wujunhui99/agents_im/pkg/rpcerror"
	groups "github.com/wujunhui99/agents_im/service/groups/rpc/groups"
	"github.com/wujunhui99/agents_im/service/groups/rpc/internal/model"
	"github.com/wujunhui99/agents_im/service/groups/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type IsMemberLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewIsMemberLogic(ctx context.Context, svcCtx *svc.ServiceContext) *IsMemberLogic {
	return &IsMemberLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

// IsMember 判定 user_id 是否为 group_id 的 active 成员（媒体下载授权 §4 群聊校验）。
// 非成员/已退群返回 is_member=false（不报错，由调用方决定拒绝语义），群不存在同样返回 false。
func (l *IsMemberLogic) IsMember(in *groups.IsMemberRequest) (*groups.IsMemberResponse, error) {
	groupID, err := validateRequiredID(in.GetGroupId(), "group_id")
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	userID, err := validateRequiredID(in.GetUserId(), "user_id")
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}

	row, err := l.svcCtx.GroupMembersModel.FindOneByGroupIdAccountId(l.ctx, groupID, userID)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return &groups.IsMemberResponse{IsMember: false}, nil
		}
		return nil, rpcerror.ToStatus(err)
	}
	return &groups.IsMemberResponse{IsMember: row.Status == model.MemberStatusActive}, nil
}
