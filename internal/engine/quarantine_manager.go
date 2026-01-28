package engine

import (
	"github.com/redis/go-redis/v9"

	"context"
	"sync"
)

type QuarantineManager struct {
	mu               sync.RWMutex
	quarantineAgents map[string]struct{}
	rdb              *redis.Client
}

func NewQuarantineManager(rdb *redis.Client) *QuarantineManager {
	return &QuarantineManager{
		quarantineAgents: make(map[string]struct{}),
		rdb:              rdb,
	}
}

// Init загружает состояние всех "песочных" агентов при старте UAG
func (m *QuarantineManager) Init(ctx context.Context) error {
	agents, err := m.rdb.SMembers(ctx, "devit:agents:sandbox_set").Result()
	if err != nil {
		return err
	}

	m.mu.Lock()
	for _, id := range agents {
		m.quarantineAgents[id] = struct{}{}
	}
	m.mu.Unlock()
	return nil
}

// StartListener подписывается на изменения режима Sandbox в реальном времени
func (m *QuarantineManager) StartListener(ctx context.Context) {
	pubsub := m.rdb.Subscribe(ctx, "devit:agents:sandbox-signal")
	ch := pubsub.Channel()

	for msg := range ch {
		// Ожидаем формат сообщения "agentID:on" или "agentID:off"
		m.processSignal(msg.Payload)
	}
}

func (m *QuarantineManager) processSignal(payload string) {
	// Простая логика парсинга сигнала
	// В проде лучше использовать JSON или Protobuf
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(payload) > 3 && payload[len(payload)-3:] == ":on" {
		id := payload[:len(payload)-3]
		m.quarantineAgents[id] = struct{}{}
	} else if len(payload) > 4 && payload[len(payload)-4:] == ":off" {
		id := payload[:len(payload)-4]
		delete(m.quarantineAgents, id)
	}
}

// IsQuarantined — Ручной контроль при подозрении
func (m *QuarantineManager) IsQuarantined(agentID string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, ok := m.quarantineAgents[agentID]
	return ok
}
