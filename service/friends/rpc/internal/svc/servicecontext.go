package svc

import (
	"github.com/wujunhui99/agents_im/service/friends/rpc/internal/config"
	"github.com/wujunhui99/agents_im/service/friends/rpc/internal/model"
	"github.com/zeromicro/go-zero/core/stores/postgres"
)

type ServiceContext struct {
	Config          config.Config
	FriendshipModel model.FriendshipsModel
}

func NewServiceContext(c config.Config) *ServiceContext {
	conn := postgres.New(c.DataSource)
	return &ServiceContext{
		Config:          c,
		FriendshipModel: model.NewFriendshipsModel(conn),
	}
}
