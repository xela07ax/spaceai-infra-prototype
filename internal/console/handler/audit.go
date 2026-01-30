package handler

import (
	"encoding/json"
	"net/http"

	"github.com/xela07ax/spaceai-infra-prototype/internal/console/service"
)

type AuditHandler struct {
	service *service.AuditService
}

func NewAuditHandler(s *service.AuditService) *AuditHandler {
	return &AuditHandler{service: s}
}

// GetLogs возвращает список событий аудита с поддержкой фильтрации
// GET /v1/audit?agent_id=...&cap_id=...
func (h *AuditHandler) GetLogs(w http.ResponseWriter, r *http.Request) {
	// Извлекаем фильтры из Query-параметров
	agentID := r.URL.Query().Get("agent_id")
	capID := r.URL.Query().Get("cap_id")

	logs, err := h.service.FetchLogs(r.Context(), agentID, capID)
	if err != nil {
		http.Error(w, "Failed to fetch audit logs", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(logs)
}
