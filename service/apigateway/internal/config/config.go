// Package config 是 apigateway 的聚合配置：一个 HTTP 进程承载原 8 个 *-api 的域路由，
// 每个域保留自己的 Config（RPC 客户端 / Redis / 域开关），按域嵌套在 yaml 中。
package config

import (
	"github.com/zeromicro/go-zero/rest"

	adminconfig "github.com/wujunhui99/agents_im/service/apigateway/internal/admin/config"
	agentconfig "github.com/wujunhui99/agents_im/service/apigateway/internal/agent/config"
	authconfig "github.com/wujunhui99/agents_im/service/apigateway/internal/auth/config"
	friendsconfig "github.com/wujunhui99/agents_im/service/apigateway/internal/friends/config"
	groupsconfig "github.com/wujunhui99/agents_im/service/apigateway/internal/groups/config"
	mediaconfig "github.com/wujunhui99/agents_im/service/apigateway/internal/media/config"
	msgconfig "github.com/wujunhui99/agents_im/service/apigateway/internal/msg/config"
	userconfig "github.com/wujunhui99/agents_im/service/apigateway/internal/user/config"
)

type Config struct {
	rest.RestConf

	User    userconfig.Config
	AuthAPI authconfig.Config
	Friends friendsconfig.Config
	Groups  groupsconfig.Config
	Msg     msgconfig.Config
	Agent   agentconfig.Config
	Admin   adminconfig.Config
	Media   mediaconfig.Config
}
