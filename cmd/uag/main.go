package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/xela07ax/spaceai-infra-prototype/internal/infra"
	"github.com/xela07ax/spaceai-infra-prototype/internal/infra/auth"
	"github.com/xela07ax/spaceai-infra-prototype/internal/repository/postgres"
	"github.com/xela07ax/spaceai-infra-prototype/internal/risk"
	"go.uber.org/zap"

	"github.com/xela07ax/spaceai-infra-prototype/internal/audit"
	"github.com/xela07ax/spaceai-infra-prototype/internal/connectors"
	"github.com/xela07ax/spaceai-infra-prototype/internal/engine"
	"github.com/xela07ax/spaceai-infra-prototype/internal/policy"

	pb "github.com/xela07ax/spaceai-infra-prototype/pkg/api/connector/v1"

	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	// Загружаем конфиг (умеет читать файлы или ENV в []byte)
	cfg, err := infra.LoadConfig()
	if err != nil {
		log.Fatalf("Config error: %v", err)
	}

	// Парсим Публичный ключ
	pubKey, err := auth.ParseRSAPublicKey(cfg.Auth.PublicKey)
	if err != nil {
		log.Fatalf("Public Key error: %v", err)
	}

	logger, err := zap.NewProduction()
	if err != nil {
		log.Fatal(fmt.Sprintf("failed to create logger: %v", err))
	}
	defer logger.Sync() // Очистка буфера при выходе

	// 1. Инициализация базовых ресурсов (Infrastructure)
	rdb := redis.NewClient(&redis.Options{Addr: "localhost:6379"})

	// Контекст приложения живет до сигналов ОС
	appCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	// Репозиторий на Background контексте (чтобы не отвалился при Shutdown)
	auditStorage := postgres.NewAgentRepo(context.Background(), cfg)
	defer auditStorage.Close() // Закрываем пул в самом конце

	// 2. Инициализация Аудита (AgentFS)
	auditor := audit.NewAgentFS(auditStorage, logger)
	auditor.Start()      // Запускаем воркера
	defer auditor.Stop() // Гарантированный flush батча при выходе

	// 3. Коннектор (External Systems)
	conn, err := grpc.Dial("localhost:50051", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("failed to connect to connector: %v", err)
	}
	defer conn.Close()

	// 4. Control Plane Managers (KillSwitch, Sandbox, Quarantine)
	ksm := engine.NewKillSwitchManager(rdb, auditStorage, logger)
	sm := engine.NewSandboxManager(rdb, auditStorage, logger)
	qm := engine.NewQuarantineManager(rdb, auditStorage, logger)

	// ВАЖНО: Сначала запускаем Listeners (Subscribe), чтобы не пропустить дельту во время Init
	go ksm.StartListener(appCtx)
	go sm.StartListener(appCtx)
	go qm.StartListener(appCtx)

	// ТЕПЕРЬ делаем Init (Холодный прогрев из БД)
	if err := ksm.Init(appCtx); err != nil {
		log.Fatalf("Kill-switch init failed: %v", err)
	}
	if err := sm.Init(appCtx); err != nil {
		log.Fatalf("Sandbox init failed: %v", err)
	}
	if err := qm.Init(appCtx); err != nil {
		log.Fatalf("Quarantine init failed: %v", err)
	}

	// 5.1. Создаем движок Policy Engine
	enforcer := policy.NewMemoEnforcer(auditStorage, rdb, logger)
	// 5.2. Первичный прогрев (Warm-up)
	if err := enforcer.Refresh(appCtx); err != nil {
		logger.Fatal("Enforcer warm load policies failed", zap.Error(err))
	}
	// 5.3. Запускаем фоновую синхронизацию через Redis Pub/Sub
	go func() {
		pubsub := rdb.Subscribe(appCtx, infra.RedisChanPolicyUpdate)
		for _ = range pubsub.Channel() {
			err := enforcer.Refresh(appCtx)
			if err != nil {
				logger.Error("Enforcer refresh failed by signal", zap.Error(err))
			} // Мгновенно обновляем кэш при сигнале из админки
		}
	}()

	// 6. Execution Layer
	connectorClient := pb.NewConnectorServiceClient(conn)
	grpcAdapter := connectors.NewGRPCAdapter(connectorClient)
	executor := engine.NewReliabilityWrapper(grpcAdapter)

	// Risk Analyzer
	ra := risk.NewAnalyzer(ksm, logger)

	// 7. Metrics (Prometheus)
	reg := prometheus.NewRegistry()
	metrics := engine.NewMetrics(reg)
	go func() {
		http.Handle("/metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{}))
		log.Printf("Metrics exporter started on :9090")
		if err := http.ListenAndServe(":9090", nil); err != nil {
			log.Printf("Metrics server error: %v", err)
		}
	}()

	// 8. Core Engine & Middleware Chain
	v := auth.NewBaseValidator(pubKey)
	uag := engine.NewUAGCore(engine.UAGDeps{
		Validator:    v,
		Policy:       enforcer,
		Auditor:      auditor,
		Executor:     executor,
		Approver:     auditStorage,
		RiskAnalyzer: ra,
		KillSwitch:   ksm,
		Quarantine:   qm,
		Sandbox:      sm,
		Metrics:      metrics,
		Redis:        rdb,
		Logger:       logger,
	})

	// Роутинг
	// 1. Создаем универсальный Auth Middleware из пакета infra
	// uag реализует метод VerifyToken(string) (*domain.CustomClaims, error)
	authMiddleware := auth.NewMiddleware(uag, logger)

	uagRouter := chi.NewRouter()

	// 2. Инфраструктурный слой
	uagRouter.Use(engine.TracingMiddleware)
	uagRouter.Use(middleware.Logger)

	// 3. Слой безопасности (наш конвейер)
	uagRouter.Group(func(r chi.Router) {
		r.Use(authMiddleware) // <--- Используем новый из internal/infra/auth
		r.Use(ksm.Middleware) // Kill-switch
		r.Use(sm.Middleware)  // Sandbox

		r.Post("/v1/execute", uag.HandleHTTPRequest)
	})

	// Прикрепляем к основному серверу
	srv := &http.Server{
		Addr:    ":8080",
		Handler: uagRouter,
	}

	grpcSrv := grpc.NewServer(grpc.UnaryInterceptor(engine.UnaryAuthInterceptor(uag)))
	pb.RegisterConnectorServiceServer(grpcSrv, engine.NewGRPCGatewayServer(uag))

	go func() {
		lis, err := net.Listen("tcp", ":50052")
		if err != nil {
			log.Fatalf("gRPC listen error: %v", err)
		}
		log.Printf("UAG gRPC Server started on :50052")
		grpcSrv.Serve(lis)
	}()

	go func() {
		log.Printf("UAG HTTP Engine started on %s", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP listen error: %s", err)
		}
	}()

	// 10. Graceful Shutdown (The Master Sequence)
	<-appCtx.Done()
	log.Print("UAG Engine shutting down...")

	// Тайм-аут на закрытие серверов
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	grpcSrv.GracefulStop()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("HTTP Server Shutdown Failed: %v", err)
	}

	// ВАЖНО: Аудитор стопается ПОСЛЕ серверов, чтобы слить последние логи
	auditor.Stop()

	log.Print("UAG Engine exited gracefully")
}
