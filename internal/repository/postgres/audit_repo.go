package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/xela07ax/spaceai-infra-prototype/internal/audit"
	"log"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib" // Драйвер Postgres
)

type AuditRepo struct {
	db *sql.DB
}

func NewAuditRepo(connString string) *AuditRepo {
	db, err := sql.Open("pgx", connString)
	if err != nil {
		// В main мы проверим соединение через Ping
		log.Fatal(err)
	}
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(25)
	db.SetConnMaxLifetime(5 * time.Minute)
	return &AuditRepo{db: db}
}

func (r *AuditRepo) WriteBatch(ctx context.Context, events []audit.AuditEvent) error {
	if len(events) == 0 {
		return nil
	}

	// Количество колонок в таблице audit_logs
	numFields := 10
	placeholderStr := ""
	vals := make([]interface{}, 0, len(events)*numFields)

	// Динамически строим запрос для пакетной вставки
	for i, e := range events {
		p := i * numFields
		placeholderStr += fmt.Sprintf("($%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d),",
			p+1, p+2, p+3, p+4, p+5, p+6, p+7, p+8, p+9, p+10)

		payload, _ := json.Marshal(e.Payload)
		resp, _ := json.Marshal(e.Response)

		vals = append(vals,
			e.ID, e.TraceID, e.AgentID, e.CapabilityID,
			payload, e.Mode, e.Status, resp, e.DurationMs, e.Timestamp,
		)
	}

	// Убираем лишнюю запятую в конце
	query := fmt.Sprintf(
		"INSERT INTO audit_logs (id, trace_id, agent_id, capability_id, payload, mode, status, response, duration_ms, timestamp) VALUES %s",
		strings.TrimSuffix(placeholderStr, ","),
	)

	_, err := r.db.ExecContext(ctx, query, vals...)
	return err
}
