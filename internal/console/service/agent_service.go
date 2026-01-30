package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/redis/go-redis/v9"
	"github.com/xela07ax/spaceai-infra-prototype/internal/domain"
	"github.com/xela07ax/spaceai-infra-prototype/internal/infra"
	"github.com/xela07ax/spaceai-infra-prototype/internal/infra/auth"
	"go.uber.org/zap"
)

// AgentRepository описывает требования к хранилищу данных об агентах
type AgentRepository interface {
	UpdateAgentStatus(ctx context.Context, agentID string, status string) error
	UpdateApprovalStatus(ctx context.Context, id, status, reviewerID, comment string) (string, error)
	SetAgentSandbox(ctx context.Context, agentID string, enabled bool) error
	GetAgent(ctx context.Context, id string) (*domain.Agent, error)
	GetGlobalStats(ctx context.Context) (*domain.GlobalStats, error)
	GetApprovalByID(ctx context.Context, id string) (*domain.ApprovalRequest, error)
	FindApprovals(ctx context.Context, status domain.ApprovalStatus) ([]*domain.ApprovalRequest, error)
	ListAgents(ctx context.Context) ([]*domain.Agent, error)
}

type AgentService struct {
	*auth.BaseValidator
	repo   AgentRepository
	rdb    *redis.Client
	logger *zap.Logger
}

func NewAgentService(rdb *redis.Client, repo AgentRepository, validator *auth.BaseValidator, logger *zap.Logger) *AgentService {
	return &AgentService{
		BaseValidator: validator,
		repo:          repo,
		rdb:           rdb,
		logger:        logger.Named("agent-service"),
	}
}

// updateAgentState — унифицированный механизм переключения состояний.
// Обновляет БД и транслирует сигнал в Redis.
func (s *AgentService) updateAgentState(
	ctx context.Context,
	agentID string,
	status domain.AgentStatus,
	redisChan string,
	signalValue string,
	actionName string,
) error {
	// 1. Persistence Layer
	if err := s.repo.UpdateAgentStatus(ctx, agentID, string(status)); err != nil {
		s.logger.Error("failed to update agent status in DB",
			zap.String("agent_id", agentID),
			zap.String("action", actionName),
			zap.Error(err))
		return fmt.Errorf("%s database error: %w", actionName, err)
	}

	// 2. Real-time Signaling
	payload := fmt.Sprintf("%s:%s", agentID, signalValue)
	if err := s.rdb.Publish(ctx, redisChan, payload).Err(); err != nil {
		s.logger.Warn("runtime signal delivery failed",
			zap.String("action", actionName),
			zap.String("channel", redisChan),
			zap.Error(err))
	} else {
		s.logger.Info("agent state updated successfully",
			zap.String("agent_id", agentID),
			zap.String("action", actionName),
			zap.String("new_status", string(status)))
	}

	return nil
}

func (s *AgentService) BlockAgent(ctx context.Context, id string) error {
	return s.updateAgentState(ctx, id, domain.StatusBlocked, infra.RedisChanKillSwitch, "true", "kill-switch-block")
}

func (s *AgentService) UnblockAgent(ctx context.Context, id string) error {
	return s.updateAgentState(ctx, id, domain.StatusActive, infra.RedisChanKillSwitch, "false", "kill-switch-unblock")
}

func (s *AgentService) QuarantineAgent(ctx context.Context, id string) error {
	return s.updateAgentState(ctx, id, domain.StatusQuarantine, infra.RedisChanQuarantine, "on", "quarantine-activation")
}

func (s *AgentService) SetSandboxMode(ctx context.Context, agentID string, enabled bool) error {
	// 1. Обновляем ТОЛЬКО поле is_sandbox в базе
	if err := s.repo.SetAgentSandbox(ctx, agentID, enabled); err != nil {
		s.logger.Error("failed to update sandbox in DB", zap.Error(err))
		return err
	}

	// 2. Шлем сигнал в Redis (как и раньше)
	val := "off"
	if enabled {
		val = "on"
	}

	payload := fmt.Sprintf("%s:%s", agentID, val)
	if err := s.rdb.Publish(ctx, infra.RedisChanSandbox, payload).Err(); err != nil {
		s.logger.Warn("sandbox signal failed", zap.Error(err))
	}

	s.logger.Info("sandbox mode toggled",
		zap.String("agent_id", agentID),
		zap.Bool("enabled", enabled))

	return nil
}

