package handler

import (
	"log"
	"net/http"

	"github.com/xela07ax/spaceai-infra-prototype/internal/console/service"

	"github.com/go-chi/chi/v5"
)

type AgentHandler struct {
	service *service.AgentService // Зависим от интерфейса или конкретного сервиса
}

func NewAgentHandler(s *service.AgentService) *AgentHandler {
	return &AgentHandler{service: s}
}

// Routes Маршруты для Chi
func (h *AgentHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Post("/{agentID}/block", h.BlockAgent) // POST /agents/123/block
	return r
}

func (h *AgentHandler) BlockAgent(w http.ResponseWriter, r *http.Request) {
	agentID := chi.URLParam(r, "agentID")
	if agentID == "" {
		http.Error(w, "agentID is required", http.StatusBadRequest)
		return
	}

	// Вызываем единый метод сервиса, который гарантирует и БД, и Redis
	// Мы ждем завершения обоих действий, чтобы гарантировать безопасность
	if err := h.service.BlockAgent(r.Context(), agentID); err != nil {
		log.Printf("failed to block agent %s: %v", agentID, err)
		// tip: Разделяй типы ошибок (404, 403, 500)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
