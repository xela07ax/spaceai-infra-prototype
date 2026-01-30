package main

import (
	"context"
	"crypto/rsa"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/xela07ax/spaceai-infra-prototype/internal/console/handler"
	"github.com/xela07ax/spaceai-infra-prototype/internal/console/server"
	"github.com/xela07ax/spaceai-infra-prototype/internal/console/service"
	"github.com/xela07ax/spaceai-infra-prototype/internal/infra"
	"github.com/xela07ax/spaceai-infra-prototype/internal/infra/auth"
	"github.com/xela07ax/spaceai-infra-prototype/internal/repository/postgres" // Пример реализации БД
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"

	"github.com/redis/go-redis/v9"
)

func main() {
	// Проверяем режим запуска
	checkBycript()

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

	// 3. Парсим Приватный ключ (нужен для выдачи токенов)
	// Делаем проверку, так как он может быть пустым
	var privKey *rsa.PrivateKey
	if len(cfg.Auth.PrivateKey) > 0 {
		privKey, err = auth.ParseRSAPrivateKey(cfg.Auth.PrivateKey)
		if err != nil {
			log.Fatalf("Private Key error: %v", err)
		}
	}

	logger, err := zap.NewProduction()
	if err != nil {
		log.Fatal(fmt.Sprintf("failed to create logger: %v", err))
	}
	defer logger.Sync() // Очистка буфера при выходе

	// --- 1. Инфраструктурный слой ---
	rdb := redis.NewClient(&redis.Options{Addr: cfg.Redis.Addr})
	pgRepo := postgres.NewAgentRepo(context.Background(), cfg) // Твой универсальный AuditStorage/Repo

	// 1.1. Создаем валидатор с публичным ключом
	pubKey, err := infra.GetPublicKey(cfg)
	if err != nil {
		logger.Fatal(fmt.Sprintf("failed to get public key: %v", err))
	}
	validatorWithKey := auth.NewBaseValidator(pubKey)
	// 1.2. Прокидываем его в сервис (он там встроится через Embedding)
	agentService := service.NewAgentService(rdb, pgRepo, validatorWithKey, logger)

	privKey, err := infra.GetPrivateKey(cfg)
	if err != nil {
		logger.Fatal(fmt.Sprintf("failed to get private key: %v", err))
	}
	authService := service.NewAuthService(pgRepo, privKey)
	authHandler := handler.NewAuthHandler(authService)

	// --- 2. Сервисный слой (Бизнес-логика) ---
	// AgentService теперь — центральный узел для управления агентами и статами

	// PolicyService управляет правилами доступа
	policyService := service.NewPolicyService(pgRepo, rdb)

	// AuditService отвечает за чтение логов
	auditService := service.NewAuditService(pgRepo)

	// --- 3. Слой доставки (Handlers) ---
	agentHandler := handler.NewAgentHandler(agentService, logger)
	dashHandler := handler.NewDashboardHandler(agentService)
	approvalHandler := handler.NewApprovalHandler(agentService)

	// Не забываем про Policy и Audit хендлеры
	policyHandler := handler.NewPolicyHandler(policyService)
	auditHandler := handler.NewAuditHandler(auditService)

	// --- 4. Запуск Console API (Control Plane) ---
	// Передаем валидатор через конструктор сервера или сервиса (как мы решили через Embedding)
	// Здесь мы передаем всё в наш NewConsoleServer, который соберет роуты chi
	consoleServer := server.NewConsoleServer(
		cfg,
		logger,
		agentService,
		authHandler,
		agentHandler,
		policyHandler,
		approvalHandler,
		dashHandler,
		auditHandler,
	)

	// --- Настройка и Запуск Сервера ---
	srv := &http.Server{
		Addr:         ":8000",
		Handler:      consoleServer,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second, // Даем больше времени на тяжелые аналитические запросы
		IdleTimeout:  120 * time.Second,
	}

	// Запуск в отдельной горутине, чтобы не блокировать основной поток для Shutdown сигналов
	go func() {
		log.Printf("🚀 Console API started on %s", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Critical: listen error: %v", err)
		}
	}()

	// Здесь должен быть твой блок ожидания SIGTERM/SIGINT,
	// который мы писали ранее для Graceful Shutdown.
}

func checkBycript() {
	// 1. Описываем флаг для генерации хеша
	genHash := flag.String("gen-password", "", "Generate bcrypt hash for a given password and exit")
	flag.Parse()

	// 2. Если флаг передан — генерируем и выходим
	if *genHash != "" {
		hash, err := bcrypt.GenerateFromPassword([]byte(*genHash), bcrypt.DefaultCost)
		if err != nil {
			fmt.Printf("Error generating hash: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("\n--- PASSWORD HASH GENERATOR ---\n")
		fmt.Printf("Password: %s\n", *genHash)
		fmt.Printf("Bcrypt Hash: %s\n", string(hash))
		fmt.Printf("-------------------------------\n")
		fmt.Printf("Copy this hash to your 'users' table in PostgreSQL.\n\n")
		os.Exit(0) // Завершаем работу, сервер не запускаем
	}
}
