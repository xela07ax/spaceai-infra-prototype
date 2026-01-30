package postgres

/*
Файл agent_repo.go является точкой входа в слой инфраструктуры PostgreSQL.
Здесь реализована инициализация высокопроизводительного пула соединений pgxpool
и методы управления состоянием агентов:
- IsBlocked/MarkAsBlocked: проверка и изменение статуса блокировки (Kill-Switch).
- GetAgent/UpdateStatus: базовые операции управления жизненным циклом агентов.

Этот файл служит базой для расширения репозитория методами аудита, политик и подтверждений,
расположенными в соседних файлах пакета.
*/

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/xela07ax/spaceai-infra-prototype/internal/audit"
	"github.com/xela07ax/spaceai-infra-prototype/internal/domain"
	"github.com/xela07ax/spaceai-infra-prototype/internal/infra"
)

// AgentRepo Основная структура, инициализация пула, методы для Agents
type AgentRepo struct {
	pool *pgxpool.Pool
}

// NewAgentRepo — инициализация пула с настройкой MaxConns и жизненного цикла соединений.
func NewAgentRepo(ctx context.Context, cfg *infra.Config) *AgentRepo {
	// Парсим строку в объект конфига
	config, err := pgxpool.ParseConfig(cfg.Database.URL)
	if err != nil {
		log.Fatalf("Unable to parse DB_URL: %v", err)
	}

	// Настраиваем пул для Highload
	config.MaxConns = cfg.Database.MaxConns   // Максимум соединений
	config.MinConns = cfg.Database.MinConns   // Минимум всегда готовых
	config.MaxConnLifetime = 30 * time.Minute // Время жизни соединения
	config.MaxConnIdleTime = 5 * time.Minute  // Время простоя

	// Создаем пул
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		log.Fatalf("Unable to create connection pool: %v", err)
	}

	return &AgentRepo{pool: pool}
}

func (r *AgentRepo) Close() { r.pool.Close() }

// Ping проверяет доступность базы при старте
func (r *AgentRepo) Ping(ctx context.Context) error { return r.pool.Ping(ctx) }

// UpdateAgentStatus меняет основной статус (например, для Kill-switch)
func (r *AgentRepo) UpdateAgentStatus(ctx context.Context, agentID string, status string) error {
	query := `UPDATE agents SET status = $1, updated_at = NOW() WHERE id = $2`

	result, err := r.pool.Exec(ctx, query, status, agentID)
	if err != nil {
		return fmt.Errorf("postgres: failed to update status: %w", err)
	}

	rows := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("postgres: agent %s not found", agentID)
	}
	return nil
}

// SetAgentSandbox включает/выключает песочницу
func (r *AgentRepo) SetAgentSandbox(ctx context.Context, agentID string, enabled bool) error {
	query := `UPDATE agents SET is_sandbox = $1, updated_at = NOW() WHERE id = $2`

	_, err := r.pool.Exec(ctx, query, enabled, agentID)
	return err
}

func (r *AgentRepo) GetSandboxAgents(ctx context.Context) ([]string, error) {
	// Выбираем агентов, у которых в поле status или в отдельной таблице указан sandbox
	query := `SELECT id FROM agents WHERE is_sandbox = true`

	var p []string
	err := r.pool.QueryRow(ctx, query).Scan(&p)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return p, nil
}

// GetBlockedIDs возвращает список ID всех агентов со статусом 'blocked'.
// Используется для "прогрева" кэша Kill-Switch при старте системы.
func (r *AgentRepo) GetBlockedIDs(ctx context.Context) ([]string, error) {
	// Мы выбираем только ID, так как для L1/L2 кэша нам не нужны полные данные агента
	query := `SELECT id FROM agents WHERE status = 'blocked'`

	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("postgres: failed to fetch blocked IDs: %w", err)
	}
	defer rows.Close()

	// Инициализируем пустой слайс, чтобы избежать возврата nil
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("postgres: failed to scan blocked ID: %w", err)
		}
		ids = append(ids, id)
	}

	// Проверяем ошибку, которая могла возникнуть во время итерации
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres: rows iteration error: %w", err)
	}

	return ids, nil
}

