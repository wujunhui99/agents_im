package svc

import (
	"log"
	"time"

	"github.com/wujunhui99/agents_im/common/middleware"
	"github.com/wujunhui99/agents_im/common/share/auth/token"
	business "github.com/wujunhui99/agents_im/internal/auth/logic"
	"github.com/wujunhui99/agents_im/internal/auth/mailadapter"
	authrepo "github.com/wujunhui99/agents_im/internal/auth/repository"
	"github.com/wujunhui99/agents_im/service/auth/rpc/internal/config"
	"github.com/wujunhui99/agents_im/service/auth/rpc/internal/model"
	authuserrpc "github.com/wujunhui99/agents_im/service/auth/rpc/internal/userrpc"
	"github.com/wujunhui99/agents_im/service/user/rpc/userclient"

	"github.com/zeromicro/go-zero/core/stores/postgres"
	"github.com/zeromicro/go-zero/zrpc"
)

type ServiceContext struct {
	Config    config.Config
	AuthLogic *business.AuthLogic
	AuthRepo  authrepo.CredentialRepository

	// goctl 数据层（auth 域自有，不走 internal/）：EnsureTestCredential 用。
	// auth 域整体重构（退役 internal/auth）后，上面的 monolith 依赖将全部并入这里。
	Credentials model.AuthCredentialsModel
	// AccountsGuard 是跨域鉴权读 keystone 例外（详见 model/accounts_guard.go 注释）。
	AccountsGuard model.AccountsGuardModel
}

func NewServiceContext(c config.Config) *ServiceContext {
	authRepo, err := authrepo.NewRepositoryForStorage(c.StorageDriver, c.DataSource)
	if err != nil {
		log.Fatalf("build auth repository: %v", err)
	}
	mailer, err := mailadapter.NewRequiredRPCClient(c.MailRPC)
	if err != nil {
		log.Fatalf("build mail rpc client: %v", err)
	}

	// 用户资料读经属主 user-rpc（#551）：不再构造 internal/logic.UserLogic / internal/repository。
	// 默认助手开通（startup backfill + 每个新用户）已由 user-rpc 负责，auth 侧不再重复。
	userCli, err := zrpc.NewClient(c.UserRPC)
	if err != nil {
		log.Fatalf("build user rpc client: %v", err)
	}
	users := authuserrpc.NewClient(userclient.NewUser(userCli))

	conn := postgres.New(c.DataSource)

	return &ServiceContext{
		Config: c,
		AuthLogic: business.NewAuthLogicWithOptions(authRepo, users, business.NewPasswordHasher(), token.NewHMACTokenManager(c.TokenAuth.AccessSecret, time.Duration(c.TokenAuth.AccessExpire)*time.Second), business.AuthOptions{
			VerificationRepo: authRepo,
			Sessions:         middleware.NewRedisSessionStore(c.SessionRedis),
			Mailer:           mailer,
		}),
		AuthRepo:      authRepo,
		Credentials:   model.NewAuthCredentialsModel(conn),
		AccountsGuard: model.NewAccountsGuardModel(conn),
	}
}
