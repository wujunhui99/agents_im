// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package main

import (
	"context"
	"flag"
	"log"
	"net/http"

	"github.com/wujunhui99/agents_im/pkg/health"
	"github.com/wujunhui99/agents_im/pkg/observability"
	"github.com/wujunhui99/agents_im/pkg/response"
	apiconfig "github.com/wujunhui99/agents_im/service/agent/api/internal/config"
	"github.com/wujunhui99/agents_im/service/agent/api/internal/handler"
	"github.com/wujunhui99/agents_im/service/agent/api/internal/svc"
	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/rest"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func main() {
	configFile := flag.String("f", "etc/agent-api.yaml", "the config file")
	flag.Parse()
	run(*configFile)
}

// run starts the agent-api service: it loads config, wires the goctl-generated
// API internals, and serves.
func run(configFile string) {
	var c apiconfig.Config
	conf.MustLoad(configFile, &c, conf.UseEnv())

	shutdownTracing, err := observability.InitServiceTracing(context.Background(), c.Tracing, c.Name)
	if err != nil {
		log.Fatalf("init tracing: %v", err)
	}
	defer func() {
		if err := observability.ShutdownTracing(shutdownTracing); err != nil {
			log.Printf("shutdown tracing: %v", err)
		}
	}()

	serviceContext, err := svc.NewServiceContextFromConfig(c)
	if err != nil {
		log.Fatalf("build agent api service context: %v", err)
	}

	httpx.SetErrorHandlerCtx(response.GoZeroErrorHandlerCtx)
	server := rest.MustNewServer(c.RestConf, rest.WithUnauthorizedCallback(response.GoZeroUnauthorizedCallback))
	defer server.Stop()
	server.Use(observability.TraceMiddlewareFunc)
	registerAgentAPIServiceHandlers(server, serviceContext)

	log.Printf("%s listening on %s:%d", c.Name, c.Host, c.Port)
	server.Start()
}

// registerAgentAPIServiceHandlers wires business and observability routes. The
// service test reuses it to build the same router in-process.
func registerAgentAPIServiceHandlers(server *rest.Server, serviceContext *svc.ServiceContext) {
	registerObservabilityHandlers(server, serviceContext)
	handler.RegisterHandlers(server, serviceContext)
}

func registerObservabilityHandlers(server *rest.Server, serviceContext *svc.ServiceContext) {
	server.AddRoutes([]rest.Route{
		{
			Method:  http.MethodGet,
			Path:    "/healthz",
			Handler: healthHandler("agent-api"),
		},
		{
			Method: http.MethodGet,
			Path:   "/readyz",
			Handler: health.ReadinessHandler("agent-api", func(*http.Request) []health.Check {
				return []health.Check{
					componentCheck("auth_config", serviceContext != nil && serviceContext.AuthConfig().AccessSecret != "", "configured"),
					componentCheck("agent_rpc", serviceContext != nil && serviceContext.AgentRPC != nil, "configured"),
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
