// Package groupsrpc 把群成员鉴权落到属主 groups-rpc ListMembers 上（#617，脱 internal GroupsLogic
// 直读 groups 表）：msg-rpc 的读访问控制（PullMessages / GetConversationsSeqState 过滤）与写路径
// 群成员解析（SendMessage）经此适配器调 groups-rpc。单向叶子调用（msg-rpc → groups-rpc），
// 不在 rpc 间成环（groups-rpc 无下游 rpc 依赖）。错误经 rpcerror.FromStatus 还原 apperror。
package groupsrpc

import (
	"context"

	"github.com/wujunhui99/agents_im/pkg/rpcerror"
	groupspb "github.com/wujunhui99/agents_im/service/groups/rpc/groups"
	"github.com/wujunhui99/agents_im/service/groups/rpc/groupsclient"
)

// GroupMemberLister 列群成员做鉴权（本地端口，取代 internal/logic.GroupMemberLister）。
type GroupMemberLister interface {
	ListMembers(ctx context.Context, req ListMembersRequest) (ListMembersResponse, error)
}

type ListMembersRequest struct {
	GroupID         string
	RequesterUserID string
}

type ListMembersResponse struct {
	GroupID string
	Members []GroupMemberInfo
}

type GroupMemberInfo struct {
	GroupID string
	UserID  string
	Role    string
	State   string
}

// Client 把 groups-rpc ListMembers 暴露为 GroupMemberLister。
type Client struct {
	rpc groupsclient.Groups
}

func NewClient(rpc groupsclient.Groups) *Client {
	return &Client{rpc: rpc}
}

func (c *Client) ListMembers(ctx context.Context, req ListMembersRequest) (ListMembersResponse, error) {
	resp, err := c.rpc.ListMembers(ctx, &groupspb.ListMembersRequest{
		GroupId:         req.GroupID,
		RequesterUserId: req.RequesterUserID,
	})
	if err != nil {
		return ListMembersResponse{}, rpcerror.FromStatus(err)
	}
	members := make([]GroupMemberInfo, 0, len(resp.GetMembers()))
	for _, m := range resp.GetMembers() {
		members = append(members, GroupMemberInfo{
			GroupID: m.GetGroupId(),
			UserID:  m.GetUserId(),
			Role:    m.GetRole(),
			State:   m.GetState(),
		})
	}
	return ListMembersResponse{GroupID: resp.GetGroupId(), Members: members}, nil
}
