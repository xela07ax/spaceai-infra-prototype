package engine

/*
Файл killswitch_manager.go реализует механизм мгновенной блокировки агентов (Kill-Switch).
Является критическим компонентом Control Plane, обеспечивающим защиту инфраструктуры
в режиме реального времени.

Ключевые особенности реализации:
- Двухуровневое состояние: хранение активных блокировок в локальной RAM (L1)
  и синхронизация через Redis Set (L2).
- Механизм Warm-up: при старте инстанса выполняется "холодная загрузка" данных
  из PostgreSQL с использованием распределенного замка (SetNX) для предотвращения
  Thundering Herd и дублирования трафика.
- Event-Driven Reaction: подписка на Redis Pub/Sub гарантирует доставку сигнала
  о блокировке до всех инстансов шлюза за миллисекунды.
- Безопасность инициализации: строгий порядок запуска (Subscribe -> Init)
  исключает "слепые зоны" и потерю обновлений в момент старта сервиса.
*/

import (
	"fmt"

	"github.com/redis/go-redis/v9"
	"github.com/xela07ax/spaceai-infra-prototype/internal/infra"
	"go.uber.org/zap"

	"context"
	"sync"
)

type BlockListLoader interface {
	GetBlockedIDs(ctx context.Context) ([]string, error)
}

type KillSwitchManager struct {
	repo         BlockListLoader
	rdb          *redis.Client
	logger       *zap.Logger
	mu           sync.RWMutex
	blockedCache map[string]bool // Кэш заблокированных агентов (L1)
}

func NewKillSwitchManager(rdb *redis.Client, repo BlockListLoader, logger *zap.Logger) *KillSwitchManager {
	return &KillSwitchManager{
		blockedCache: make(map[string]bool),
		rdb:          rdb,
		repo:         repo,
		logger:       logger.With(zap.String("mod", "ksm")),
	}
}

// Init загружает текущее состояние блокировок при старте сервиса
func (ksm *KillSwitchManager) Init(ctx context.Context) error {
	ids, err := ksm.repo.GetBlockedIDs(ctx)
	if err != nil {
		return fmt.Errorf("failed to fetch blocked agents from DB: %w", err)
	}

	return WarmupState(ctx, ksm.rdb, ksm.logger, ids, infra.RedisKeyBlockedAgents, infra.RedisKeyLockBlocked, func(items []string) {
		ksm.mu.Lock()
		defer ksm.mu.Unlock()
		for _, id := range items {
			ksm.blockedCache[id] = true
		}
	})
}

func (ksm *KillSwitchManager) StartListener(ctx context.Context) {
	ListenStateResilient(ctx, ksm.rdb, ksm.logger, infra.RedisChanKillSwitch,
		func() error { return ksm.Init(ctx) }, // Переподключение
		func(id string, status bool) { // Обработка сообщения
			ksm.mu.Lock()
			defer ksm.mu.Unlock()
			if status {
				ksm.blockedCache[id] = true
			} else {
				delete(ksm.blockedCache, id)
			}
		},
	)
}

// MarkAsBlocked — внутренний метод для обновления мапы
func (ksm *KillSwitchManager) MarkAsBlocked(agentID string) {
	ksm.mu.Lock()
	defer ksm.mu.Unlock()
	ksm.blockedCache[agentID] = true
}

func (ksm *KillSwitchManager) IsBlocked(agentID string) bool {
	ksm.mu.RLock()
	defer ksm.mu.RUnlock()
	return ksm.blockedCache[agentID] // Просто и элегантно
}
