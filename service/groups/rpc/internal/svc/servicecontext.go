package svc

import (
	"github.com/wujunhui99/agents_im/service/groups/rpc/internal/config"
	"github.com/wujunhui99/agents_im/service/groups/rpc/internal/model"
	"github.com/zeromicro/go-zero/core/stores/postgres"
)

type ServiceContext struct {
	Config            config.Config
	GroupsModel       model.GroupsModel
	GroupMembersModel model.GroupMembersModel
}

func NewServiceContext(c config.Config) *ServiceContext {
	conn := postgres.New(c.DataSource)
	return &ServiceContext{
		Config:            c,
		GroupsModel:       model.NewGroupsModel(conn),
		GroupMembersModel: model.NewGroupMembersModel(conn),
	}
}
