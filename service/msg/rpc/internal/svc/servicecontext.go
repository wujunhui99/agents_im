package svc

import (
	"log"

	business "github.com/wujunhui99/agents_im/internal/logic"
	"github.com/wujunhui99/agents_im/internal/mediavalidate"
	"github.com/wujunhui99/agents_im/internal/repository"
	appconfig "github.com/wujunhui99/agents_im/pkg/config"
	"github.com/wujunhui99/agents_im/service/msg/rpc/internal/config"
	"github.com/wujunhui99/agents_im/service/msg/rpc/internal/model"
	"github.com/zeromicro/go-zero/core/stores/postgres"
)

type ServiceContext struct {
	Config config.Config

	// 消息域自有数据层（goctl model，脱 internal/repository）。
	Messages model.MessagesModel
	Threads  model.ConversationThreadsModel
	States   model.UserConversationStatesModel
	Outbox   model.MessageOutboxModel

	// 跨域 keystone 例外：SendMessage 写路径需要 inline 鉴权（群成员解析 + 媒体校验），
	// 无法干净 BFF 化，暂依赖 internal（待 groups/media 完全 BFF/rpc 化后删）。
	Groups business.GroupMemberLister
	Media  business.MessageMediaValidator
}

func NewServiceContext(c config.Config) *ServiceContext {
	conn := postgres.New(c.DataSource)

	groupsRepo, err := repository.NewGroupsRepositoryForStorage(appconfig.StorageDriverPostgres, c.DataSource)
	if err != nil {
		log.Fatalf("build groups repository: %v", err)
	}
	mediaRepo, err := repository.NewMediaRepositoryForStorage(appconfig.StorageDriverPostgres, c.DataSource)
	if err != nil {
		log.Fatalf("build media repository: %v", err)
	}

	return &ServiceContext{
		Config:   c,
		Messages: model.NewMessagesModel(conn),
		Threads:  model.NewConversationThreadsModel(conn),
		States:   model.NewUserConversationStatesModel(conn),
		Outbox:   model.NewMessageOutboxModel(conn),
		Groups:   business.NewGroupsLogic(groupsRepo, nil),
		Media:    mediavalidate.NewMessageValidator(mediaRepo),
	}
}
