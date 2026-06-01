package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/wujunhui99/agents_im/pkg/apperror"
	authrepo "github.com/wujunhui99/agents_im/internal/auth/repository"
	"github.com/wujunhui99/agents_im/common/share/auth/token"
	appconfig "github.com/wujunhui99/agents_im/pkg/config"
	"github.com/wujunhui99/agents_im/pkg/ctxuser"
	"github.com/wujunhui99/agents_im/pkg/health"
	"github.com/wujunhui99/agents_im/common/share/model"
	"github.com/wujunhui99/agents_im/pkg/observability"
	"github.com/wujunhui99/agents_im/internal/repository"
	"github.com/wujunhui99/agents_im/pkg/response"
	"github.com/wujunhui99/agents_im/service/admin/api/internal/config"
	adminhandler "github.com/wujunhui99/agents_im/service/admin/api/internal/handler/admin"
	"github.com/wujunhui99/agents_im/service/admin/api/internal/svc"
	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/rest"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func main() {
	configFile := flag.String("f", "etc/admin-api.yaml", "the config file")
	flag.Parse()
	run(*configFile)
}

func run(configFile string) {
	var c config.Config
	conf.MustLoad(configFile, &c, conf.UseEnv())
	if c.Auth.AccessSecret == "" {
		c.Auth.AccessSecret = appconfig.DefaultJWTAuthConfig().AccessSecret
	}
	c.Telemetry = appconfig.GoZeroTelemetryConfig(c.Tracing, c.Name)
	log.Printf("starting %s on port %d", c.Name, c.Port)

	shutdownTracing, err := observability.InitServiceTracing(context.Background(), c.Tracing, c.Name)
	if err != nil {
		log.Fatalf("init tracing: %v", err)
	}
	defer func() {
		if err := observability.ShutdownTracing(shutdownTracing); err != nil {
			log.Printf("shutdown tracing: %v", err)
		}
	}()

	auth := appconfig.JWTAuthConfig{
		AccessSecret: c.Auth.AccessSecret,
		AccessExpire: c.Auth.AccessExpire,
	}

	accountRepo, err := repository.NewRepositoryForStorage(c.StorageDriver, c.DataSource)
	if err != nil {
		log.Fatalf("build account repository: %v", err)
	}
	messageRepo, err := repository.NewMessageRepositoryForStorage(c.StorageDriver, c.DataSource)
	if err != nil {
		log.Fatalf("build message repository: %v", err)
	}
	agentAuditRepo, err := repository.NewAgentAuditRepositoryForStorage(c.StorageDriver, c.DataSource)
	if err != nil {
		log.Fatalf("build agent audit repository: %v", err)
	}
	feedbackRepo, err := repository.NewFeedbackRepositoryForStorage(c.StorageDriver, c.DataSource)
	if err != nil {
		log.Fatalf("build feedback repository: %v", err)
	}
	taskReportRepo, err := repository.NewTaskReportRepositoryForStorage(c.StorageDriver, c.DataSource)
	if err != nil {
		log.Fatalf("build task report repository: %v", err)
	}

	var serviceContext *svc.ServiceContext

	if appconfig.ResolveStorageDriver(c.StorageDriver) == appconfig.StorageDriverPostgres {
		postgresAccountRepo, ok := accountRepo.(*repository.PostgresRepository)
		if !ok {
			log.Fatalf("postgres account repository has unexpected type %T", accountRepo)
		}
		postgresMessageRepo, ok := messageRepo.(*repository.PostgresMessageRepository)
		if !ok {
			log.Fatalf("postgres message repository has unexpected type %T", messageRepo)
		}
		postgresAgentAuditRepo, ok := agentAuditRepo.(*repository.PostgresAgentAuditRepository)
		if !ok {
			log.Fatalf("postgres agent audit repository has unexpected type %T", agentAuditRepo)
		}
		postgresFeedbackRepo, ok := feedbackRepo.(*repository.PostgresFeedbackRepository)
		if !ok {
			log.Fatalf("postgres feedback repository has unexpected type %T", feedbackRepo)
		}
		postgresTaskReportRepo, ok := taskReportRepo.(*repository.PostgresTaskReportRepository)
		if !ok {
			log.Fatalf("postgres task report repository has unexpected type %T", taskReportRepo)
		}
		serviceContext = svc.NewServiceContextWithAuth(svc.Dependencies{
			Accounts:    postgresAccountRepo,
			Friends:     postgresAccountRepo,
			Messages:    postgresMessageRepo,
			AgentAudits: postgresAgentAuditRepo,
			Feedback:    postgresFeedbackRepo,
			TaskReports: postgresTaskReportRepo,
		}, auth)

		authRepo, err := authrepo.NewRepositoryForStorage(c.StorageDriver, c.DataSource)
		if err != nil {
			log.Fatalf("build auth repository: %v", err)
		}
		serviceContext.AuthSessions = authRepo
	} else {
		memoryAccountRepo, ok := accountRepo.(*repository.MemoryRepository)
		if !ok {
			log.Fatalf("memory account repository has unexpected type %T", accountRepo)
		}
		memoryMessageRepo, ok := messageRepo.(*repository.MemoryMessageRepository)
		if !ok {
			log.Fatalf("memory message repository has unexpected type %T", messageRepo)
		}
		memoryAgentAuditRepo, ok := agentAuditRepo.(*repository.MemoryAgentAuditRepository)
		if !ok {
			log.Fatalf("memory agent audit repository has unexpected type %T", agentAuditRepo)
		}
		memoryFeedbackRepo, ok := feedbackRepo.(*repository.MemoryFeedbackRepository)
		if !ok {
			log.Fatalf("memory feedback repository has unexpected type %T", feedbackRepo)
		}
		memoryTaskReportRepo, ok := taskReportRepo.(*repository.MemoryTaskReportRepository)
		if !ok {
			log.Fatalf("memory task report repository has unexpected type %T", taskReportRepo)
		}
		serviceContext = svc.NewServiceContextWithAuth(svc.Dependencies{
			Accounts:    memoryAccountRepo,
			Friends:     memoryAccountRepo,
			Messages:    memoryMessageRepo,
			AgentAudits: memoryAgentAuditRepo,
			Feedback:    memoryFeedbackRepo,
			TaskReports: memoryTaskReportRepo,
		}, auth)
		log.Printf("active session shared validation disabled for storage driver %q; use postgres for single-device enforcement across services", appconfig.ResolveStorageDriver(c.StorageDriver))
	}

	httpx.SetErrorHandlerCtx(response.GoZeroErrorHandlerCtx)
	server := rest.MustNewServer(c.RestConf, rest.WithUnauthorizedCallback(response.GoZeroUnauthorizedCallback))
	defer server.Stop()
	server.Use(observability.TraceMiddlewareFunc)
	registerAdminHandlers(server, serviceContext)

	log.Printf("%s listening on %s:%d", c.Name, c.Host, c.Port)
	server.Start()
}

