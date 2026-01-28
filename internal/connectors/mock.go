package connectors

import (
	"context"
	"fmt"
	"math/rand/v2" // Используем v2 для Go 1.25
	"time"
)

type MockSystemsConnector struct{}

func (c *MockSystemsConnector) Call(ctx context.Context, capID string, payload []byte) ([]byte, error) {
	// В v2 используется rand.IntN (с большой N)
	// Имитируем задержку 50-300мс
	latency := time.Duration(50+rand.IntN(250)) * time.Millisecond

	select {
	case <-time.After(latency):
		// Имитация работы
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	if capID == "unstable.service" {
		return nil, fmt.Errorf("service internal error")
	}

	switch capID {
	case "jira.ticket.delete":
		return []byte(`{"status": "deleted", "integration": "jira", "id": "DEV-101"}`), nil
	case "slack.message.send":
		return []byte(`{"status": "sent", "integration": "slack", "channel": "#general"}`), nil

	// Коннектор к БД
	case "db.query.execute":
		// Имитируем чтение данных
		return []byte(`{"status": "success", "rows_affected": 0, "data": [{"id": 1, "balance": 5000}]}`), nil

	// Коннектор к CRM (например, Salesforce)
	case "crm.lead.create":
		return []byte(`{"status": "created", "lead_id": "L-990"}`), nil

	default:
		return nil, fmt.Errorf("capability %s not supported by connector", capID)
	}
}
