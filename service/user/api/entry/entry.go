package entry

import (
	"context"
	"log"
	"net/http"

	appconfig "github.com/wujunhui99/agents_im/pkg/config"
	"github.com/wujunhui99/agents_im/pkg/health"
	"github.com/wujunhui99/agents_im/pkg/observability"
	"github.com/wujunhui99/agents_im/pkg/response"
	"github.com/wujunhui99/agents_im/service/user/api/internal/config"
	"github.com/wujunhui99/agents_im/service/user/api/internal/handler"
	"github.com/wujunhui99/agents_im/service/user/api/internal/svc"
	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/rest"
	"github.com/zeromicro/go-zero/rest/httpx"
)

// Start bridges cmd/user-api to the service/user/api goctl-generated API internals.
// cmd/user-api cannot import service/user/api/internal/* directly because of Go
// internal package visibility.
func Start(configFile string) {
	var c config.Config
	conf.MustLoad(configFile, &c, conf.UseEnv())
	c.Telemetry = appconfig.GoZeroTelemetryConfig(c.Tracing, c.Name)

	shutdownTracing, err := observability.InitServiceTracing(context.Background(), c.Tracing, c.Name)
	if err != nil {
		log.Fatalf("init tracing: %v", err)
	}
	defer func() {
		if err := observability.ShutdownTracing(shutdownTracing); err != nil {
			log.Printf("shutdown tracing: %v", err)
		}
	}()

	serviceContext, err := svc.NewServiceContext(c)
	if err != nil {
		log.Fatalf("build user api service context: %v", err)
	}

	httpx.SetErrorHandlerCtx(response.GoZeroErrorHandlerCtx)
	server := rest.MustNewServer(c.RestConf, rest.WithUnauthorizedCallback(response.GoZeroUnauthorizedCallback))
	defer server.Stop()
	server.Use(observability.TraceMiddlewareFunc)
	registerObservabilityHandlers(server, serviceContext)
	handler.RegisterHandlers(server, serviceContext)

	log.Printf("%s listening on %s:%d", c.Name, c.Host, c.Port)
	server.Start()
}

func registerObservabilityHandlers(server *rest.Server, serviceContext *svc.ServiceContext) {
	server.AddRoutes([]rest.Route{
		{
			Method:  http.MethodGet,
			Path:    "/healthz",
			Handler: healthHandler("user-api"),
		},
		{
			Method: http.MethodGet,
			Path:   "/readyz",
			Handler: health.ReadinessHandler("user-api", func(*http.Request) []health.Check {
				return []health.Check{
					componentCheck("auth_config", serviceContext != nil && serviceContext.Config.Auth.AccessSecret != "", "configured"),
					componentCheck("user_rpc", serviceContext != nil && serviceContext.UserRPC != nil, "configured"),
				}
			}),
		},
		{
			Method:  http.MethodGet,
			Path:    "/metrics",
			Handler: observability.MetricsHandler(),
		},
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
