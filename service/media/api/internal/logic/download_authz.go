// Package logic holds media-api(BFF) 的下载授权编排（EPIC #527 §4，#532）。
//
// 微服务分层约定（见 AGENTS.md）：跨域聚合在 api(BFF) 层做，rpc 之间不互相调用。media-rpc 只暴露
// GetMedia（元数据读）+ GetDownloadURL（纯签发），下载授权所需的 msg/friends/groups 跨域事实由
// media-api 在这里聚合判定，避免 media-rpc↔msg-rpc 形成 rpc 调用环。
package logic

import (
	"context"
	"strings"

	"github.com/wujunhui99/agents_im/pkg/apperror"
	"github.com/wujunhui99/agents_im/service/friends/rpc/friendsclient"
	"github.com/wujunhui99/agents_im/service/groups/rpc/groupsclient"
	"github.com/wujunhui99/agents_im/service/media/rpc/mediaclient"
	"github.com/wujunhui99/agents_im/service/msg/rpc/msgclient"
	"google.golang.org/grpc"
)

// chat_type 取值（msg-rpc GetMessageRef 返回，对齐 msg model.ChatTypeSingle/Group）。
const (
	chatTypeSingle = "single"
	chatTypeGroup  = "group"
)

// 下面四个窄接口只取下载授权需要的方法，真实 zrpc 客户端天然满足；单测用各自的 fake 实现。
type (
	mediaMetaGetter interface {
		GetMedia(ctx context.Context, in *mediaclient.GetMediaRequest, opts ...grpc.CallOption) (*mediaclient.MediaObject, error)
	}
	messageRefGetter interface {
		GetMessageRef(ctx context.Context, in *msgclient.GetMessageRefRequest, opts ...grpc.CallOption) (*msgclient.GetMessageRefResponse, error)
	}
	friendshipGetter interface {
		GetFriendship(ctx context.Context, in *friendsclient.GetFriendshipRequest, opts ...grpc.CallOption) (*friendsclient.GetFriendshipResponse, error)
	}
	memberChecker interface {
		IsMember(ctx context.Context, in *groupsclient.IsMemberRequest, opts ...grpc.CallOption) (*groupsclient.IsMemberResponse, error)
	}
)

// authorizeDownload 实现下载授权编排（EPIC #527 §4），授权通过返回 nil，否则返回 apperror/透传 rpc 错误：
//  1. GetMedia → uploader == requester 快速放行（含「我删了对方仍能下我自己发的文件」）；
//  2. 否则 GetMessageRef(msg_id) 取消息引用做**链路校验**：消息真正引用的 media_id 必须等于入参
//     media_id（media_id/msg_id 客户端各自传入，不校验就能用合法 msg_id 配任意别人的 media_id 越权）；
//  3. 私聊 → friends 单向好友校验（只看 requester→peer，对方删 requester 不影响）；
//     群聊 → groups 成员校验；失败拒绝。语义按当前关系（删好友/退群后不可再下历史文件）。
func authorizeDownload(ctx context.Context, requester, mediaID, msgID string, mediaCli mediaMetaGetter, msgCli messageRefGetter, friendCli friendshipGetter, groupCli memberChecker) error {
	meta, err := mediaCli.GetMedia(ctx, &mediaclient.GetMediaRequest{MediaId: mediaID})
	if err != nil {
		return err
	}
	if meta.GetOwnerUserId() == requester {
		return nil
	}

	if strings.TrimSpace(msgID) == "" {
		return apperror.InvalidArgument("msg_id is required to download media you did not upload")
	}
	ref, err := msgCli.GetMessageRef(ctx, &msgclient.GetMessageRefRequest{ServerMsgId: msgID, RequesterAccountId: requester})
	if err != nil {
		return err
	}

	// 链路校验：入参 media 必须等于消息真正引用的 media（content ->> 'mediaId'）。
	if ref.GetMediaId() == "" || ref.GetMediaId() != mediaID {
		return apperror.Forbidden("media object is not referenced by this message")
	}

	switch ref.GetChatType() {
	case chatTypeSingle:
		peer := ref.GetPeerAccountId()
		if peer == "" {
			return apperror.Forbidden("media object is not accessible by requester")
		}
		fr, err := friendCli.GetFriendship(ctx, &friendsclient.GetFriendshipRequest{UserId: requester, FriendId: peer})
		if err != nil {
			return err
		}
		if !fr.GetFriendship().GetIsFriend() {
			return apperror.Forbidden("media object is not accessible by requester")
		}
		return nil
	case chatTypeGroup:
		groupID := ref.GetGroupId()
		if groupID == "" {
			return apperror.Forbidden("media object is not accessible by requester")
		}
		mem, err := groupCli.IsMember(ctx, &groupsclient.IsMemberRequest{GroupId: groupID, UserId: requester})
		if err != nil {
			return err
		}
		if !mem.GetIsMember() {
			return apperror.Forbidden("media object is not accessible by requester")
		}
		return nil
	default:
		return apperror.Forbidden("media object is not accessible by requester")
	}
}
