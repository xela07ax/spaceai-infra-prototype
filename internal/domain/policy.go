package domain

import (
	"encoding/json"
	"time"
)

// PolicyEffect определяет, что делать с запросом
type PolicyEffect string

const (
	EffectAllow PolicyEffect = "ALLOW" // Разрешить (Live)
	EffectDeny  PolicyEffect = "DENY"  // Заблокировать

	// EffectSandbox позволяет «учителю» (Teacher) проверять гипотезы агента, фиксируя результаты в аудите для последующего обучения.
	EffectSandbox PolicyEffect = "SANDBOX" // Выполнить в песочнице

	// EffectQuarantine флаг Human-in-the-loop - Вместо жесткого переключателя «вкл/выкл», шлюз динамически определяет риск операции
	EffectQuarantine PolicyEffect = "QUARANTINE" // Требовать ручного подтверждения (HITL)
)

// Policy Архитектурный контур Security + Teacher + Connectors, представляет собой правило безопасности для Capability
type Policy struct {
	ID           string       `json:"id"`
	AgentID      string       `json:"agent_id"`      // "*" для всех агентов
	CapabilityID string       `json:"capability_id"` // Какое действие регулируем e.g. "sap.invoice.create"
	Effect       PolicyEffect `json:"effect"`

	// Ограничения (например, лимит суммы или список разрешенных IP)
	Conditions json.RawMessage `json:"conditions,omitempty"` // Лимиты: {"max_amount": 1000, "currency": "USD"}
	// позволяет ИБ-команде писать сложные правила (например, "только для транзакций до $100"), не меняя структуру БД.

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Decide — метод-интерпретатор. Гарантирует возврат валидного эффекта,
// даже если объект политики не проинициализирован (Zero Trust).
func (p *Policy) Decide() PolicyEffect {
	// Если политика nil (объект не найден в кэше) — жесткий запрет
	if p == nil {
		return EffectDeny
	}

	// Если эффект не задан или некорректен — карантин для безопасности (или Deny)
	if p.Effect == "" {
		return EffectDeny
	}

	return p.Effect
}
