package handler

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/xela07ax/spaceai-infra-prototype/internal/domain"
)

// DashboardService Описываем, что нам нужно от сервиса
type DashboardService interface {
	GetGlobalStats(ctx context.Context) (*domain.GlobalStats, error)
}

type DashboardHandler struct {
	service DashboardService
}

func NewDashboardHandler(s DashboardService) *DashboardHandler {
	return &DashboardHandler{service: s}
}

func (h *DashboardHandler) GetStats(w http.ResponseWriter, r *http.Request) {
	stats, err := h.service.GetGlobalStats(r.Context())
	if err != nil {
		http.Error(w, "Failed to fetch stats", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

func (h *AgentHandler) GetDashboardStats(w http.ResponseWriter, r *http.Request) {
	// Запрос к базе данных (audit_logs)
	// 1. Кол-во действий за час
	// 2. Кол-во блокировок
	// 3. Топ опасных способностей (capabilities)

	stats, _ := h.service.GetGlobalStats(r.Context())
	json.NewEncoder(w).Encode(stats)
}

func (h *AgentHandler) GetDashboard(w http.ResponseWriter, r *http.Request) {
	stats, err := h.service.GetGlobalStats(r.Context())
	if err != nil {
		http.Error(w, "Failed to fetch stats", 500)
		return
	}
	json.NewEncoder(w).Encode(stats)
}
