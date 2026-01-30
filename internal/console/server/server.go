package server

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/xela07ax/spaceai-infra-prototype/internal/console/handler"
	"github.com/xela07ax/spaceai-infra-prototype/internal/console/service"
	"github.com/xela07ax/spaceai-infra-prototype/internal/infra"
	"github.com/xela07ax/spaceai-infra-prototype/internal/infra/auth"
	"go.uber.org/zap"
)

type ConsoleServer struct {
	router *chi.Mux
	logger *zap.Logger
	cfg    *infra.Config

	// Интерфейс для проверки токенов (RS256)
	// Реализуется через embedding BaseValidator в AuthService
	authValidator auth.TokenValidator

	// Обработчики бизнес-доменов
	authHandler     *handler.AuthHandler      // /auth/token
	agentHandler    *handler.AgentHandler     // /v1/agents
	policyHandler   *handler.PolicyHandler    // /v1/policies
	approvalHandler *handler.ApprovalHandler  // /v1/approvals (HITL)
	dashHandler     *handler.DashboardHandler // /api/v1/dashboard
	auditHandler    *handler.AuditHandler     // /v1/audit (Logs)
}

// NewConsoleServer инициализирует сервер админки со всеми зависимостями
func NewConsoleServer(
	cfg *infra.Config,
	logger *zap.Logger,
	agentService *service.AgentService,
	authH *handler.AuthHandler,
	agentH *handler.AgentHandler,
	policyH *handler.PolicyHandler,
	approvalH *handler.ApprovalHandler,
	dashH *handler.DashboardHandler,
	auditH *handler.AuditHandler,
) *ConsoleServer {
	s := &ConsoleServer{
		router:          chi.NewRouter(),
		logger:          logger.Named("console-api"),
		cfg:             cfg,
		authService:     agentService,
		authHandler:     authH,
		agentHandler:    agentH,
		policyHandler:   policyH,
		approvalHandler: approvalH,
		dashHandler:     dashH,
		auditHandler:    auditH,
	}

	s.routes()
	return s
}

func (s *ConsoleServer) routes() {
	r := s.router

	// --- 1. Глобальные инфраструктурные Middleware (для всех) ---
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	// --- 2. ПУБЛИЧНЫЕ РОУТЫ (Открыты для всех) ---
	r.Group(func(r chi.Router) {
		// Логин должен быть доступен без токена
		r.Post("/auth/token", s.authHandler.Login)

		// Опционально: Healthcheck для мониторинга
		r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})
	})

	// --- 3. ЗАЩИЩЕННЫЙ ПЕРИМЕТР (Требуют RS256 токен) ---
	r.Group(func(r chi.Router) {
		// Подключаем универсальный Middleware только для этой группы
		r.Use(auth.NewMiddleware(s.authService, s.logger))

		// Dashboard & Stats
		r.Get("/api/v1/dashboard/stats", s.dashHandler.GetStats)

		// Управление Агентами (Status, Kill-Switch)
		r.Route("/v1/agents", func(r chi.Router) {
			r.Get("/", s.agentHandler.List) // Список всех агентов
			r.Route("/{id}", func(r chi.Router) {
				r.Get("/", s.agentHandler.Get)                // Информация об агенте
				r.Post("/block", s.agentHandler.Block)        // Мгновенная блокировка (Kill-switch)
				r.Post("/unblock", s.agentHandler.Unblock)    // Разблокировка
				r.Post("/sandbox", s.agentHandler.SetSandbox) // Перевод в режим песочницы
			})
		})

		// Управление Политиками (Policy Engine)
		r.Route("/v1/policies", func(r chi.Router) {
			r.Get("/", s.policyHandler.List)    // Все активные политики
			r.Post("/", s.policyHandler.Create) // Создание новой (например, Wildcard '*')
			r.Route("/{id}", func(r chi.Router) {
				r.Get("/", s.policyHandler.Get)       // Детали политики
				r.Put("/", s.policyHandler.Update)    // Редактирование (Conditions/Effect)
				r.Delete("/", s.policyHandler.Delete) // Удаление
			})
		})

		// Human-in-the-loop (Approvals)
		r.Route("/v1/approvals", func(r chi.Router) {
			r.Get("/", s.approvalHandler.List) // Очередь запросов на проверку
			r.Route("/{id}", func(r chi.Router) {
				r.Get("/", s.approvalHandler.GetDetails)
				r.Post("/decide", s.approvalHandler.Decide) // Approve/Reject + Redis Publish
			})
		})
		// Аудит и Логи (Observability)
		r.Get("/v1/audit", s.auditHandler.GetLogs)
	})
}

// ServeHTTP позволяет использовать ConsoleServer как стандартный http.Handler
func (s *ConsoleServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.router.ServeHTTP(w, r)
}
