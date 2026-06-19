package svc

import (
	"errors"

	"github.com/wujunhui99/agents_im/service/friends/rpc/friendsclient"
	"github.com/wujunhui99/agents_im/service/groups/rpc/groupsclient"
	"github.com/wujunhui99/agents_im/service/media/api/internal/config"
	"github.com/wujunhui99/agents_im/service/media/rpc/mediaclient"
	"github.com/wujunhui99/agents_im/service/msg/rpc/msgclient"
	"github.com/zeromicro/go-zero/zrpc"
)

var (
	ErrMediaRPCConfigRequired   = errors.New("media rpc client config is required")
	ErrMsgRPCConfigRequired     = errors.New("msg rpc client config is required")
	ErrFriendsRPCConfigRequired = errors.New("friends rpc client config is required")
	ErrGroupsRPCConfigRequired  = errors.New("groups rpc client config is required")
)

// ServiceContext 持有 media-api(BFF) 的下游 rpc 客户端。下载授权（EPIC #527 §4，#532）在这一层
// 聚合：MsgRPC/FriendsRPC/GroupsRPC 做链路校验 + 私聊单向好友 / 群成员判定，再调 MediaRPC 纯签发。
type ServiceContext struct {
	Config     config.Config
	MediaRPC   mediaclient.Media
	MsgRPC     msgclient.Msg
	FriendsRPC friendsclient.Friends
	GroupsRPC  groupsclient.Groups
}

func NewServiceContext(c config.Config) (*ServiceContext, error) {
	if !hasRPCClientConfig(c.MediaRPC) {
		return nil, ErrMediaRPCConfigRequired
	}
	mediaCli, err := zrpc.NewClient(c.MediaRPC)
	if err != nil {
		return nil, err
	}
	if !hasRPCClientConfig(c.MsgRPC) {
		return nil, ErrMsgRPCConfigRequired
	}
	msgCli, err := zrpc.NewClient(c.MsgRPC)
	if err != nil {
		return nil, err
	}
	if !hasRPCClientConfig(c.FriendsRPC) {
		return nil, ErrFriendsRPCConfigRequired
	}
	friendsCli, err := zrpc.NewClient(c.FriendsRPC)
	if err != nil {
		return nil, err
	}
	if !hasRPCClientConfig(c.GroupsRPC) {
		return nil, ErrGroupsRPCConfigRequired
	}
	groupsCli, err := zrpc.NewClient(c.GroupsRPC)
	if err != nil {
		return nil, err
	}
	return &ServiceContext{
		Config:     c,
		MediaRPC:   mediaclient.NewMedia(mediaCli),
		MsgRPC:     msgclient.NewMsg(msgCli),
		FriendsRPC: friendsclient.NewFriends(friendsCli),
		GroupsRPC:  groupsclient.NewGroups(groupsCli),
	}, nil
}

func hasRPCClientConfig(conf zrpc.RpcClientConf) bool {
	return conf.Target != "" || len(conf.Endpoints) > 0 || (len(conf.Etcd.Hosts) > 0 && conf.Etcd.Key != "")
}
