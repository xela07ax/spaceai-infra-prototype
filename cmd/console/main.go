package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/xela07ax/spaceai-infra-prototype/internal/console/handler"
	"github.com/xela07ax/spaceai-infra-prototype/internal/console/service"
	"github.com/xela07ax/spaceai-infra-prototype/internal/repository/postgres" // Пример реализации БД

	"github.com/go-chi/chi/v5"
	"github.com/redis/go-redis/v9"
)

func main() {
	// 1. Инициализация ресурсов
	rdb := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
	dbURL := os.Getenv("DB_URL")
	if dbURL == "" {
		log.Fatal("DB_URL environment variable is required")
	}
	pgRepo := postgres.NewAgentRepo(dbURL)
	// Проверяем соединение с таймаутом
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	if err := pgRepo.Ping(ctx); err != nil {
		log.Fatalf("Database unreachable: %v", err)
	}
	cancel()

	// 2. Инициализация слоев (Dependency Injection)
	agentService := service.NewAgentService(pgRepo, rdb)
	agentHandler := handler.NewAgentHandler(agentService)

	// 3. Настройка роутера
	r := chi.NewRouter()
	r.Mount("/agents", agentHandler.Routes())

	// 4. Запуск сервера
	srv := &http.Server{
		Addr:         ":8000",
		Handler:      r,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	log.Printf("Console API started on %s", srv.Addr)
	log.Fatal(srv.ListenAndServe())
}
