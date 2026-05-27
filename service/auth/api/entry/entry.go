package entry

import (
	"context"
	"log"
	"net/http"

	appconfig "github.com/wujunhui99/agents_im/internal/config"
	"github.com/wujunhui99/agents_im/internal/health"
	"github.com/wujunhui99/agents_im/internal/observability"
	"github.com/wujunhui99/agents_im/internal/response"
	"github.com/wujunhui99/agents_im/service/auth/api/internal/config"
	"github.com/wujunhui99/agents_im/service/auth/api/internal/handler"
	"github.com/wujunhui99/agents_im/service/auth/api/internal/svc"
	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/rest"
	"github.com/zeromicro/go-zero/rest/httpx"
)

// Start bridges cmd/auth-api to the service/auth/api goctl-generated API internals.
// cmd/auth-api cannot import service/auth/api/internal/* directly because of Go
// internal package visibility.
func Start(configFile string) {
	var c config.Config
	conf.MustLoad(configFile, &c)
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
		log.Fatalf("build auth api service context: %v", err)
	}

	httpx.SetErrorHandlerCtx(response.GoZeroErrorHandlerCtx)
	server := rest.MustNewServer(c.RestConf)
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
			Handler: healthHandler("auth-api"),
		},
		{
			Method: http.MethodGet,
			Path:   "/readyz",
			Handler: health.ReadinessHandler("auth-api", func(*http.Request) []health.Check {
				return []health.Check{
					componentCheck("auth_rpc", serviceContext != nil && serviceContext.AuthRPC != nil, "configured"),
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
