package domain

import "time"

type Policy struct {
	ID                 uuid.UUID
	AgentID            uuid.UUID
	Capability         string           // e.g. "sap.invoice.create"
	Constraints        jsonb.RawMessage // Лимиты: {"max_amount": 1000, "currency": "USD"}
	IsApprovalRequired bool             // Флаг Human-in-the-loop
	CreatedAt          time.Time
}
