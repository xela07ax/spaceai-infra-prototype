package engine

import (
	"github.com/redis/go-redis/v9"

	"context"
	"sync"
)

type SandboxManager struct {
	mu            sync.RWMutex
	sandboxAgents map[string]struct{}
	rdb           *redis.Client
}

func NewSandboxManager(rdb *redis.Client) *SandboxManager {
	return &SandboxManager{
		sandboxAgents: make(map[string]struct{}),
		rdb:           rdb,
	}
}

// Init загружает состояние всех "песочных" агентов при старте UAG
func (m *SandboxManager) Init(ctx context.Context) error {
	agents, err := m.rdb.SMembers(ctx, "devit:agents:sandbox_set").Result()
	if err != nil {
		return err
	}

	m.mu.Lock()
	for _, id := range agents {
		m.sandboxAgents[id] = struct{}{}
	}
	m.mu.Unlock()
	return nil
}

// StartListener подписывается на изменения режима Sandbox в реальном времени
func (m *SandboxManager) StartListener(ctx context.Context) {
	pubsub := m.rdb.Subscribe(ctx, "devit:agents:sandbox-signal")
	ch := pubsub.Channel()

	for msg := range ch {
		// Ожидаем формат сообщения "agentID:on" или "agentID:off"
		m.processSignal(msg.Payload)
	}
}

func (m *SandboxManager) processSignal(payload string) {
	// Простая логика парсинга сигнала
	// В проде лучше использовать JSON или Protobuf
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(payload) > 3 && payload[len(payload)-3:] == ":on" {
		id := payload[:len(payload)-3]
		m.sandboxAgents[id] = struct{}{}
	} else if len(payload) > 4 && payload[len(payload)-4:] == ":off" {
		id := payload[:len(payload)-4]
		delete(m.sandboxAgents, id)
	}
}

// IsSandbox — максимально быстрый метод для проверки в Hot Path
func (m *SandboxManager) IsSandbox(agentID string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, ok := m.sandboxAgents[agentID]
	return ok
}
