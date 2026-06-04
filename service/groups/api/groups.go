package main

import (
	"flag"
	"log"
	"net/http"

	"github.com/wujunhui99/agents_im/pkg/health"
	"github.com/wujunhui99/agents_im/pkg/observability"
	"github.com/wujunhui99/agents_im/pkg/response"
	"github.com/wujunhui99/agents_im/service/groups/api/internal/config"
	"github.com/wujunhui99/agents_im/service/groups/api/internal/handler"
	"github.com/wujunhui99/agents_im/service/groups/api/internal/svc"
	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/rest"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func main() {
	configFile := flag.String("f", "etc/groups-api.yaml", "the config file")
	flag.Parse()
	run(*configFile)
}

// run starts the groups-api service: it loads config and serves.
func run(configFile string) {
	var c config.Config
	conf.MustLoad(configFile, &c, conf.UseEnv())

	// tracing 由 go-zero 自带 Telemetry（yaml 配置）驱动：rest 内置 otel 中间件 + ServiceConf 启动 trace agent。
	serviceContext, err := svc.NewServiceContext(c)
	if err != nil {
		log.Fatalf("build groups api service context: %v", err)
	}

	httpx.SetErrorHandlerCtx(response.GoZeroErrorHandlerCtx)
	server := rest.MustNewServer(c.RestConf, rest.WithUnauthorizedCallback(response.GoZeroUnauthorizedCallback))
	defer server.Stop()
	registerObservabilityHandlers(server, serviceContext)
	handler.RegisterHandlers(server, serviceContext)

	log.Printf("%s listening on %s:%d", c.Name, c.Host, c.Port)
	server.Start()
}

func registerObservabilityHandlers(server *rest.Server, serviceContext *svc.ServiceContext) {
	server.AddRoutes([]rest.Route{
		{Method: http.MethodGet, Path: "/healthz", Handler: healthHandler("groups-api")},
		{Method: http.MethodGet, Path: "/readyz", Handler: health.ReadinessHandler("groups-api", func(*http.Request) []health.Check {
			return []health.Check{
				componentCheck("auth_config", serviceContext != nil && serviceContext.Config.Auth.AccessSecret != "", "configured"),
				componentCheck("groups_rpc", serviceContext != nil && serviceContext.GroupsRPC != nil, "configured"),
			}
		})},
		{Method: http.MethodGet, Path: "/metrics", Handler: observability.MetricsHandler()},
	})
}

func healthHandler(service string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		response.WriteOK(w, map[string]string{"service": service, "status": "ok"})
	}
}

func componentCheck(name string, ok bool, readyMessage string) health.Check {
	if ok {
		return health.ComponentCheck(name, true, readyMessage)
	}
	return health.ComponentCheck(name, false, "missing")
}
