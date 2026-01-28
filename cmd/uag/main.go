package main

import (
	"context"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/xela07ax/spaceai-infra-prototype/internal/repository/postgres"

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
	// 1. Инфраструктура и ресурсы
	rdb := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
	// Инициализируем Postgres для Аудита
	dbURL := os.Getenv("DB_URL")
	if dbURL == "" {
		log.Fatal("DB_URL environment variable is required")
	}
	auditStorage := postgres.NewAuditRepo(dbURL)
	// Теперь данные полетят в базу пачками
	agentFS := audit.NewAgentFS(auditStorage)

	// Настраиваем gRPC соединение с коннектором (например, Jira Connector)
	// В реальном проде адрес будет из конфига или Service Discovery
	conn, err := grpc.Dial("localhost:50051", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("failed to connect to connector: %v", err)
	}
	defer conn.Close()

	// Контекст для управления жизненным циклом фоновых горутин
	// При завершении main() или срабатывании SIGTERM, cancel() остановит слушателей
	appCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 2. Control Plane (Менеджеры управления)
	ksm := engine.NewKillSwitchManager(rdb)
	if err := ksm.Init(appCtx); err != nil {
		log.Fatalf("Failed to init kill-switch manager: %v", err)
	}
	go ksm.StartListener(appCtx)

	qm := engine.NewQuarantineManager(rdb)
	if err := qm.Init(appCtx); err != nil {
		log.Fatalf("Failed to init quarantine manager: %v", err)
	}
	go qm.StartListener(appCtx)

	sm := engine.NewSandboxManager(rdb)
	if err := sm.Init(appCtx); err != nil {
		log.Fatalf("Failed to init sandbox manager: %v", err)
	}
	go sm.StartListener(appCtx)

	// 3. Execution Layer (Исполнение + Надежность)
	connectorClient := pb.NewConnectorServiceClient(conn)
	grpcAdapter := connectors.NewGRPCAdapter(connectorClient)
	// Оборачиваем в Reliability (Retries, Circuit Breaker)
	safeExecutor := engine.NewReliabilityWrapper(grpcAdapter)

	// Метрики
	reg := prometheus.NewRegistry()
	metrics := engine.NewMetrics(reg)

	// Экспортируем метрики для Prometheus
	go func() {
		http.Handle("/metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{}))
		log.Fatal(http.ListenAndServe(":9090", nil))
	}()

	// 4. Core (Сборка ядра UAG)
	uag := engine.NewUAGCore(
		&policy.MemoEnforcer{}, // MVP политика
		agentFS,
		safeExecutor,
		ksm,
		qm,
		sm,
		metrics,
	)

	// 5. HTTP Server
	// Middleware. Порядок важен: Trace -> KillSwitch -> Sandbox
	// Цепочка защиты (снизу вверх)
	endpoint := http.HandlerFunc(uag.HandleHTTPRequest)

	protectedHandler := engine.TracingMiddleware( // 1. Присваиваем Trace-ID
		uag.AuthMiddleware( // 2. Проверяем токен и Scopes (ИБ-слой)
			ksm.Middleware( // 3. Проверяем блокировку (Kill-Switch)
				sm.Middleware( // 4. Проверяем режим песочницы
					endpoint, // 5. Исполняем запрос
				),
			),
		),
	)

	// Регистрируем в роутере
	mux := http.NewServeMux()
	mux.Handle("/v1/execute", protectedHandler)

	srv := &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}

	// Инициализируем gRPC сервер шлюза
	grpcSrv := grpc.NewServer(grpc.UnaryInterceptor(engine.UnaryAuthInterceptor(uag)))
	gatewaySrv := engine.NewGRPCGatewayServer(uag)
	pb.RegisterConnectorServiceServer(grpcSrv, gatewaySrv)

	// Запускаем gRPC в отдельной горутине
	go func() {
		lis, err := net.Listen("tcp", ":50052")
		if err != nil {
			log.Fatalf("failed to listen gRPC: %v", err)
		}
		log.Printf("UAG gRPC Server started on :50052")
		if err := grpcSrv.Serve(lis); err != nil {
			log.Fatalf("failed to serve gRPC: %v", err)
		}
	}()

	// 6. Graceful Shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		log.Printf("UAG Engine started on %s", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %s\n", err)
		}
	}()

	<-stop // Ждем сигнал
	log.Print("UAG Engine stopping...")

	// Даем 5 секунд на завершение запросов
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("Server Shutdown Failed: %+v", err)
	}
	grpcSrv.GracefulStop()
	log.Print("UAG Engine exited properly")
}
