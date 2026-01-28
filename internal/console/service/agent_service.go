package service

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"
)

// AgentRepository Сначала определим, что мы ждем от базы. (чтобы можно было мокать в тестах)
type AgentRepository interface {
	UpdateStatus(ctx context.Context, id string, status string) error
	UpdateSandboxStatus(ctx context.Context, id string, enabled bool) error
}

type AgentService struct {
	db  AgentRepository
	rdb *redis.Client
}

func NewAgentService(db AgentRepository, rdb *redis.Client) *AgentService {
	return &AgentService{
		db:  db,
		rdb: rdb,
	}
}

func (s *AgentService) BlockAgent(ctx context.Context, agentID string) error {
	// 1. Сначала БД — это наш "источник правды"
	// Если БД упала, мы не идем дальше.
	if err := s.db.UpdateStatus(ctx, agentID, "blocked"); err != nil {
		return fmt.Errorf("database update failed: %w", err)
	}

	// 2. Затем Redis (State + Signal) через Pipeline
	// Мы НЕ делаем это в горутине, так как блокировка — критическое действие.
	// Админ должен быть уверен: если пришел ответ 204, агент УЖЕ заблокирован везде.
	pipe := s.rdb.Pipeline()

	// Сохраняем состояние (для новых UAG)
	pipe.SAdd(ctx, "devit:agents:blocked_set", agentID)
	// Шлем сигнал (для запущенных UAG)
	pipe.Publish(ctx, "devit:agents:kill-switch", agentID)

	if _, err := pipe.Exec(ctx); err != nil {
		// Log & Alert: В БД заблокирован, но в рантайме может продолжать работать!
		// Это требует немедленного внимания (Inconsistency)
		return fmt.Errorf("failed to sync block signal to redis: %w", err)
	}

	return nil
}

func (s *AgentService) SetSandboxMode(ctx context.Context, agentID string, enabled bool) error {
	// 1. Фиксируем в БД
	if err := s.db.UpdateSandboxStatus(ctx, agentID, enabled); err != nil {
		return err
	}

	// 2. Обновляем Runtime состояние в Redis
	// Используем Set для хранения списка "песочных" агентов
	var err error
	if enabled {
		err = s.rdb.SAdd(ctx, "devit:agents:sandbox_set", agentID).Err()
		s.rdb.Publish(ctx, "devit:agents:sandbox-signal", agentID+":on")
	} else {
		err = s.rdb.SRem(ctx, "devit:agents:sandbox_set", agentID).Err()
		s.rdb.Publish(ctx, "devit:agents:sandbox-signal", agentID+":off")
	}

	return err
}

func (s *AgentService) QuarantineAgent(ctx context.Context, agentID string) error {
	// 1. БД: статус 'quarantine'
	if err := s.db.UpdateStatus(ctx, agentID, "quarantine"); err != nil {
		return err
	}

	// 2. Redis: сигнализируем шлюзам
	pipe := s.rdb.Pipeline()
	pipe.SAdd(ctx, "devit:agents:quarantine_set", agentID)
	pipe.Publish(ctx, "devit:agents:quarantine-signal", agentID+":on")

	_, err := pipe.Exec(ctx)
	return err
}
