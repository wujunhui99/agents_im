package friends

import (
	"strings"

	"github.com/wujunhui99/agents_im/pkg/apperror"
	"github.com/wujunhui99/agents_im/service/friends/api/internal/types"
	friendspb "github.com/wujunhui99/agents_im/service/friends/rpc/friends"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func friendshipFromRPC(friendship *friendspb.Friendship) (types.Friendship, error) {
	if friendship == nil {
		return types.Friendship{}, apperror.Internal("friends rpc returned empty friendship")
	}
	view := types.Friendship{
		UserID:    friendship.GetUserId(),
		FriendID:  friendship.GetFriendId(),
		Status:    friendship.GetStatus(),
		IsFriend:  friendship.GetIsFriend(),
		CreatedAt: friendship.GetCreatedAt(),
		UpdatedAt: friendship.GetUpdatedAt(),
	}
	if friendship.GetFriend() != nil {
		view.Friend = friendProfileFromRPC(friendship.GetFriend())
	}
	return view, nil
}

func friendProfileFromRPC(profile *friendspb.FriendProfile) types.FriendProfile {
	if profile == nil {
		return types.FriendProfile{}
	}
	return types.FriendProfile{
		UserID:             profile.GetUserId(),
		Identifier:         profile.GetIdentifier(),
		DisplayName:        profile.GetDisplayName(),
		Name:               profile.GetName(),
		Gender:             profile.GetGender(),
		BirthDate:          profile.GetBirthDate(),
		Region:             profile.GetRegion(),
		AccountType:        profile.GetAccountType(),
		AvatarMediaID:      profile.GetAvatarMediaId(),
		AvatarURL:          profile.GetAvatarUrl(),
		AvatarURLExpiresAt: profile.GetAvatarUrlExpiresAt(),
		CreatedAt:          profile.GetCreatedAt(),
		UpdatedAt:          profile.GetUpdatedAt(),
	}
}

func friendshipsFromRPC(items []*friendspb.Friendship) ([]types.Friendship, error) {
	out := make([]types.Friendship, 0, len(items))
	for _, item := range items {
		friendship, err := friendshipFromRPC(item)
		if err != nil {
			return nil, err
		}
		out = append(out, friendship)
	}
	return out, nil
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
