package domain

import (
	"errors"
	"time"
)

// Статусы State Machine
type ApprovalStatus string

const (
	StatusPending  ApprovalStatus = "PENDING"
	StatusApproved ApprovalStatus = "APPROVED"
	StatusRejected ApprovalStatus = "REJECTED"
)

var (
	ErrInvalidTransition = errors.New("invalid approval status transition")
	ErrAlreadyProcessed  = errors.New("approval request already processed")
)

type ApprovalRequest struct {
	ID          string         `json:"id"`
	ExecutionID string         `json:"execution_id"` // Ссылка на зависший запрос в UAG
	AgentID     string         `json:"agent_id"`
	Capability  string         `json:"capability"`
	Payload     string         `json:"payload"` // Данные, которые агент хотел отправить
	Status      ApprovalStatus `json:"status"`

	ReviewerID *string `json:"reviewer_id,omitempty"`
	Comment    *string `json:"comment,omitempty"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// CanTransitionTo проверяет правила конечного автомата
func (a *ApprovalRequest) CanTransitionTo(next ApprovalStatus) error {
	if a.Status != StatusPending {
		return ErrAlreadyProcessed
	}
	if next == StatusPending {
		return ErrInvalidTransition
	}
	return nil
}
