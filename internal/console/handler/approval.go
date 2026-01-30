package handler

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/xela07ax/spaceai-infra-prototype/internal/domain"
)

// ApprovalService Описываем, что нам нужно от сервиса
type ApprovalService interface {
	GetApproval(ctx context.Context, id string) (*domain.ApprovalRequest, error)
	GetApprovals(ctx context.Context, status string) ([]*domain.ApprovalRequest, error)
	DecideApproval(ctx context.Context, id string, approved bool, reviewer, comment string) error
}

type ApprovalHandler struct {
	service ApprovalService
}

func NewApprovalHandler(s ApprovalService) *ApprovalHandler {
	return &ApprovalHandler{service: s}
}

func (h *ApprovalHandler) GetDetails(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	approval, err := h.service.GetApproval(r.Context(), id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(approval)
}

func (h *ApprovalHandler) List(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status") // Достаем из ?status=...
	if status == "" {
		status = "PENDING" // Дефолт для удобства админки
	}

	list, err := h.service.GetApprovals(r.Context(), status)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(list)
}

type DecideRequest struct {
	Approved bool   `json:"approved"`
	Comment  string `json:"comment"`
}

func (h *ApprovalHandler) Decide(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var req DecideRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	// Имитируем получение ReviewerID из контекста (авторизованный админ)
	reviewerID := r.Context().Value("user_id").(string)
	if reviewerID == "" {
		http.Error(w, "reviewer_id is required", http.StatusBadRequest)
		return
	}

	err := h.service.DecideApproval(r.Context(), id, req.Approved, reviewerID, req.Comment)
	if err != nil {
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
