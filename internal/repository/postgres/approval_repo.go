package postgres

/*
Файл approval_repo.go содержит реализацию методов для механизма Human-in-the-loop (HITL, «человек в контуре»).
*/

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/xela07ax/spaceai-infra-prototype/internal/domain"
)

// GetApprovalByID получение деталей запроса для анализа.
func (r *AgentRepo) GetApprovalByID(ctx context.Context, id string) (*domain.ApprovalRequest, error) {
	query := `SELECT id, execution_id, agent_id, capability, payload, status, reviewer_id, comment, created_at, updated_at 
	          FROM approvals WHERE id = $1`

	row := r.pool.QueryRow(ctx, query, id)

	var app domain.ApprovalRequest
	var reviewerID, comment sql.NullString // Используем для обработки NULL из БД

	err := row.Scan(
		&app.ID,
		&app.ExecutionID,
		&app.AgentID,
		&app.Capability,
		&app.Payload,
		&app.Status,
		&reviewerID,
		&comment,
		&app.CreatedAt,
		&app.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("approval not found: %w", err)
		}
		return nil, fmt.Errorf("database error: %w", err)
	}

	// Маппим NULL значения в строки (если есть)
	if reviewerID.Valid {
		val := reviewerID.String
		app.ReviewerID = &val // Берем адрес
	}
	if comment.Valid {
		val := comment.String
		app.Comment = &val
	}

	return &app, nil
}

// FindApprovals фильтрация и выборка списка запросов (Decision Queue).
func (r *AgentRepo) FindApprovals(ctx context.Context, status domain.ApprovalStatus) ([]*domain.ApprovalRequest, error) {
	// Базовый запрос
	query := `SELECT id, execution_id, agent_id, capability, payload, status, reviewer_id, comment, created_at, updated_at 
              FROM approvals`

	var args []interface{}
	if status != "" {
		query += " WHERE status = $1"
		args = append(args, status)
	}

	query += " ORDER BY created_at DESC LIMIT 100"

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("postgres: failed to query approvals: %w", err)
	}
	defer rows.Close()

	// Инициализируем пустой слайс, чтобы в JSON был [] вместо null
	results := make([]*domain.ApprovalRequest, 0)

	for rows.Next() {
		var app domain.ApprovalRequest
		var reviewerID, comment sql.NullString

		err := rows.Scan(
			&app.ID, &app.ExecutionID, &app.AgentID, &app.Capability,
			&app.Payload, &app.Status, &reviewerID, &comment,
			&app.CreatedAt, &app.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("postgres: failed to scan approval: %w", err)
		}

		if reviewerID.Valid {
			val := reviewerID.String
			app.ReviewerID = &val
		}
		if comment.Valid {
			val := comment.String
			app.Comment = &val
		}

		results = append(results, &app)
	}

	return results, nil
}

// CreateApproval создает запись в таблице approvals для механизма Human-in-the-loop.
// Это позволяет операторам через Console API увидеть запрос, выполнение которого было приостановлено шлюзом UAG.
func (r *AgentRepo) CreateApproval(ctx context.Context, app *domain.ApprovalRequest) error {
	query := `INSERT INTO approvals (id, execution_id, agent_id, capability, payload, status) 
	          VALUES ($1, $2, $3, $4, $5, $6)`
	_, err := r.pool.Exec(ctx, query, app.ID, app.ExecutionID, app.AgentID, app.Capability, app.Payload, app.Status)
	if err != nil {
		return fmt.Errorf("postgres: failed to create approval request: %w", err)
	}
	return nil
}

// UpdateApprovalStatus атомарно обновляет статус заявки на подтверждение.
// Использует условие WHERE status = 'PENDING' для предотвращения Double Decision.
// Возвращает execution_id, который необходим для отправки сигнала в Redis.
func (r *AgentRepo) UpdateApprovalStatus(ctx context.Context, id, status, reviewerID, comment string) (string, error) {
	var executionID string
	// RETURNING позволяет нам получить execution_id за один проход,
	// не делая предварительный SELECT (экономия ресурсов и исключение Race Condition)
	query := `
		UPDATE approvals 
		SET status = $1, 
		    reviewer_id = $2, 
		    comment = $3, 
		    updated_at = NOW() 
		WHERE id = $4 AND status = 'PENDING'
		RETURNING execution_id`

	err := r.pool.QueryRow(ctx, query, status, reviewerID, comment, id).Scan(&executionID)
	if err != nil {
		if err == pgx.ErrNoRows {
			// Если строк не найдено, значит либо ID неверный,
			// либо (что чаще) решение по этой заявке уже было принято ранее
			return "", fmt.Errorf("approval request not found or already processed (id: %s)", id)
		}
		return "", fmt.Errorf("postgres: failed to update approval status: %w", err)
	}
	return executionID, nil
}

// GetQuarantineAgents возвращает список ID всех агентов, находящихся в статусе карантина.
// Используется для инициализации L1 (RAM) кэша QuarantineManager при старте шлюза.
func (r *AgentRepo) GetQuarantineAgents(ctx context.Context) ([]string, error) {
	// Выбираем только ID, чтобы минимизировать трафик между БД и приложением
	query := `SELECT id FROM agents WHERE status = 'quarantine'`

	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("postgres: failed to fetch quarantine agents: %w", err)
	}
	defer rows.Close()

	// Инициализируем слайс, чтобы избежать возврата nil
	ids := make([]string, 0)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("postgres: scan agent id error: %w", err)
		}
		ids = append(ids, id)
	}

	// Проверка на ошибки итерации (стандарт качества pgx)
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres: rows iteration error: %w", err)
	}

	return ids, nil
}
