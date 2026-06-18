package svc

import (
	"log"
	"time"

	"github.com/wujunhui99/agents_im/pkg/middleware"
	"github.com/wujunhui99/agents_im/pkg/auth/token"
	"github.com/wujunhui99/agents_im/service/auth/rpc/internal/config"
	"github.com/wujunhui99/agents_im/service/auth/rpc/internal/model"
	"github.com/wujunhui99/agents_im/service/third/rpc/mailclient"
	"github.com/wujunhui99/agents_im/service/user/rpc/userclient"

	"github.com/zeromicro/go-zero/core/stores/postgres"
	"github.com/zeromicro/go-zero/zrpc"
)

type ServiceContext struct {
	Config config.Config

	// Tokens/Sessions：签发/校验 JWT 与活跃会话（per-device 单会话，Redis 存储）。
	Tokens   token.Manager
	Sessions middleware.SessionStore

	// 下游微服务 client（直调，不再经 internal/auth adapter）：
	// 注册/登录读用户资料经属主 user-rpc；注册验证码经 mail-rpc。
	Users  userclient.User
	Mailer mailclient.Mail

	// auth 域自有 goctl 数据层。
	Credentials        model.AuthCredentialsModel
	EmailVerifications model.AuthEmailVerificationTokensModel
	// AccountsGuard 是跨域鉴权读 keystone 例外（详见 model/accounts_guard.go 注释）。
	AccountsGuard model.AccountsGuardModel
}

func NewServiceContext(c config.Config) *ServiceContext {
	conn := postgres.New(c.DataSource)

	// user-rpc / mail-rpc 直调 client（go-zero 原生 Telemetry，不挂 observability 拦截器）。
	if !hasRPCClientConfig(c.UserRPC) {
		log.Fatalf("user rpc client config is required")
	}
	userCli, err := zrpc.NewClient(c.UserRPC)
	if err != nil {
		log.Fatalf("build user rpc client: %v", err)
	}
	if !hasRPCClientConfig(c.MailRPC) {
		log.Fatalf("mail rpc client config is required")
	}
	mailCli, err := zrpc.NewClient(c.MailRPC)
	if err != nil {
		log.Fatalf("build mail rpc client: %v", err)
	}

	return &ServiceContext{
		Config:             c,
		Tokens:             token.NewHMACTokenManager(c.TokenAuth.AccessSecret, time.Duration(c.TokenAuth.AccessExpire)*time.Second),
		Sessions:           middleware.NewRedisSessionStore(c.SessionRedis),
		Users:              userclient.NewUser(userCli),
		Mailer:             mailclient.NewMail(mailCli),
		Credentials:        model.NewAuthCredentialsModel(conn),
		EmailVerifications: model.NewAuthEmailVerificationTokensModel(conn),
		AccountsGuard:      model.NewAccountsGuardModel(conn),
	}
}

// hasRPCClientConfig 判断 zrpc 客户端配置是否提供了 Target/Endpoints/Etcd 之一。
func hasRPCClientConfig(conf zrpc.RpcClientConf) bool {
	return conf.Target != "" || len(conf.Endpoints) > 0 || (len(conf.Etcd.Hosts) > 0 && conf.Etcd.Key != "")
}
