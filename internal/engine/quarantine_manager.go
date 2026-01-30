package engine

import (
	"context"
	"fmt"
	"sync"

	"github.com/redis/go-redis/v9"
	"github.com/xela07ax/spaceai-infra-prototype/internal/infra"
	"go.uber.org/zap"
)

type QuarantineProvider interface {
	GetQuarantineAgents(ctx context.Context) ([]string, error)
}

type QuarantineManager struct {
	repo           QuarantineProvider
	rdb            *redis.Client
	logger         *zap.Logger
	mu             sync.RWMutex
	quarantineCash map[string]bool
}

func NewQuarantineManager(rdb *redis.Client, repo QuarantineProvider, logger *zap.Logger) *QuarantineManager {
	return &QuarantineManager{
		quarantineCash: make(map[string]bool),
		repo:           repo,
		rdb:            rdb,
	}
}

func (qm *QuarantineManager) Init(ctx context.Context) error {
	ids, err := qm.repo.GetQuarantineAgents(ctx)
	if err != nil {
		return fmt.Errorf("failed to fetch quarantine agents from DB: %w", err)
	}

	return WarmupState(ctx, qm.rdb, qm.logger, ids, infra.RedisKeyQuarantineAgents, infra.RedisKeyLockBlockedQuarantine, func(items []string) {
		qm.mu.Lock()
		defer qm.mu.Unlock()
		for _, id := range items {
			qm.quarantineCash[id] = true
		}
	})
}

func (qm *QuarantineManager) StartListener(ctx context.Context) {
	ListenStateResilient(ctx, qm.rdb, qm.logger, infra.RedisChanQuarantine,
		func() error { return qm.Init(ctx) }, // Переподключение
		func(id string, status bool) { // Обработка сообщения
			qm.mu.Lock()
			defer qm.mu.Unlock()
			if status {
				qm.quarantineCash[id] = true
			} else {
				delete(qm.quarantineCash, id)
			}
		},
	)
}

// IsQuarantined — Ручной контроль при подозрении
func (qm *QuarantineManager) IsQuarantined(agentID string) bool {
	qm.mu.RLock()
	defer qm.mu.RUnlock()
	_, ok := qm.quarantineCash[agentID]
	return ok
}
