// apigateway 是 agents_im 唯一的 HTTP 入口：一个进程承载原 user/auth/friends/groups/
// msg/agent/admin/media 八个 API 域的 handler，对外只暴露一个端口。JWT 校验、jti 会话
// 校验、限流与身份注入都在这一层完成；下游只有内部 gRPC 服务（*-rpc）。
//
// 与 msggateway 的分工：msggateway 负责 WebSocket 长连接与下行广播，独立部署；两者共享
// 同一套 JWT/会话语义（同一 secret + 同一 Redis session key）。
package main

import (
	"flag"
	"log"
	"net/http"

	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/rest"
	"github.com/zeromicro/go-zero/rest/httpx"

	"github.com/wujunhui99/agents_im/pkg/health"
	"github.com/wujunhui99/agents_im/pkg/observability"
	"github.com/wujunhui99/agents_im/pkg/response"
	"github.com/wujunhui99/agents_im/service/apigateway/internal/config"

	adminhandler "github.com/wujunhui99/agents_im/service/apigateway/internal/admin/handler"
	adminsvc "github.com/wujunhui99/agents_im/service/apigateway/internal/admin/svc"
	agenthandler "github.com/wujunhui99/agents_im/service/apigateway/internal/agent/handler"
	agentsvc "github.com/wujunhui99/agents_im/service/apigateway/internal/agent/svc"
	authhandler "github.com/wujunhui99/agents_im/service/apigateway/internal/auth/handler"
	authsvc "github.com/wujunhui99/agents_im/service/apigateway/internal/auth/svc"
	friendshandler "github.com/wujunhui99/agents_im/service/apigateway/internal/friends/handler"
	friendssvc "github.com/wujunhui99/agents_im/service/apigateway/internal/friends/svc"
	groupshandler "github.com/wujunhui99/agents_im/service/apigateway/internal/groups/handler"
	groupssvc "github.com/wujunhui99/agents_im/service/apigateway/internal/groups/svc"
	mediahandler "github.com/wujunhui99/agents_im/service/apigateway/internal/media/handler"
	mediasvc "github.com/wujunhui99/agents_im/service/apigateway/internal/media/svc"
	msghandler "github.com/wujunhui99/agents_im/service/apigateway/internal/msg/handler"
	msgsvc "github.com/wujunhui99/agents_im/service/apigateway/internal/msg/svc"
	userhandler "github.com/wujunhui99/agents_im/service/apigateway/internal/user/handler"
	usersvc "github.com/wujunhui99/agents_im/service/apigateway/internal/user/svc"
)

func main() {
	configFile := flag.String("f", "etc/apigateway.yaml", "the config file")
	flag.Parse()
	run(*configFile)
}

// run starts apigateway: it loads the aggregated config, builds every domain context and
// mounts all domain routes on one HTTP server.
func run(configFile string) {
	var c config.Config
	conf.MustLoad(configFile, &c, conf.UseEnv())

	userCtx, err := usersvc.NewServiceContext(c.User)
	if err != nil {
		log.Fatalf("build user api context: %v", err)
	}
	authCtx, err := authsvc.NewServiceContext(c.AuthAPI)
	if err != nil {
		log.Fatalf("build auth api context: %v", err)
	}
	friendsCtx, err := friendssvc.NewServiceContext(c.Friends)
	if err != nil {
		log.Fatalf("build friends api context: %v", err)
	}
	groupsCtx, err := groupssvc.NewServiceContext(c.Groups)
	if err != nil {
		log.Fatalf("build groups api context: %v", err)
	}
	msgCtx, err := msgsvc.NewServiceContext(c.Msg)
	if err != nil {
		log.Fatalf("build msg api context: %v", err)
	}
	agentCtx, err := agentsvc.NewServiceContextFromConfig(c.Agent)
	if err != nil {
		log.Fatalf("build agent api context: %v", err)
	}
	adminCtx, err := adminsvc.NewServiceContext(c.Admin)
	if err != nil {
		log.Fatalf("build admin api context: %v", err)
	}
	mediaCtx, err := mediasvc.NewServiceContext(c.Media)
	if err != nil {
		log.Fatalf("build media api context: %v", err)
	}

	httpx.SetErrorHandlerCtx(response.GoZeroErrorHandlerCtx)
	server := rest.MustNewServer(c.RestConf, rest.WithUnauthorizedCallback(response.GoZeroUnauthorizedCallback))
	defer server.Stop()

	registerObservabilityHandlers(server)
	userhandler.RegisterHandlers(server, userCtx)
	authhandler.RegisterHandlers(server, authCtx)
	friendshandler.RegisterHandlers(server, friendsCtx)
	groupshandler.RegisterHandlers(server, groupsCtx)
	msghandler.RegisterHandlers(server, msgCtx)
	agenthandler.RegisterHandlers(server, agentCtx)
	adminhandler.RegisterHandlers(server, adminCtx)
	mediahandler.RegisterHandlers(server, mediaCtx)

	log.Printf("%s listening on %s:%d", c.Name, c.Host, c.Port)
	server.Start()
}

func registerObservabilityHandlers(server *rest.Server) {
	server.AddRoutes([]rest.Route{
		{Method: http.MethodGet, Path: "/healthz", Handler: healthHandler()},
		{Method: http.MethodGet, Path: "/readyz", Handler: health.ReadinessHandler("apigateway", func(*http.Request) []health.Check {
			return []health.Check{health.ComponentCheck("domains", true, "wired")}
		})},
		{Method: http.MethodGet, Path: "/metrics", Handler: observability.MetricsHandler()},
	})
}

func healthHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		response.WriteOK(w, map[string]string{"service": "apigateway", "status": "ok"})
	}
}
