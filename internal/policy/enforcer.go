package policy

import "context"

type Enforcer interface {
	Authorize(ctx context.Context, agentID, capID string, data []byte) (bool, error)
}

// Mock для демонстрации
type MemoEnforcer struct{}

func (e *MemoEnforcer) Authorize(ctx context.Context, agentID, capID string, data []byte) (bool, error) {
	// В MVP тут может быть мапа: AgentID -> []Capabilities
	// В будущем - вызов Sidecar-контейнера с Open Policy Agent
	return true, nil
}