func registerAdminHandlers(server *rest.Server, serverCtx *svc.ServiceContext) {
	server.AddRoutes([]rest.Route{
		{
			Method:  http.MethodGet,
			Path:    "/healthz",
			Handler: healthHandler("admin-api"),
		},
		{
			Method:  http.MethodGet,
			Path:    "/readyz",
			Handler: health.ReadinessHandler("admin-api", func(*http.Request) []health.Check {
				return []health.Check{
					componentCheck("auth_config", serverCtx != nil && serverCtx.Auth.AccessSecret != "", "configured"),
					componentCheck("admin_logic", serverCtx != nil && serverCtx.AdminLogic != nil, "configured"),
					componentCheck("accounts", serverCtx != nil && serverCtx.Accounts != nil, "configured"),
				}
			}),
		},
		{
			Method:  http.MethodGet,
			Path:    "/metrics",
			Handler: observability.MetricsHandler(),
		},
	})

	routes := []rest.Route{
		{
			Method:  http.MethodGet,
			Path:    "/admin/dashboard",
			Handler: adminhandler.DashboardHandler(serverCtx),
		},
		{
			Method:  http.MethodGet,
			Path:    "/admin/llm-traces",
			Handler: adminhandler.ListLLMTracesHandler(serverCtx),
		},
		{
			Method:  http.MethodGet,
			Path:    "/admin/llm-traces/:trace_id",
			Handler: adminhandler.GetLLMTraceHandler(serverCtx),
		},
		{
			Method:  http.MethodGet,
			Path:    "/api/admin/feedback",
			Handler: adminhandler.ListFeedbackHandler(serverCtx),
		},
		{
			Method:  http.MethodGet,
			Path:    "/api/admin/feedback/:feedback_id",
			Handler: adminhandler.GetFeedbackHandler(serverCtx),
		},
		{
			Method:  http.MethodPatch,
			Path:    "/api/admin/feedback/:feedback_id",
			Handler: adminhandler.UpdateFeedbackHandler(serverCtx),
		},
		{
			Method:  http.MethodGet,
			Path:    "/api/admin/task-reports",
			Handler: adminhandler.ListTaskReportsHandler(serverCtx),
		},
		{
			Method:  http.MethodPost,
			Path:    "/api/admin/task-reports",
			Handler: adminhandler.IngestTaskReportHandler(serverCtx),
		},
		{
			Method:  http.MethodGet,
			Path:    "/admin/conversations/:conversation_id/messages",
			Handler: adminhandler.GetConversationMessagesHandler(serverCtx),
		},
		{
			Method:  http.MethodPost,
			Path:    "/admin/conversations/:conversation_id/messages/:server_msg_id/replay-agent",
			Handler: adminhandler.ReplayAgentMessageHandler(serverCtx),
		},
		{
			Method:  http.MethodGet,
			Path:    "/admin/users",
			Handler: adminhandler.SearchUsersHandler(serverCtx),
		},
		{
			Method:  http.MethodGet,
			Path:    "/admin/users/:account_id",
			Handler: adminhandler.GetUserHandler(serverCtx),
		},
		{
			Method:  http.MethodGet,
			Path:    "/admin/users/:account_id/friends",
			Handler: adminhandler.GetUserFriendsHandler(serverCtx),
		},
		{
			Method:  http.MethodGet,
			Path:    "/admin/users/:account_id/conversations",
			Handler: adminhandler.GetUserConversationsHandler(serverCtx),
		},
	}
	routes = authenticatedRoutes(serverCtx, routes)
	routes = rest.WithMiddleware(adminOnlyMiddleware(serverCtx), routes...)
	server.AddRoutes(routes, rest.WithJwt(serverCtx.AuthConfig().AccessSecret))
}

