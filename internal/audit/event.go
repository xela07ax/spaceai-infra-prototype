package audit

import "time"

type AuditEvent struct {
	ID           string                 `json:"id"`            // UUID события
	TraceID      string                 `json:"trace_id"`      // Сквозной ID запроса
	AgentID      string                 `json:"agent_id"`      // Кто делал
	CapabilityID string                 `json:"capability_id"` // Что хотел сделать
	Payload      map[string]interface{} `json:"payload"`       // С какими данными

	// Контекст исполнения
	Mode     string `json:"mode"`      // "LIVE" или "SANDBOX"
	PolicyID string `json:"policy_id"` // Какая политика разрешила/перехватила

	// Результат
	Status     string      `json:"status"`   // "SUCCESS", "FAILED", "INTERCEPTED"
	Response   interface{} `json:"response"` // Что вернули агенту
	Timestamp  time.Time   `json:"timestamp"`
	DurationMs int64       `json:"duration_ms"` // Время обработки
	Error      string      `json:"error"`
}
