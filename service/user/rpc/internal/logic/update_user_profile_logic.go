package logic

import (
	"context"
	"strings"

	"github.com/wujunhui99/agents_im/common/share/rpcerror"
	"github.com/wujunhui99/agents_im/pkg/apperror"
	"github.com/wujunhui99/agents_im/service/user/rpc/internal/model"
	"github.com/wujunhui99/agents_im/service/user/rpc/internal/svc"
	userpb "github.com/wujunhui99/agents_im/service/user/rpc/user"

	"github.com/zeromicro/go-zero/core/logx"
)

type UpdateUserProfileLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUpdateUserProfileLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateUserProfileLogic {
	return &UpdateUserProfileLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *UpdateUserProfileLogic) UpdateUserProfile(in *userpb.UpdateUserProfileRequest) (*userpb.UserResponse, error) {
	userID := strings.TrimSpace(in.GetUserId())
	if userID == "" {
		return nil, rpcerror.ToStatus(apperror.InvalidArgument("user_id is required"))
	}

	patch, err := buildProfilePatch(in)
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}

	if err := l.svcCtx.Profiles.UpdateProfileFields(l.ctx, userID, patch); err != nil {
		return nil, rpcerror.ToStatus(mapReadError(err))
	}
	ap, err := l.svcCtx.Accounts.FindAccountProfileByID(l.ctx, userID)
	if err != nil {
		return nil, rpcerror.ToStatus(mapReadError(err))
	}
	return toUserResponse(ap), nil
}

// buildProfilePatch 把 pb optional 字段映射为 DB-ready 补丁，并保留 monolith 的回填规则：
// display_name/name 任一缺省时互补（仅设其一时，另一项同步成同值）。
func buildProfilePatch(in *userpb.UpdateUserProfileRequest) (model.ProfilePatch, error) {
	patch := model.ProfilePatch{}
	if in.DisplayName != nil {
		value, err := validateProfileName(*in.DisplayName, "display_name")
		if err != nil {
			return model.ProfilePatch{}, err
		}
		patch.DisplayName = &value
		if in.Name == nil {
			patch.Name = &value
		}
	}
	if in.Name != nil {
		value, err := validateProfileName(*in.Name, "name")
		if err != nil {
			return model.ProfilePatch{}, err
		}
		patch.Name = &value
		if in.DisplayName == nil {
			patch.DisplayName = &value
		}
	}
	if in.Gender != nil {
		value, err := validateGender(*in.Gender)
		if err != nil {
			return model.ProfilePatch{}, err
		}
		dbValue := genderToDB(value)
		patch.Gender = &dbValue
	}
	if in.BirthDate != nil {
		value := strings.TrimSpace(*in.BirthDate)
		patch.BirthDate = &value
	}
	if in.Region != nil {
		value, err := validateRegion(*in.Region)
		if err != nil {
			return model.ProfilePatch{}, err
		}
		patch.Region = &value
	}
	return patch, nil
}
