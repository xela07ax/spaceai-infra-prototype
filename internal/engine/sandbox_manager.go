package engine

import (
	"fmt"

	"github.com/redis/go-redis/v9"
	"github.com/xela07ax/spaceai-infra-prototype/internal/infra"
	"go.uber.org/zap"

	"context"
	"sync"
)

type SandboxProvider interface {
	GetSandboxAgents(ctx context.Context) ([]string, error)
}

type SandboxManager struct {
	repo       SandboxProvider
	rdb        *redis.Client
	logger     *zap.Logger
	mu         sync.RWMutex
	agentsCash map[string]bool
}

func NewSandboxManager(rdb *redis.Client, repo SandboxProvider, logger *zap.Logger) *SandboxManager {
	return &SandboxManager{
		agentsCash: make(map[string]bool),
		repo:       repo,
		rdb:        rdb,
		logger:     logger.With(zap.String("mod", "sandbox")),
	}
}

// Init загружает состояние всех "песочных" агентов при старте UAG
func (sm *SandboxManager) Init(ctx context.Context) error {
	ids, err := sm.repo.GetSandboxAgents(ctx)
	if err != nil {
		return fmt.Errorf("failed to fetch sandbox agents from DB: %w", err)
	}

	return WarmupState(ctx, sm.rdb, sm.logger, ids, infra.RedisKeySandboxAgents, infra.RedisKeyLockBlockedSandbox, func(items []string) {
		sm.mu.Lock()
		defer sm.mu.Unlock()
		for _, id := range items {
			sm.agentsCash[id] = true
		}
	})
}

// StartListener подписывается на изменения режима Sandbox в реальном времени
func (sm *SandboxManager) StartListener(ctx context.Context) {
	ListenStateResilient(ctx, sm.rdb, sm.logger, infra.RedisChanSandbox,
		func() error { return sm.Init(ctx) }, // Переподключение
		func(id string, status bool) { // Обработка сообщения
			sm.mu.Lock()
			defer sm.mu.Unlock()
			if status {
				sm.agentsCash[id] = true
			} else {
				delete(sm.agentsCash, id)
			}
		},
	)
}

func (sm *SandboxManager) processSignal(payload string) {
	// Простая логика парсинга сигнала
	// В проде лучше использовать JSON или Protobuf
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if len(payload) > 3 && payload[len(payload)-3:] == ":on" {
		id := payload[:len(payload)-3]
		sm.agentsCash[id] = true
	} else if len(payload) > 4 && payload[len(payload)-4:] == ":off" {
		id := payload[:len(payload)-4]
		delete(sm.agentsCash, id)
	}
}

// IsSandbox — максимально быстрый метод для проверки в Hot Path
func (sm *SandboxManager) IsSandbox(agentID string) bool {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.agentsCash[agentID]
}
