package gateway

import (
	"github.com/wujunhui99/agents_im/internal/config"
	"github.com/wujunhui99/agents_im/internal/logic"
	"github.com/wujunhui99/agents_im/internal/servicecontext/common"
)

type ServiceContext struct {
	common.AuthRuntime
	MessageLogic *logic.MessageLogic
}

func NewServiceContext(messageLogic *logic.MessageLogic, auth config.JWTAuthConfig) *ServiceContext {
	return &ServiceContext{
		AuthRuntime:  common.NewAuthRuntime(auth),
		MessageLogic: messageLogic,
	}
}