func (s *AgentService) GetGlobalStats(ctx context.Context) (*domain.GlobalStats, error) {
	// здесь можно добавить кэширование в Redis на 1 минуту,
	// чтобы не нагружать Postgres тяжелыми аналитическими запросами.
	return s.repo.GetGlobalStats(ctx)
}

// DecideApproval фиксирует решение оператора по запросу из карантина.
// Мы передаем reviewerID для обеспечения подотчетности (Accountability).
func (s *AgentService) DecideApproval(ctx context.Context, approvalID string, approved bool, reviewerID, comment string) error {
	// 1. Определяем финальный статус на основе решения
	status := domain.StatusRejected
	if approved {
		status = domain.StatusApproved
	}

	// 2. Атомарно обновляем БД
	// Метод UpdateApprovalStatus должен возвращать executionID для Redis-сигнала
	executionID, err := s.repo.UpdateApprovalStatus(ctx, approvalID, string(status), reviewerID, comment)
	if err != nil {
		s.logger.Error("failed to persist approval decision",
			zap.String("approval_id", approvalID),
			zap.String("reviewer_id", reviewerID),
			zap.Error(err))
		return fmt.Errorf("database update failed: %w", err)
	}

	// 3. Публикуем сигнал "пробуждения" для горутины шлюза
	// Канал уникален для конкретного запроса: devit:approvals:execution:{executionID}
	chanName := fmt.Sprintf("%s:execution:%s", infra.RedisChanApprovalDecisions, executionID)

	// Шлем статус (APPROVED/REJECTED), который вычитает заждавшийся шлюз
	err = s.rdb.Publish(ctx, chanName, string(status)).Err()
	if err != nil {
		// Если Redis недоступен, горутина на шлюзе завершится по таймауту (Fail-Safe)
		s.logger.Error("critical: decision saved but signal not delivered",
			zap.String("execution_id", executionID),
			zap.Error(err))
		return fmt.Errorf("redis signal failure: %w", err)
	}

	s.logger.Info("HITL decision processed successfully",
		zap.String("execution_id", executionID),
		zap.String("reviewer", reviewerID),
		zap.String("result", string(status)))

	return nil
}

func (s *AgentService) GetApproval(ctx context.Context, id string) (*domain.ApprovalRequest, error) {
	// Здесь можно добавить логику проверки прав текущего пользователя (RBAC),
	// прежде чем отдавать детали запроса на подтверждение.
	return s.repo.GetApprovalByID(ctx, id)
}

func (s *AgentService) GetApprovals(ctx context.Context, status string) ([]*domain.ApprovalRequest, error) {
	// Здесь можно добавить логику проверки прав текущего пользователя (RBAC),
	// прежде чем отдавать детали запроса на подтверждение.

	// Приводим к верхнему регистру, так как в константах PENDING/APPROVED
	status = strings.ToUpper(status)

	// Можно добавить проверку: если статус не входит в список разрешенных - вернуть ошибку
	return s.repo.FindApprovals(ctx, domain.ApprovalStatus(status))
}
func (s *AgentService) GetPending(ctx context.Context) (*domain.ApprovalRequest, error) {
	// Здесь можно добавить логику проверки прав текущего пользователя (RBAC),
	// прежде чем отдавать детали запроса на подтверждение.
	return s.repo.GetApprovalByID(ctx, string(domain.StatusPending))
}

func (s *AgentService) GetAgent(ctx context.Context, agentID string) (*domain.Agent, error) {
	agent, err := s.repo.GetAgent(ctx, agentID)
	if err != nil {
		s.logger.Error("failed to fetch agent details", zap.String("id", agentID), zap.Error(err))
		return nil, err
	}
	return agent, nil
}

// ListAgents возвращает список всех зарегистрированных агентов.
// Используется для отображения основной таблицы в Console API.
func (s *AgentService) ListAgents(ctx context.Context) ([]*domain.Agent, error) {
	agents, err := s.repo.ListAgents(ctx)
	if err != nil {
		s.logger.Error("failed to list agents from repository", zap.Error(err))
		return nil, fmt.Errorf("service: could not fetch agents: %w", err)
	}

	// Senior-акцент: гарантируем, что фронтенд получит пустой массив [], а не null,
	// если в базе еще нет ни одного агента.
	if agents == nil {
		return []*domain.Agent{}, nil
	}

	s.logger.Debug("agents listed successfully", zap.Int("count", len(agents)))
	return agents, nil
}
