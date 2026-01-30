package service

import (
	"context"

	"github.com/redis/go-redis/v9"
	"github.com/xela07ax/spaceai-infra-prototype/internal/domain"
	"github.com/xela07ax/spaceai-infra-prototype/internal/infra"
)

// PolicyRepository описывает требования сервиса к хранилищу политик
type PolicyRepository interface {
	GetPolicyByID(ctx context.Context, id string) (*domain.Policy, error)
	GetAllPolicies(ctx context.Context) ([]domain.Policy, error)
	CreatePolicy(ctx context.Context, p *domain.Policy) error
	UpdatePolicy(ctx context.Context, p *domain.Policy) error
	DeletePolicy(ctx context.Context, id string) error
}

type PolicyService struct {
	repo PolicyRepository
	rdb  *redis.Client
}

func NewPolicyService(repo PolicyRepository, rdb *redis.Client) *PolicyService {
	return &PolicyService{
		repo: repo,
		rdb:  rdb,
	}
}

func (s *PolicyService) GetByID(ctx context.Context, id string) (*domain.Policy, error) {
	return s.repo.GetPolicyByID(ctx, id)
}

// GetAll возвращает все политики из БД
func (s *PolicyService) GetAll(ctx context.Context) ([]domain.Policy, error) {
	return s.repo.GetAllPolicies(ctx)
}

// Create сохраняет политику и уведомляет шлюзы об обновлении
func (s *PolicyService) Create(ctx context.Context, p *domain.Policy) error {
	if err := s.repo.CreatePolicy(ctx, p); err != nil {
		return err
	}
	return s.notifyUpdate(ctx)
}

// Update обновляет политику и инициирует инвалидацию кэша
func (s *PolicyService) Update(ctx context.Context, p *domain.Policy) error {
	if err := s.repo.UpdatePolicy(ctx, p); err != nil {
		return err
	}
	return s.notifyUpdate(ctx)
}

// Delete удаляет политику
func (s *PolicyService) Delete(ctx context.Context, id string) error {
	if err := s.repo.DeletePolicy(ctx, id); err != nil {
		return err
	}
	return s.notifyUpdate(ctx)
}

// notifyUpdate отправляет широковещательный сигнал в Redis.
// Все инстансы UAG, подписанные на этот канал, вызовут Refresh() своего MemoEnforcer.
func (s *PolicyService) notifyUpdate(ctx context.Context) error {
	// Сигнал может быть простым "refresh", так как шлюз сам перечитает всю таблицу
	return s.rdb.Publish(ctx, infra.RedisChanPolicyUpdate, "refresh").Err()
}
