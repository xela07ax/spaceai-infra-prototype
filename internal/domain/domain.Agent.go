package domain

import "time"

type AgentStatus string

const (
	StatusActive     AgentStatus = "active"     // Полный доступ
	StatusBlocked    AgentStatus = "blocked"    // Kill-switch (блокировка)
	StatusQuarantine AgentStatus = "quarantine" // Требует HITL-подтверждения
	StatusSandbox    AgentStatus = "sandbox"    // Безопасный режим (Live-данные не меняются)
)

type Agent struct {
	ID        string      `json:"id"`         // UUID
	Name      string      `json:"name"`       // Человекочитаемое имя (например, "Jira-Helper-Bot")
	Status    AgentStatus `json:"status"`     // Текущее состояние в Control Plane
	IsSandbox bool        `json:"is_sandbox"` // Флаг режима песочницы
	Scopes    []string    `json:"scopes"`     // Список разрешенных Capability ID (для токена)

	// Метаданные для Observability
	LastActivity time.Time `json:"last_activity"` // Последний успешный запрос
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`

	// Дополнительные данные (версия, окружение и т.д.)
	Metadata map[string]interface{} `json:"metadata"`
}
