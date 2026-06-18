package svc

import (
	"context"
	"log"

	business "github.com/wujunhui99/agents_im/internal/logic"
	"github.com/wujunhui99/agents_im/internal/repository"
	"github.com/wujunhui99/agents_im/service/media/rpc/mediaclient"
	"github.com/wujunhui99/agents_im/service/user/rpc/internal/config"
	"github.com/wujunhui99/agents_im/service/user/rpc/internal/model"

	"github.com/zeromicro/go-zero/core/stores/postgres"
	"github.com/zeromicro/go-zero/zrpc"
)

// DefaultAssistantProvisioner 是「新用户开通默认助手」的 keystone 跨域写接口（agent 域）。
// 无 agent-rpc 可 BFF，暂由 internal/logic 实现注入，仍读/写 internal/repository；
// 待 agent/message 迁移后删（见 docs/refactor/v1/progress/02-microservices.md）。
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

	// keystone 跨域例外（仍依赖 internal），见各接口注释。
	Assistant       DefaultAssistantProvisioner
	AvatarValidator AvatarValidator
}

func NewServiceContext(c config.Config) *ServiceContext {
	conn := postgres.New(c.DataSource)

	// keystone：默认助手开通（agent 域写）。沿用 internal god-repository（account/agent/registry 同一实现）。
	repo, err := repository.NewPostgresRepository(c.DataSource)
	if err != nil {
		log.Fatalf("build account repository: %v", err)
	}
	provisioner := business.NewDefaultAssistantProvisioner(repo, repo, repo)
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
		Accounts:        model.NewAccountsModel(conn),
		Profiles:        model.NewProfilesModel(conn),
		Assistant:       provisioner,
		AvatarValidator: newMediaRPCAvatarValidator(mediaclient.NewMedia(mediaCli)),
	}
}
