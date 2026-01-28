package engine

import (
	"github.com/redis/go-redis/v9"

	"context"
	"sync"
)

type KillSwitchManager struct {
	mu            sync.RWMutex
	blockedAgents map[string]struct{}
	rdb           *redis.Client
}

func NewKillSwitchManager(rdb *redis.Client) *KillSwitchManager {
	return &KillSwitchManager{
		blockedAgents: make(map[string]struct{}),
		rdb:           rdb,
	}
}

// Init загружает текущее состояние блокировок при старте сервиса
func (m *KillSwitchManager) Init(ctx context.Context) error {
	agents, err := m.rdb.SMembers(ctx, "devit:agents:blocked_set").Result()
	if err != nil {
		return err
	}

	m.mu.Lock()
	for _, id := range agents {
		m.blockedAgents[id] = struct{}{}
	}
	m.mu.Unlock()
	return nil
}
