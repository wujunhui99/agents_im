package entry

import (
	"context"
	"log"
	"net/http"

	appconfig "github.com/wujunhui99/agents_im/internal/config"
	legacyhandler "github.com/wujunhui99/agents_im/internal/handler"
	"github.com/wujunhui99/agents_im/internal/health"
	"github.com/wujunhui99/agents_im/internal/observability"
	"github.com/wujunhui99/agents_im/internal/response"
	"github.com/wujunhui99/agents_im/service/message/api/internal/config"
	"github.com/wujunhui99/agents_im/service/message/api/internal/handler"
	"github.com/wujunhui99/agents_im/service/message/api/internal/svc"
	"github.com/zeromicro/go-zero/rest"
	"github.com/zeromicro/go-zero/rest/httpx"
)

// Start bridges cmd/message-api to the service/message/api goctl-generated API internals.
// cmd/message-api cannot import service/message/api/internal/* directly because
// of Go internal package visibility.
func Start(configFile string) {
	c, err := config.Load(configFile)
	if err != nil {
		log.Fatalf("load message api config: %v", err)
	}
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
		log.Fatalf("build message api service context: %v", err)
	}
	adminContext, err := svc.NewAdminServiceContext(serviceContext)
	if err != nil {
		log.Fatalf("build message api admin service context: %v", err)
	}

	httpx.SetErrorHandlerCtx(response.GoZeroErrorHandlerCtx)
	server := rest.MustNewServer(appconfig.ToRestConf(c), rest.WithUnauthorizedCallback(response.GoZeroUnauthorizedCallback))
	defer server.Stop()
	server.Use(observability.TraceMiddlewareFunc)
	registerObservabilityHandlers(server, serviceContext)
	handler.RegisterHandlers(server, serviceContext)
	legacyhandler.RegisterAdminGoZeroHandlers(server, adminContext)

	log.Printf("%s listening on %s:%d", c.Name, c.Host, c.Port)
	server.Start()
}

func registerObservabilityHandlers(server *rest.Server, serviceContext *svc.ServiceContext) {
	server.AddRoutes([]rest.Route{
		{
			Method:  http.MethodGet,
			Path:    "/healthz",
			Handler: healthHandler("message-api"),
		},
		{
			Method: http.MethodGet,
			Path:   "/readyz",
			Handler: health.ReadinessHandler("message-api", func(*http.Request) []health.Check {
				return []health.Check{
					componentCheck("auth_config", serviceContext != nil && serviceContext.AuthConfig().AccessSecret != "", "configured"),
					componentCheck("message_logic", serviceContext != nil && serviceContext.MessageLogic != nil, "configured"),
					componentCheck("ai_hosting_logic", serviceContext != nil && serviceContext.AIHostingLogic != nil, "configured"),
					componentCheck("message_repository", serviceContext != nil && serviceContext.MessageRepo != nil, "configured"),
					componentCheck("feedback_logic", serviceContext != nil && serviceContext.FeedbackLogic != nil, "configured"),
					componentCheck("feedback_repository", serviceContext != nil && serviceContext.FeedbackRepo != nil, "configured"),
					componentCheck("ai_hosting_repository", serviceContext != nil && serviceContext.AIHostingRepo != nil, "configured"),
					componentCheck("outbox_repository", serviceContext != nil && serviceContext.OutboxRepo != nil, "configured"),
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
