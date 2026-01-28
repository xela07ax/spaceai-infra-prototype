package engine

import (
	"context"

	"github.com/redis/go-redis/v9"
)

func (m *KillSwitchManager) Listen(ctx context.Context, rdb *redis.Client) {
	pubsub := rdb.Subscribe(ctx, "agent_kill_switch")
	ch := pubsub.Channel()

	for msg := range ch {
		agentID := msg.Payload
		m.mu.Lock()
		m.blockedAgents[agentID] = struct{}{}
		m.mu.Unlock()

		// Опционально: здесь можно вызвать логику отмены всех
		// активных context.Context для этого agentID
	}
}

func (m *KillSwitchManager) IsBlocked(agentID string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, blocked := m.blockedAgents[agentID]
	return blocked
}
