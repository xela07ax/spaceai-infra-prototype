package postgres

/*
Файл policy_repo.go отвечает за хранение и поставку правил безопасности (Policies).
Данный слой обеспечивает отделение долговременного хранения правил в PostgreSQL
от их мгновенной проверки в оперативной памяти шлюза.
*/

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/xela07ax/spaceai-infra-prototype/internal/domain"
)

func (r *AgentRepo) GetPolicyByID(ctx context.Context, id string) (*domain.Policy, error) {
	query := `
		SELECT id, agent_id, capability_id, effect, conditions 
		FROM policies 
		WHERE id = $1`

	p := &domain.Policy{}
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&p.ID,
		&p.AgentID,
		&p.CapabilityID,
		&p.Effect,
		&p.Conditions,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil // Возвращаем nil для 404 в хендлере
		}
		return nil, err
	}
	return p, nil
}

// GetAllPolicies выполняет "холодную загрузку" всего набора активных политик при старте.
func (r *AgentRepo) GetAllPolicies(ctx context.Context) ([]domain.Policy, error) {
	query := `SELECT id, agent_id, capability_id, effect, conditions FROM policies`

	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []domain.Policy
	for rows.Next() {
		var p domain.Policy
		if err := rows.Scan(&p.ID, &p.AgentID, &p.CapabilityID, &p.Effect, &p.Conditions); err != nil {
			return nil, err
		}
		results = append(results, p)
	}
	return results, nil
}

// todo убрали вызов из за инфорсера
// GetDecision точечное получение правил для специфичных проверок. для связки Агент + Capability.
// - Логика Wildcards: поддерживает выборку правил с учетом иерархии (конкретный агент vs '*').
func (r *AgentRepo) GetDecision(ctx context.Context, agentID, capID string) (*domain.Policy, error) {
	// Ищем специфичную политику для агента или общую ("*").
	query := `
		SELECT id, agent_id, capability_id, effect,  conditions, created_at, updated_at
		FROM policies
		WHERE (agent_id = $1 OR agent_id = '*') AND capability_id = $2
		ORDER BY (agent_id != '*') DESC -- Сначала специфичные политики агента
		LIMIT 1`
	// Wildcard Matching (*): Мы реализовали простую иерархию (сначала агент, потом глобал).
	// Это позволяет ИБ-команде одной кнопкой закрыть доступ к db.delete для всех, но разрешить его конкретному admin-agent.

	var p domain.Policy
	err := r.pool.QueryRow(ctx, query, agentID, capID).Scan(
		&p.ID, &p.AgentID, &p.CapabilityID, &p.Effect, &p.Conditions, &p.CreatedAt, &p.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil // Политика не найдена (можно трактовать как Deny по умолчанию)
		}
		return nil, err // Политика не найдена
	}
	return &p, nil
}

// CreatePolicy создает новую запись.
// Позволяет задавать agent_id = '*' для глобальных правил.
func (r *AgentRepo) CreatePolicy(ctx context.Context, p *domain.Policy) error {
	query := `
		INSERT INTO policies (id, agent_id, capability_id, effect, conditions)
		VALUES (gen_random_uuid(), $1, $2, $3, $4)`

	_, err := r.pool.Exec(ctx, query, p.AgentID, p.CapabilityID, p.Effect, p.Conditions)
	if err != nil {
		return fmt.Errorf("postgres: failed to create policy: %w", err)
	}
	return nil
}

// UpdatePolicy обновляет условия или эффект существующей политики.
func (r *AgentRepo) UpdatePolicy(ctx context.Context, p *domain.Policy) error {
	query := `
		UPDATE policies 
		SET effect = $1, conditions = $2 
		WHERE id = $3`

	ct, err := r.pool.Exec(ctx, query, p.Effect, p.Conditions, p.ID)
	if err != nil {
		return fmt.Errorf("postgres: failed to update policy: %w", err)
	}

	if ct.RowsAffected() == 0 {
		return fmt.Errorf("postgres: policy not found")
	}
	return nil
}

// DeletePolicy удаляет политику по ID.
func (r *AgentRepo) DeletePolicy(ctx context.Context, id string) error {
	query := `DELETE FROM policies WHERE id = $1`

	ct, err := r.pool.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("postgres: failed to delete policy: %w", err)
	}

	if ct.RowsAffected() == 0 {
		return fmt.Errorf("postgres: policy not found")
	}
	return nil
}
