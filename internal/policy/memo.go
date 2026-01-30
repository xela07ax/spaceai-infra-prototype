package policy

import (
	"context"
	"sync"

	"github.com/redis/go-redis/v9"
	"github.com/xela07ax/spaceai-infra-prototype/internal/domain"
	"go.uber.org/zap"
)

type PolicyRepository interface {
	GetAllPolicies(ctx context.Context) ([]domain.Policy, error)
}

// MemoEnforcer реализует интерфейс Enforcer, используя потокобезопасную мапу.
// Представляет In-memory cache политик. В распределенной системе он синхронизируется с БД,
// но в рантайме шлюз обращается только к памяти.
type MemoEnforcer struct {
	mu sync.RWMutex
	// Кэш: "agent_id:capability_id" -> Policy
	policies map[string]domain.Policy

	repo   PolicyRepository // Используется только для Refresh()
	rdb    *redis.Client
	logger *zap.Logger
}

func NewMemoEnforcer(repo PolicyRepository, rdb *redis.Client, logger *zap.Logger) *MemoEnforcer {
	return &MemoEnforcer{
		policies: make(map[string]domain.Policy),
		repo:     repo,
		rdb:      rdb,
		logger:   logger.Named("enforcer"),
	}
}

// GetPolicy политики для авторизации. Он работает только с RAM. Он не знает про Postgres. Это и есть наш "Hot Path"
// Нам мало знать «можно или нельзя». Нам нужно знать «как именно» (Live, Sandbox, Quarantine)
// Логика принятия решения (Decision Logic) находится в UAGCore, где есть доступ к KillSwitch, SandboxManager и Policy
func (e *MemoEnforcer) GetPolicy(agentID, capID string) domain.Policy {
	e.mu.RLock()
	defer e.mu.RUnlock()

	// 1. Сначала ищем персональную политику агента
	specificKey := agentID + ":" + capID
	if p, ok := e.policies[specificKey]; ok {
		return p
	}

	// 2. Если нет — ищем глобальную политику (wildcard) для всех агентов
	globalKey := "*:" + capID
	if p, ok := e.policies[globalKey]; ok {
		return p
	}

	// 3. Если ничего не нашли — возвращаем дефолтный запрет Default Deny (Zero Trust)
	return domain.Policy{Effect: domain.EffectDeny}
}

// Refresh он выполняет «холодную загрузку» для втономномности всех политик из PostgreSQL в память шлюза (при старте).
func (e *MemoEnforcer) Refresh(ctx context.Context) error {
	policiesDb, err := e.repo.GetAllPolicies(ctx)
	if err != nil {
		return err
	}

	newPolicies := make(map[string]domain.Policy)
	for _, p := range policiesDb {
		key := p.AgentID + ":" + p.CapabilityID
		newPolicies[key] = p
	}

	e.mu.Lock()
	e.policies = newPolicies
	e.mu.Unlock()

	e.logger.Info("policy cache refreshed", zap.Int("count", len(newPolicies)))
	return nil
}