func (r *AgentRepo) GetGlobalStats(ctx context.Context) (*domain.GlobalStats, error) {
	stats := &domain.GlobalStats{
		TopCapabilities: make(map[string]int64),
	}

	// 1. Общая статистика и блокировки за 24 часа
	query := `
		SELECT 
			count(*) as total,
			count(*) FILTER (WHERE status = 'BLOCKED' OR status = 'DENIED') as blocked
		FROM audit_logs 
		WHERE timestamp > NOW() - INTERVAL '24 hours'`

	err := r.pool.QueryRow(ctx, query).Scan(&stats.TotalActions, &stats.BlockedActions)
	if err != nil {
		return nil, err
	}

	if stats.TotalActions > 0 {
		stats.RiskRatio = float64(stats.BlockedActions) / float64(stats.TotalActions)
	}

	// 2. Топ-5 популярных способностей (capabilities)
	rows, err := r.pool.Query(ctx, `
		SELECT capability_id, count(*) 
		FROM audit_logs 
		WHERE timestamp > NOW() - INTERVAL '24 hours'
		GROUP BY capability_id 
		ORDER BY count(*) DESC LIMIT 5`)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var capID string
			var count int64
			err := rows.Scan(&capID, &count)
			if err != nil {
				return nil, err
			}
			stats.TopCapabilities[capID] = count
		}
	}

	return stats, nil
}
func (r *AgentRepo) FetchLogs(ctx context.Context, agentID, capID string) ([]audit.AuditEvent, error) {
	// $1 = '' OR agent_id = $1 — это эффективный способ сделать фильтры опциональными
	query := `
		SELECT id, agent_id, capability_id, status, duration_ms, timestamp 
		FROM audit_logs 
		WHERE ($1 = '' OR agent_id = $1) 
		  AND ($2 = '' OR capability_id = $2)
		ORDER BY timestamp DESC 
		LIMIT 100`

	rows, err := r.pool.Query(ctx, query, agentID, capID)
	if err != nil {
		return nil, fmt.Errorf("postgres: query failed: %w", err)
	}
	defer rows.Close()

	// Инициализируем пустой слайс (не nil), чтобы фронтенд получил [] вместо null
	logs := make([]audit.AuditEvent, 0)

	for rows.Next() {
		var log audit.AuditEvent
		err := rows.Scan(
			&log.ID,
			&log.AgentID,
			&log.CapabilityID,
			&log.Status,
			&log.DurationMs,
			&log.Timestamp,
		)
		if err != nil {
			return nil, fmt.Errorf("postgres: scan error: %w", err)
		}
		logs = append(logs, log)
	}

	// Проверяем на ошибки, возникшие во время итерации (например, обрыв связи)
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres: rows iteration error: %w", err)
	}

	return logs, nil
}

func (r *AgentRepo) GetAgent(ctx context.Context, id string) (*domain.Agent, error) {
	query := `
		SELECT id, name, status, is_sandbox, scopes, last_activity, created_at, updated_at, metadata 
		FROM agents 
		WHERE id = $1`

	var a domain.Agent
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&a.ID, &a.Name, &a.Status, &a.IsSandbox, &a.Scopes,
		&a.LastActivity, &a.CreatedAt, &a.UpdatedAt, &a.Metadata,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil // Возвращаем nil без ошибки, чтобы хендлер выдал 404
		}
		return nil, err
	}
	return &a, nil
}

func (r *AgentRepo) ListAgents(ctx context.Context) ([]*domain.Agent, error) {
	// Выбираем основные поля для списка.
	// Metadata и Scopes берем, чтобы фронт мог сразу их показать без доп. запросов.
	query := `
		SELECT id, name, status, is_sandbox, last_activity, created_at, metadata 
		FROM agents 
		ORDER BY last_activity DESC NULLS LAST`

	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var agents []*domain.Agent
	for rows.Next() {
		a := &domain.Agent{}
		err := rows.Scan(
			&a.ID,
			&a.Name,
			&a.Status,
			&a.IsSandbox,
			&a.LastActivity,
			&a.CreatedAt,
			&a.Metadata,
		)
		if err != nil {
			return nil, err
		}
		agents = append(agents, a)
	}

	return agents, nil
}
