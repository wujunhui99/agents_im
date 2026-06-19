package svc

import (
	"context"
	"log"

	appconfig "github.com/wujunhui99/agents_im/pkg/config"
	"github.com/wujunhui99/agents_im/pkg/idgen"
	"github.com/wujunhui99/agents_im/pkg/objectstorage"
	"github.com/wujunhui99/agents_im/pkg/rpcerror"
	"github.com/wujunhui99/agents_im/service/friends/rpc/friendsclient"
	"github.com/wujunhui99/agents_im/service/groups/rpc/groupsclient"
	"github.com/wujunhui99/agents_im/service/media/rpc/internal/config"
	"github.com/wujunhui99/agents_im/service/media/rpc/internal/model"
	"github.com/wujunhui99/agents_im/service/msg/rpc/msgclient"
	"github.com/zeromicro/go-zero/core/stores/postgres"
	"github.com/zeromicro/go-zero/zrpc"
)

// media_id 雪花中段（12 位）布局：media 无单/群语义，route hint 宽度为 0（保留，恒传 0，
// 见 routedflake.go 与 ADR #529）；低 10 位机器号（默认 1024 实例），高 2 位为保留间隙。
// 扩副本调 Snowflake.MachineBits（机器号靠右收缩、不挪位）。
const (
	mediaHintBits           = 0
	defaultMediaMachineBits = 10
)

// MessageRefResolver 取一条消息的下载授权引用（chat_type/group_id/私聊对端/引用的 media_id）。
// 由 msg-rpc 满足（GetMessageRef，EPIC #527 §4 链路校验 + 关系判定的入口）。
type MessageRefResolver interface {
	GetMessageRef(ctx context.Context, serverMsgID, requesterAccountID string) (chatType, groupID, peerAccountID, mediaID string, err error)
}

// FriendshipChecker 判定 requester 视角下与 peer 的单向好友关系是否仍正常（friends 双记录，
// 只看 requester→peer 这条；requester 没删对方即放行，对方删 requester 不影响）。由 friends-rpc
// 的 GetFriendship 满足（EPIC #527 §4 私聊校验）。
type FriendshipChecker interface {
	IsFriendOneWay(ctx context.Context, requesterAccountID, peerAccountID string) (bool, error)
}

// GroupMembershipChecker 判定 requester 当前是否为某群 active 成员。由 groups-rpc 的 IsMember
// 满足（EPIC #527 §4 群聊校验）。
type GroupMembershipChecker interface {
	IsMember(ctx context.Context, groupID, requesterAccountID string) (bool, error)
}

// ServiceContext 持有 media-rpc 的数据层、对象存储与下载授权编排的跨域 rpc 客户端。
// media_objects 写入/读取走 goctl MediaModel（脱 internal/repository）；下载授权（EPIC #527 §4）
// 经属主 msg/friends/groups rpc 编排，不再反向依赖 internal/repository。
type ServiceContext struct {
	Config     config.Config
	MediaModel model.MediaObjectsModel
	MediaIDGen *idgen.RoutedFlake
	Store      objectstorage.ObjectStore
	Bucket     string
	MessageRef MessageRefResolver
	Friends    FriendshipChecker
	Groups     GroupMembershipChecker
}

func NewServiceContext(c config.Config) *ServiceContext {
	mediaModel := model.NewMediaObjectsModel(postgres.New(c.DataSource))

	mediaIDGen, err := newMediaIDGenerator(c.Snowflake)
	if err != nil {
		log.Fatalf("build media id generator: %v", err)
	}

	osCfg, err := appconfig.ResolveObjectStorageConfig(c.ObjectStorage, "postgres")
	if err != nil {
		log.Fatalf("resolve object storage config: %v", err)
	}
	objectStore, err := objectstorage.NewStore(osCfg)
	if err != nil {
		log.Fatalf("build object storage: %v", err)
	}

	// 下载授权编排（EPIC #527 §4）：msg/friends/groups 是必需依赖，缺配置显式失败（失败优先）。
	if !hasRPCClientConfig(c.MsgRPC) {
		log.Fatalf("media-rpc requires msg rpc client config (MsgRPC) for download authorization")
	}
	msgRPCClient, err := zrpc.NewClient(c.MsgRPC)
	if err != nil {
		log.Fatalf("build msg rpc client: %v", err)
	}
	if !hasRPCClientConfig(c.FriendsRPC) {
		log.Fatalf("media-rpc requires friends rpc client config (FriendsRPC) for download authorization")
	}
	friendsRPCClient, err := zrpc.NewClient(c.FriendsRPC)
	if err != nil {
		log.Fatalf("build friends rpc client: %v", err)
	}
	if !hasRPCClientConfig(c.GroupsRPC) {
		log.Fatalf("media-rpc requires groups rpc client config (GroupsRPC) for download authorization")
	}
	groupsRPCClient, err := zrpc.NewClient(c.GroupsRPC)
	if err != nil {
		log.Fatalf("build groups rpc client: %v", err)
	}

	return &ServiceContext{
		Config:     c,
		MediaModel: mediaModel,
		MediaIDGen: mediaIDGen,
		Store:      objectStore,
		Bucket:     osCfg.Bucket,
		MessageRef: newMsgRPCMessageRefResolver(msgclient.NewMsg(msgRPCClient)),
		Friends:    newFriendsRPCFriendshipChecker(friendsclient.NewFriends(friendsRPCClient)),
		Groups:     newGroupsRPCMembershipChecker(groupsclient.NewGroups(groupsRPCClient)),
	}
}

