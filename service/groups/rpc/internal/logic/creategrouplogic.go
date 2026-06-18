package logic

import (
	"context"

	"github.com/wujunhui99/agents_im/pkg/idgen"
	"github.com/wujunhui99/agents_im/pkg/rpcerror"
	groups "github.com/wujunhui99/agents_im/service/groups/rpc/groups"
	"github.com/wujunhui99/agents_im/service/groups/rpc/internal/model"
	"github.com/wujunhui99/agents_im/service/groups/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type CreateGroupLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCreateGroupLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateGroupLogic {
	return &CreateGroupLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

func (l *CreateGroupLogic) CreateGroup(in *groups.CreateGroupRequest) (*groups.GroupResponse, error) {
	creatorUserID, err := validateRequiredID(in.GetCreatorUserId(), "creator_user_id")
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	name, err := validateGroupName(in.GetName())
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	description, err := validateGroupDescription(in.GetDescription())
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	memberUserIDs, err := validateGroupMemberIDs(creatorUserID, in.GetMemberUserIds())
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}

	groupID, err := idgen.NewString()
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}

	// 事务边界在 Logic 层：建群与写入成员要么全成、要么全败。
	var created *model.Groups
	err = l.svcCtx.GroupsModel.Transact(l.ctx, func(ctx context.Context, session sqlx.Session) error {
		groupsModel := l.svcCtx.GroupsModel.WithSession(session)
		membersModel := l.svcCtx.GroupMembersModel.WithSession(session)

		g, err := groupsModel.InsertGroup(ctx, &model.Groups{
			GroupId:          groupID,
			Name:             name,
			Description:      description,
			CreatorAccountId: creatorUserID,
		})
		if err != nil {
			return err
		}
		created = g

		for _, userID := range memberUserIDs {
			role := model.MemberRoleMember
			if userID == creatorUserID {
				role = model.MemberRoleOwner
			}
			if _, err := membersModel.UpsertActiveMember(ctx, created.GroupId, userID, role); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	return &groups.GroupResponse{Group: toGroup(created, "")}, nil
}
