package postgres

import (
	"context"

	"github.com/xela07ax/spaceai-infra-prototype/internal/domain"
)

func (r *AgentRepo) GetUnifiedDashboard(ctx context.Context) (*domain.UnifiedDashboard, error) {
	d := &domain.UnifiedDashboard{}

	// 1. Сбор Инцидентов и Активности Агентов
	err := r.pool.QueryRow(ctx, `
		SELECT 
			COUNT(*) FILTER (WHERE status = 'active'),
			COUNT(*) FILTER (WHERE status = 'blocked'),
			COUNT(*) FILTER (WHERE status = 'quarantine')
		FROM agents`).Scan(&d.Activity.ActiveAgents, &d.Incidents.BlockedAgents, &d.Risks.QuarantineRequests)
	if err != nil {
		return nil, err
	}

	// 2. Сбор метрик из Audit Logs за последние 60 минут
	// Мы используем PERCENTILE_CONT для расчета честного P95 Latency
	err = r.pool.QueryRow(ctx, `
		SELECT 
			COUNT(*),
			COUNT(*) FILTER (WHERE status = 'SYSTEM_ERROR'),
			COALESCE(PERCENTILE_CONT(0.95) WITHIN GROUP (ORDER BY duration_ms), 0)
		FROM audit_logs 
		WHERE timestamp > NOW() - INTERVAL '60 minutes'`).Scan(
		&d.Activity.TotalRequests,
		&d.Incidents.SystemErrors,
		&d.Quality.P95Latency,
	)

	// RPS = Всего запросов за час / 3600
	d.Activity.RPS = float64(d.Activity.TotalRequests) / 3600

	return d, err
}
