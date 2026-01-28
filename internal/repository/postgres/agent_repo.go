package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib" // Драйвер Postgres
)

type AgentRepo struct {
	db *sql.DB
}

// NewAgentRepo создает новый экземпляр репозитория
func NewAgentRepo(connString string) *AgentRepo {
	db, err := sql.Open("pgx", connString)
	if err != nil {
		// В main мы проверим соединение через Ping
		log.Fatal(err)
	}
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(25)
	db.SetConnMaxLifetime(5 * time.Minute)
	return &AgentRepo{db: db}
}

// UpdateStatus меняет основной статус (например, для Kill-switch)
func (r *AgentRepo) UpdateStatus(ctx context.Context, id string, status string) error {
	query := `UPDATE agents SET status = $1, updated_at = NOW() WHERE id = $2`

	result, err := r.db.ExecContext(ctx, query, status, id)
	if err != nil {
		return fmt.Errorf("postgres: failed to update status: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("postgres: agent %s not found", id)
	}
	return nil
}

// UpdateSandboxStatus включает/выключает песочницу
func (r *AgentRepo) UpdateSandboxStatus(ctx context.Context, id string, enabled bool) error {
	query := `UPDATE agents SET is_sandbox = $1, updated_at = NOW() WHERE id = $2`

	_, err := r.db.ExecContext(ctx, query, enabled, id)
	return err
}

// Ping проверяет доступность базы при старте
func (r *AgentRepo) Ping(ctx context.Context) error {
	return r.db.PingContext(ctx)
}
