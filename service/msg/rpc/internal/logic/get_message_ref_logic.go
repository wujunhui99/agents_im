package logic

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"

	"github.com/wujunhui99/agents_im/pkg/apperror"
	"github.com/wujunhui99/agents_im/pkg/rpcerror"
	"github.com/wujunhui99/agents_im/service/msg/rpc/internal/model"
	"github.com/wujunhui99/agents_im/service/msg/rpc/internal/svc"
	"github.com/wujunhui99/agents_im/service/msg/rpc/msg"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetMessageRefLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetMessageRefLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetMessageRefLogic {
	return &GetMessageRefLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

// GetMessageRef 取一条消息的下载授权引用（EPIC #527 §4）：chat_type / group_id / 私聊对端 /
// 引用的 media_id（content ->> 'mediaId'）。media 编排 GetDownloadURL 用它做链路校验
// （入参 media 必须等于 msg 真正引用的 media）+ 私聊好友 / 群成员判定。
func (l *GetMessageRefLogic) GetMessageRef(in *msg.GetMessageRefRequest) (*msg.GetMessageRefResponse, error) {
	messageID, err := strconv.ParseInt(strings.TrimSpace(in.GetServerMsgId()), 10, 64)
	if err != nil || messageID <= 0 {
		return nil, rpcerror.ToStatus(apperror.InvalidArgument("server_msg_id must be a positive integer"))
	}

	m, err := l.svcCtx.Messages.FindOne(l.ctx, messageID)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return nil, rpcerror.ToStatus(apperror.NotFound("message not found"))
		}
		return nil, rpcerror.ToStatus(err)
	}

	chatType := model.ConversationTypeString(m.ConversationType)
	resp := &msg.GetMessageRefResponse{
		ChatType: chatType,
		MediaId:  mediaIDFromContent(m.Content),
	}
	switch m.ConversationType {
	case model.ConversationTypeGroup:
		resp.GroupId = m.GroupId
	case model.ConversationTypeSingle:
		resp.PeerAccountId = singleChatPeer(m, strings.TrimSpace(in.GetRequesterAccountId()))
	}
	return resp, nil
}

// singleChatPeer 求私聊对端：peer = sender/receiver 中非 requester 的一方。requester 为空或非参与方时
// 回退到消息 receiver（friends 校验仍以 requester 视角执行，由 media 编排层 §4 收口）。
func singleChatPeer(m *model.Messages, requester string) string {
	if requester != "" && requester == m.ReceiverAccountId {
		return m.SenderAccountId
	}
	return m.ReceiverAccountId
}

// mediaIDFromContent 从消息存库 jsonb content 取 mediaId（image / file 两类消息都带此字段）；
// text 等无附件消息返回空串。
func mediaIDFromContent(content string) string {
	var body struct {
		MediaID string `json:"mediaId"`
	}
	if err := json.Unmarshal([]byte(content), &body); err != nil {
		return ""
	}
	return strings.TrimSpace(body.MediaID)
}
