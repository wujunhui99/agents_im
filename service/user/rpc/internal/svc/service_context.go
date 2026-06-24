package svc

import (
	"context"
	"log"

	"github.com/wujunhui99/agents_im/service/agent/rpc/agentclient"
	"github.com/wujunhui99/agents_im/service/friends/rpc/friendsclient"
	"github.com/wujunhui99/agents_im/service/media/rpc/mediaclient"
	"github.com/wujunhui99/agents_im/service/user/rpc/internal/config"
	"github.com/wujunhui99/agents_im/service/user/rpc/internal/model"

	"github.com/zeromicro/go-zero/core/stores/postgres"
	"github.com/zeromicro/go-zero/zrpc"
)

// DefaultAssistantProvisioner 是「新用户开通默认助手」的跨域编排接口（#606：account=user 域本地、
// agent 域经 agent-rpc、好友经 friends-rpc）。由 user-rpc 装配，详见 default_assistant.go。
type DefaultAssistantProvisioner interface {
	EnsureForUser(ctx context.Context, accountID string) error
}

// AvatarValidator 校验头像 media 的存在/归属/类型（media 域）。#533 起经 media-rpc，
// 不再直读 media_objects（脱 internal/mediavalidate）。
type AvatarValidator interface {
	ValidateAvatarMedia(ctx context.Context, ownerUserID string, mediaID string) error
}

type ServiceContext struct {
	Config   config.Config
	Accounts model.AccountsModel
	Profiles model.ProfilesModel

	Assistant       DefaultAssistantProvisioner
	AvatarValidator AvatarValidator
}

func NewServiceContext(c config.Config) *ServiceContext {
	conn := postgres.New(c.DataSource)
	accountsModel := model.NewAccountsModel(conn)
	profilesModel := model.NewProfilesModel(conn)

	if !hasRPCClientConfig(c.AgentRPC) {
		log.Fatalf("user-rpc requires agent rpc client config (AgentRPC) for default assistant provisioning")
	}
	if !hasRPCClientConfig(c.FriendsRPC) {
		log.Fatalf("user-rpc requires friends rpc client config (FriendsRPC) for default assistant friendship")
	}
	agentRPCClient, err := zrpc.NewClient(c.AgentRPC)
	if err != nil {
		log.Fatalf("build agent rpc client: %v", err)
	}
	friendsRPCClient, err := zrpc.NewClient(c.FriendsRPC)
	if err != nil {
		log.Fatalf("build friends rpc client: %v", err)
	}

	// 默认助手账号读写经 user-rpc 自有 goctl model（bigint-safe，gate #550）；agent 域装配 + 好友建立
	// 经 agent-rpc / friends-rpc（#606，脱 internal/repository agent registry 写与 EnsureAcceptedFriendship）。
	accountRepo := newAssistantAccountRepo(accountsModel, profilesModel, nil)
	provisioner := newDefaultAssistantProvisioner(accountRepo, agentclient.NewAgent(agentRPCClient), friendsclient.NewFriends(friendsRPCClient))
	if _, err := provisioner.Backfill(context.Background()); err != nil {
		log.Fatalf("backfill default assistant: %v", err)
	}

	// 头像 media 校验经 media-rpc（#533，脱 internal/mediavalidate 直读 media_objects）。
	mediaCli, err := zrpc.NewClient(c.MediaRPC)
	if err != nil {
		log.Fatalf("build media rpc client: %v", err)
	}

	return &ServiceContext{
		Config:          c,
		Accounts:        accountsModel,
		Profiles:        profilesModel,
		Assistant:       provisioner,
		AvatarValidator: newMediaRPCAvatarValidator(mediaclient.NewMedia(mediaCli)),
	}
}

// hasRPCClientConfig 判断 zrpc 客户端是否已配置（target / endpoints / etcd 任一）。
func hasRPCClientConfig(conf zrpc.RpcClientConf) bool {
	return conf.Target != "" || len(conf.Endpoints) > 0 || (len(conf.Etcd.Hosts) > 0 && conf.Etcd.Key != "")
}
