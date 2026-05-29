package groups

import (
	"strings"

	"github.com/wujunhui99/agents_im/pkg/apperror"
	"github.com/wujunhui99/agents_im/service/groups/api/internal/types"
	groupspb "github.com/wujunhui99/agents_im/service/groups/rpc/groups"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func groupResp(group *groupspb.Group) *types.GroupResp {
	return &types.GroupResp{Code: string(apperror.CodeOK), Message: "ok", Data: toGroup(group)}
}

func toGroup(group *groupspb.Group) types.Group {
	if group == nil {
		return types.Group{}
	}
	return types.Group{
		GroupID:         group.GetGroupId(),
		Name:            group.GetName(),
		Description:     group.GetDescription(),
		Announcement:    group.GetAnnouncement(),
		AvatarMediaID:   group.GetAvatarMediaId(),
		AvatarURL:       group.GetAvatarUrl(),
		CreatorUserID:   group.GetCreatorUserId(),
		CurrentUserRole: group.GetCurrentUserRole(),
		CreatedAt:       group.GetCreatedAt(),
		UpdatedAt:       group.GetUpdatedAt(),
	}
}

func memberResp(resp *groupspb.MemberResponse) *types.MemberResp {
	if resp == nil {
		resp = &groupspb.MemberResponse{}
	}
	return &types.MemberResp{Code: string(apperror.CodeOK), Message: "ok", Data: types.MemberData{Member: toGroupMember(resp.GetMember()), AlreadyMember: resp.GetAlreadyMember()}}
}

func toGroupMember(member *groupspb.GroupMember) types.GroupMember {
	if member == nil {
		return types.GroupMember{}
	}
	return types.GroupMember{
		GroupID:       member.GetGroupId(),
		UserID:        member.GetUserId(),
		Role:          member.GetRole(),
		State:         member.GetState(),
		JoinedAt:      member.GetJoinedAt(),
		LeftAt:        member.GetLeftAt(),
		Identifier:    member.GetIdentifier(),
		DisplayName:   member.GetDisplayName(),
		Name:          member.GetName(),
		AvatarMediaID: member.GetAvatarMediaId(),
		AvatarURL:     member.GetAvatarUrl(),
	}
}

func apiError(err error) error {
	if err == nil {
		return nil
	}
	if appErr := apperror.From(err); appErr.Code != apperror.CodeInternal || strings.HasPrefix(err.Error(), string(apperror.CodeInternal)+":") {
		return err
	}
	st, ok := status.FromError(err)
	if !ok {
		return err
	}
	switch st.Code() {
	case codes.InvalidArgument:
		return apperror.InvalidArgument(st.Message())
	case codes.Unauthenticated:
		return apperror.Unauthenticated(st.Message())
	case codes.PermissionDenied:
		return apperror.Forbidden(st.Message())
	case codes.NotFound:
		return apperror.NotFound(st.Message())
	case codes.AlreadyExists:
		return apperror.AlreadyExists(st.Message())
	case codes.ResourceExhausted:
		return apperror.RateLimited(st.Message())
	case codes.Unavailable:
		return apperror.ServiceUnavailable(st.Message())
	default:
		return apperror.Internal("internal server error")
	}
}
