// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package svc

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"

	"github.com/wujunhui99/agents_im/pkg/apperror"
	"github.com/wujunhui99/agents_im/pkg/ctxuser"
	"github.com/wujunhui99/agents_im/pkg/middleware"
	"github.com/wujunhui99/agents_im/pkg/model"
	"github.com/wujunhui99/agents_im/pkg/rpcerror"
	"github.com/wujunhui99/agents_im/service/admin/api/internal/bootstrap"
	"github.com/wujunhui99/agents_im/service/admin/api/internal/config"
	adminpb "github.com/wujunhui99/agents_im/service/admin/rpc/admin"
	"github.com/wujunhui99/agents_im/service/admin/rpc/adminclient"
	"github.com/wujunhui99/agents_im/service/auth/rpc/authclient"
	"github.com/wujunhui99/agents_im/service/user/rpc/userclient"
	"github.com/zeromicro/go-zero/rest"
	"github.com/zeromicro/go-zero/rest/httpx"
	"github.com/zeromicro/go-zero/zrpc"
)

var (
	ErrAdminRPCConfigRequired = errors.New("admin rpc client config is required")
	ErrUserRPCConfigRequired  = errors.New("user rpc client config is required")
	ErrAuthRPCConfigRequired  = errors.New("auth rpc client config is required")
)

type ServiceContext struct {
	Config   config.Config
	AdminRPC adminclient.Admin
	// UserRPC / AuthRPC：BFF 编排「创建测试账户」（user-rpc 建号 + auth-rpc 设凭据）。
	UserRPC userclient.User
	AuthRPC authclient.Auth
	// DeviceAuth：admin 路由组中间件。先走共享活跃会话校验（common/middleware.DeviceAuth），
	// 再过 admin 账号闸（经 admin-rpc 校验请求者是 admin）。健康检查/metrics 不挂此中间件。
	DeviceAuth rest.Middleware
}

func NewServiceContext(c config.Config) (*ServiceContext, error) {
	if !hasRPCClientConfig(c.AdminRPC) {
		return nil, ErrAdminRPCConfigRequired
	}
	cli, err := zrpc.NewClient(c.AdminRPC)
	if err != nil {
		return nil, err
	}
	adminRPC := adminclient.NewAdmin(cli)
	if !hasRPCClientConfig(c.UserRPC) {
		return nil, ErrUserRPCConfigRequired
	}
	userCli, err := zrpc.NewClient(c.UserRPC)
	if err != nil {
		return nil, err
	}
	userRPC := userclient.NewUser(userCli)
	if !hasRPCClientConfig(c.AuthRPC) {
		return nil, ErrAuthRPCConfigRequired
	}
	authCli, err := zrpc.NewClient(c.AuthRPC)
	if err != nil {
		return nil, err
	}
	authRPC := authclient.NewAuth(authCli)
	if created, err := bootstrap.EnsureAdminAccount(context.Background(), c.AdminBootstrap, userRPC, authRPC); err != nil {
		return nil, fmt.Errorf("bootstrap admin account: %w", err)
	} else if created {
		log.Printf("admin bootstrap account ensured for identifier %q", c.AdminBootstrap.Identifier)
	}
	deviceAuth := middleware.NewDeviceAuthMiddleware(middleware.NewRedisSessionStore(c.Redis)).Handle
	return &ServiceContext{
		Config:     c,
		AdminRPC:   adminRPC,
		UserRPC:    userRPC,
		AuthRPC:    authRPC,
		DeviceAuth: chainMiddlewares(deviceAuth, adminOnlyMiddleware(adminRPC)),
	}, nil
}

func hasRPCClientConfig(conf zrpc.RpcClientConf) bool {
	return conf.Target != "" || len(conf.Endpoints) > 0 || (len(conf.Etcd.Hosts) > 0 && conf.Etcd.Key != "")
}

// chainMiddlewares 按声明顺序串起多个中间件：chainMiddlewares(a, b) 执行顺序为 a → b → handler。
func chainMiddlewares(mws ...rest.Middleware) rest.Middleware {
	return func(final http.HandlerFunc) http.HandlerFunc {
		for i := len(mws) - 1; i >= 0; i-- {
			final = mws[i](final)
		}
		return final
	}
}

// adminOnlyMiddleware 校验请求者是 admin 账号。经 admin-rpc 取账号（admin-api 不碰 DB），
// 非 admin 返回 403。运行在 jwt + DeviceAuth 之后，请求者 user_id 已在 context。
func adminOnlyMiddleware(adminRPC adminclient.Admin) rest.Middleware {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			userID, err := ctxuser.UserID(r.Context())
			if err != nil {
				httpx.ErrorCtx(r.Context(), w, err)
				return
			}
			detail, err := adminRPC.GetUserDetail(r.Context(), &adminpb.UserDetailRequest{AccountId: userID})
			if err != nil {
				httpx.ErrorCtx(r.Context(), w, rpcerror.FromStatus(err))
				return
			}
			if detail.GetUser().GetAccountType() != string(model.AccountTypeAdmin) {
				httpx.ErrorCtx(r.Context(), w, apperror.Forbidden("admin account is required"))
				return
			}
			next(w, r)
		}
	}
}
