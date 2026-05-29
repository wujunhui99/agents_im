package entry

import (
	"context"
	"log"
	"net/http"

	"github.com/wujunhui99/agents_im/pkg/pythonexec"
	appconfig "github.com/wujunhui99/agents_im/pkg/config"
	"github.com/wujunhui99/agents_im/pkg/health"
	"github.com/wujunhui99/agents_im/internal/logic"
	"github.com/wujunhui99/agents_im/pkg/observability"
	"github.com/wujunhui99/agents_im/internal/repository"
	"github.com/wujunhui99/agents_im/pkg/response"
	"github.com/wujunhui99/agents_im/service/agent/api/internal/handler"
	"github.com/wujunhui99/agents_im/service/agent/api/internal/svc"
	"github.com/zeromicro/go-zero/rest"
	"github.com/zeromicro/go-zero/rest/httpx"
)

type ServiceContext = svc.ServiceContext

// Start launches the agent-api service, wiring the goctl-generated API internals. It lives in the entry package so the service
// binary and tests can share one startup path.
func Start(configFile string) {
	cfg, err := appconfig.LoadAPIConfig(configFile)
	if err != nil {
		log.Fatalf("load api config: %v", err)
	}
	restConf := appconfig.ToRestConf(cfg)

	shutdownTracing, err := observability.InitServiceTracing(context.Background(), cfg.Tracing, cfg.Name)
	if err != nil {
		log.Fatalf("init tracing: %v", err)
	}
	defer func() {
		if err := observability.ShutdownTracing(shutdownTracing); err != nil {
			log.Printf("shutdown tracing: %v", err)
		}
	}()

	serviceContext, err := svc.NewServiceContextFromConfig(cfg)
	if err != nil {
		log.Fatalf("build agent api service context: %v", err)
	}

	httpx.SetErrorHandlerCtx(response.GoZeroErrorHandlerCtx)
	server := rest.MustNewServer(restConf, rest.WithUnauthorizedCallback(response.GoZeroUnauthorizedCallback))
	defer server.Stop()
	server.Use(observability.TraceMiddlewareFunc)
	RegisterAgentAPIServiceHandlers(server, serviceContext)

	log.Printf("%s listening on %s:%d", cfg.Name, cfg.Host, cfg.Port)
	server.Start()
}

func RegisterAgentAPIServiceHandlers(server *rest.Server, serviceContext *ServiceContext) {
	registerObservabilityHandlers(server, serviceContext)
	handler.RegisterHandlers(server, serviceContext)
}

func NewServiceContextWithAuth(repo repository.AgentRepository, accountTypeChecker logic.UserAccountTypeChecker, auth appconfig.JWTAuthConfig) *ServiceContext {
	return svc.NewServiceContextWithAuth(repo, accountTypeChecker, auth)
}

func NewServiceContextWithAuthAndPythonExecutor(repo repository.AgentRepository, accountTypeChecker logic.UserAccountTypeChecker, auth appconfig.JWTAuthConfig, executor pythonexec.Executor) *ServiceContext {
	return svc.NewServiceContextWithAuthAndPythonExecutor(repo, accountTypeChecker, auth, executor)
}

func registerObservabilityHandlers(server *rest.Server, serviceContext *ServiceContext) {
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
					componentCheck("agent_logic", serviceContext != nil && serviceContext.AgentLogic != nil, "configured"),
					componentCheck("agent_definition_logic", serviceContext != nil && serviceContext.AgentDefinitionLogic != nil, "configured"),
					componentCheck("agent_repository", serviceContext != nil && serviceContext.AgentRepo != nil, "configured"),
					componentCheck("agent_registry_repository", serviceContext != nil && serviceContext.AgentRegistryRepo != nil, "configured"),
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
