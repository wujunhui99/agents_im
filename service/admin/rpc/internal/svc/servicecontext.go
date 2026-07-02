package svc

import (
	"log"

	"github.com/wujunhui99/agents_im/service/admin/rpc/internal/config"
	"github.com/wujunhui99/agents_im/service/admin/rpc/internal/feedbackstore"
	"github.com/wujunhui99/agents_im/service/admin/rpc/internal/model"
	"github.com/wujunhui99/agents_im/service/agent/rpc/agentclient"
	"github.com/wujunhui99/agents_im/service/friends/rpc/friendsclient"
	"github.com/wujunhui99/agents_im/service/msg/rpc/msgclient"
	"github.com/wujunhui99/agents_im/service/user/rpc/userclient"
	"github.com/zeromicro/go-zero/core/stores/postgres"
	"github.com/zeromicro/go-zero/zrpc"
)

// ServiceContext 持有 admin-rpc 的依赖。
//
// admin-rpc 是 admin 域的只读聚合器，本身只碰两张自有表，其余跨域事实全部经属主 rpc（#618 起
// 已彻底脱顶层 internal/repository + MessageCreatedHook）：
//   - TaskReportModel 是 admin 独占表 task_reports 的 goctl 自有数据层；Feedback 是 admin
//     自有数据层（feedbackstore goctl model 背靠 feedback 表，#678）。
//   - UserRPC 跨域账号只读直调属主 user-rpc（gate #550）；AgentRPC agent 审计只读经属主
//     agent-rpc gRPC（#616）。
//   - MsgRPC 会话消息只读 + AI 重放经属主 msg-rpc gRPC（#618，取代 AdminMessageRepository
//     直读与休眠的 MessageCreatedHook；重放由 msg-rpc 侧重发 agent.trigger.v1）。
//   - FriendsRPC 好友关系只读经属主 friends-rpc gRPC（#618，取代 FriendshipRepository）。
type ServiceContext struct {
	Config config.Config

	TaskReportModel model.TaskReportsModel

	UserRPC    userclient.User
	AgentRPC   agentclient.Agent
	MsgRPC     msgclient.Msg
	FriendsRPC friendsclient.Friends
	Feedback   feedbackstore.Store
}

func NewServiceContext(c config.Config) *ServiceContext {
	conn := postgres.New(c.DataSource)

	return &ServiceContext{
		Config:          c,
		TaskReportModel: model.NewTaskReportsModel(conn),
		UserRPC:         userclient.NewUser(mustRPCClient(c.UserRPC, "UserRPC")),
		AgentRPC:        agentclient.NewAgent(mustRPCClient(c.AgentRPC, "AgentRPC")),
		MsgRPC:          msgclient.NewMsg(mustRPCClient(c.MsgRPC, "MsgRPC")),
		FriendsRPC:      friendsclient.NewFriends(mustRPCClient(c.FriendsRPC, "FriendsRPC")),
		Feedback:        feedbackstore.NewModelStore(c.DataSource),
	}
}

// mustRPCClient 构造一个属主 rpc 客户端；缺配置或建连失败即启动失败（失败优先，不静默降级）。
func mustRPCClient(conf zrpc.RpcClientConf, name string) zrpc.Client {
	if !hasRPCClientConfig(conf) {
		log.Fatalf("admin-rpc requires rpc client config (%s)", name)
	}
	client, err := zrpc.NewClient(conf)
	if err != nil {
		log.Fatalf("build %s rpc client: %v", name, err)
	}
	return client
}

// hasRPCClientConfig 判断 zrpc 客户端是否已配置(target / endpoints / etcd 任一)。
func hasRPCClientConfig(conf zrpc.RpcClientConf) bool {
	return conf.Target != "" || len(conf.Endpoints) > 0 || (len(conf.Etcd.Hosts) > 0 && conf.Etcd.Key != "")
}
