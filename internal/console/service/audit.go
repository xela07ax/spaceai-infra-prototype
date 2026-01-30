package service

import (
	"context"
	"fmt"

	"github.com/xela07ax/spaceai-infra-prototype/internal/audit"
)

// AuditLogProvider описывает контракт для чтения данных аудита.
// Мы используем структуру AuditEvent из пакета audit, чтобы сохранить единую модель данных.
type AuditLogProvider interface {
	FetchLogs(ctx context.Context, agentID, capID string) ([]audit.AuditEvent, error)
}

type AuditService struct {
	repo AuditLogProvider
}

func NewAuditService(repo AuditLogProvider) *AuditService {
	return &AuditService{
		repo: repo,
	}
}

// FetchLogs запрашивает логи с фильтрацией.
// Логика фильтрации (пустые строки или конкретные ID) инкапсулирована в репозитории.
func (s *AuditService) FetchLogs(ctx context.Context, agentID, capID string) ([]audit.AuditEvent, error) {
	logs, err := s.repo.FetchLogs(ctx, agentID, capID)
	if err != nil {
		return nil, fmt.Errorf("audit_service: failed to fetch logs: %w", err)
	}
	return logs, nil
}