func authenticatedRoutes(serverCtx *svc.ServiceContext, routes []rest.Route) []rest.Route {
	if serverCtx == nil || serverCtx.ActiveSessionRepository() == nil {
		return routes
	}
	return rest.WithMiddleware(activeSessionMiddleware(serverCtx), routes...)
}

func activeSessionMiddleware(serverCtx *svc.ServiceContext) rest.Middleware {
	auth := serverCtx.AuthConfig()
	activeSessions := serverCtx.ActiveSessionRepository()
	tokenManager := token.NewHMACTokenManager(auth.AccessSecret, time.Duration(auth.AccessExpire)*time.Second)
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			claims, err := tokenManager.Validate(bearerToken(r.Header.Get("Authorization")))
			if err != nil {
				httpx.ErrorCtx(r.Context(), w, err)
				return
			}
			if err := authrepo.ValidateActiveSession(r.Context(), activeSessions, claims); err != nil {
				httpx.ErrorCtx(r.Context(), w, err)
				return
			}
			next(w, r)
		}
	}
}

func adminOnlyMiddleware(serverCtx *svc.ServiceContext) rest.Middleware {
	auth := appconfig.DefaultJWTAuthConfig()
	if serverCtx != nil {
		auth = serverCtx.AuthConfig()
	}
	tokenManager := token.NewHMACTokenManager(auth.AccessSecret, time.Duration(auth.AccessExpire)*time.Second)
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			if serverCtx == nil || serverCtx.Accounts == nil {
				httpx.ErrorCtx(r.Context(), w, apperror.Internal("admin account repository is not configured"))
				return
			}
			userID, err := ctxuser.UserID(r.Context())
			if err != nil {
				claims, tokenErr := tokenManager.Validate(bearerToken(r.Header.Get("Authorization")))
				if tokenErr != nil {
					httpx.ErrorCtx(r.Context(), w, tokenErr)
					return
				}
				userID = claims.UserID
			}
			account, err := serverCtx.Accounts.GetByID(r.Context(), userID)
			if err != nil {
				httpx.ErrorCtx(r.Context(), w, err)
				return
			}
			if account.AccountType != model.AccountTypeAdmin {
				httpx.ErrorCtx(r.Context(), w, apperror.Forbidden("admin account is required"))
				return
			}
			next(w, r)
		}
	}
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

func bearerToken(headerValue string) string {
	headerValue = strings.TrimSpace(headerValue)
	if headerValue == "" {
		return ""
	}
	parts := strings.Fields(headerValue)
	if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
		return strings.TrimSpace(parts[1])
	}
	return ""
}