// newMediaIDGenerator 构造 media_id 的 RoutedFlake。机器号优先用 idgen.ResolveMachineID()
// （env AGENTS_IM_SNOWFLAKE_MACHINE_ID 或 StatefulSet pod ordinal）；解析不到时回退到
// 配置值（默认 0，适用单副本）。多副本部署须经 env/ordinal 注入唯一机器号，否则同毫秒碰撞。
func newMediaIDGenerator(cfg config.SnowflakeConfig) (*idgen.RoutedFlake, error) {
	machineBits := cfg.MachineBits
	if machineBits == 0 {
		machineBits = defaultMediaMachineBits
	}
	machineID := cfg.MachineID
	if resolved, err := idgen.ResolveMachineID(); err == nil {
		machineID = resolved
	}
	return idgen.NewRoutedFlake(idgen.RoutedFlakeConfig{
		HintBits:    mediaHintBits,
		MachineBits: machineBits,
		MachineID:   machineID,
	})
}

// hasRPCClientConfig 判断 zrpc 客户端是否已配置(target / endpoints / etcd 任一)。
func hasRPCClientConfig(conf zrpc.RpcClientConf) bool {
	return conf.Target != "" || len(conf.Endpoints) > 0 || (len(conf.Etcd.Hosts) > 0 && conf.Etcd.Key != "")
}

// --- 跨域 rpc 客户端 → 下载授权编排接口的适配器 ---

type msgRPCMessageRefResolver struct {
	cli msgclient.Msg
}

func newMsgRPCMessageRefResolver(cli msgclient.Msg) MessageRefResolver {
	return msgRPCMessageRefResolver{cli: cli}
}

func (r msgRPCMessageRefResolver) GetMessageRef(ctx context.Context, serverMsgID, requesterAccountID string) (string, string, string, string, error) {
	resp, err := r.cli.GetMessageRef(ctx, &msgclient.GetMessageRefRequest{
		ServerMsgId:        serverMsgID,
		RequesterAccountId: requesterAccountID,
	})
	if err != nil {
		return "", "", "", "", rpcerror.FromStatus(err)
	}
	return resp.GetChatType(), resp.GetGroupId(), resp.GetPeerAccountId(), resp.GetMediaId(), nil
}

type friendsRPCFriendshipChecker struct {
	cli friendsclient.Friends
}

func newFriendsRPCFriendshipChecker(cli friendsclient.Friends) FriendshipChecker {
	return friendsRPCFriendshipChecker{cli: cli}
}

// IsFriendOneWay 只看 requester→peer 这条记录是否 accepted（GetFriendship 按 user→friend 有向查）。
func (c friendsRPCFriendshipChecker) IsFriendOneWay(ctx context.Context, requesterAccountID, peerAccountID string) (bool, error) {
	resp, err := c.cli.GetFriendship(ctx, &friendsclient.GetFriendshipRequest{
		UserId:   requesterAccountID,
		FriendId: peerAccountID,
	})
	if err != nil {
		return false, rpcerror.FromStatus(err)
	}
	return resp.GetFriendship().GetIsFriend(), nil
}

type groupsRPCMembershipChecker struct {
	cli groupsclient.Groups
}

func newGroupsRPCMembershipChecker(cli groupsclient.Groups) GroupMembershipChecker {
	return groupsRPCMembershipChecker{cli: cli}
}

func (c groupsRPCMembershipChecker) IsMember(ctx context.Context, groupID, requesterAccountID string) (bool, error) {
	resp, err := c.cli.IsMember(ctx, &groupsclient.IsMemberRequest{
		GroupId: groupID,
		UserId:  requesterAccountID,
	})
	if err != nil {
		return false, rpcerror.FromStatus(err)
	}
	return resp.GetIsMember(), nil
}
