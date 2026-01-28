package engine

import (
	"context"
	"log"
)

// MarkAsBlocked — внутренний метод для обновления мапы
func (m *KillSwitchManager) MarkAsBlocked(agentID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.blockedAgents[agentID] = struct{}{}
}

// StartListener подписывается на Redis и обновляет состояние
func (m *KillSwitchManager) StartListener(ctx context.Context) {
	// Канал должен совпадать с тем, что в Console API
	pubsub := m.rdb.Subscribe(ctx, "devit:agents:kill-switch")

	// Используем defer для очистки ресурсов при выходе из горутины
	defer pubsub.Close()

	ch := pubsub.Channel()
	log.Println("UAG: Kill-switch listener started")

	for {
		select {
		case msg, ok := <-ch:
			if !ok {
				log.Println("UAG: Kill-switch channel closed")
				return
			}

			agentID := msg.Payload
			log.Printf("UAG: Received KILL signal for agent [%s]", agentID)

			// Обновляем локальный потокобезопасный кэш
			m.MarkAsBlocked(agentID)

		case <-ctx.Done():
			log.Println("UAG: Kill-switch listener stopping by context...")
			return
		}
	}
}
