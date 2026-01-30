package handler

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/xela07ax/spaceai-infra-prototype/internal/console/service"
	"go.uber.org/zap"

	"github.com/go-chi/chi/v5"
)

type AgentHandler struct {
	service *service.AgentService
	logger  *zap.Logger
}

func NewAgentHandler(s *service.AgentService, logger *zap.Logger) *AgentHandler {
	return &AgentHandler{service: s, logger: logger.Named("http-agent")}
}

// UnblockAgent — снятие блокировки
func (h *AgentHandler) UnblockAgent(w http.ResponseWriter, r *http.Request) {
	agentID := chi.URLParam(r, "agentID")
	if err := h.service.UnblockAgent(r.Context(), agentID); err != nil {
		h.sendError(w, "failed to unblock agent", err, 500)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// SetSandbox — переключение режима песочницы (On/Off через query или body)
func (h *AgentHandler) SetSandbox(w http.ResponseWriter, r *http.Request) {
	agentID := chi.URLParam(r, "agentID")
	// Допустим, передаем статус в query: ?enabled=true
	enabled := r.URL.Query().Get("enabled") == "true"

	if err := h.service.SetSandboxMode(r.Context(), agentID, enabled); err != nil {
		h.sendError(w, "failed to set sandbox mode", err, 500)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ListAgents — список всех агентов для админки
func (h *AgentHandler) ListAgents(w http.ResponseWriter, r *http.Request) {
	agents, err := h.service.ListAgents(r.Context())
	if err != nil {
		h.sendError(w, "failed to list agents", err, 500)
		return
	}
	json.NewEncoder(w).Encode(agents)
}

// Вспомогательный метод для чистоты кода
func (h *AgentHandler) sendError(w http.ResponseWriter, msg string, err error, code int) {
	// Логируем ошибку структурно
	h.logger.Error(msg,
		zap.Error(err),
		zap.Int("http_status", code),
	)

	// Отправляем ответ клиенту (в идеале — JSON)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{
		"error":   msg,
		"message": err.Error(),
	})
}

// GetAgent возвращает полную информацию об агенте, включая метаданные и скоупы
func (h *AgentHandler) GetAgent(w http.ResponseWriter, r *http.Request) {
	agentID := chi.URLParam(r, "agentID")
	if agentID == "" {
		h.sendError(w, "agentID is required", nil, http.StatusBadRequest)
		return
	}

	agent, err := h.service.GetAgent(r.Context(), agentID)
	if err != nil {
		h.sendError(w, "failed to get agent details", err, http.StatusInternalServerError)
		return
	}

	if agent == nil {
		h.sendError(w, "agent not found", nil, http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(agent)
}

// List возвращает список всех агентов.
// Используется для рендеринга главной таблицы в Console UI.
func (h *AgentHandler) List(w http.ResponseWriter, r *http.Request) {
	// 1. Вызываем бизнес-логику через сервис
	// Контекст r.Context() несет в себе Trace-ID и таймауты
	agents, err := h.service.ListAgents(r.Context())
	if err != nil {
		// Используем наш вспомогательный метод для логирования и ответа
		h.sendError(w, "Could not retrieve agents list", err, http.StatusInternalServerError)
		return
	}

	// 2. Устанавливаем заголовок контента
	w.Header().Set("Content-Type", "application/json")

	// 3. Кодируем результат.
	// Если список пуст, благодаря нашей проверке в сервисе, вернется [], а не null.
	if err := json.NewEncoder(w).Encode(agents); err != nil {
		h.sendError(w, "Failed to encode response", err, http.StatusInternalServerError)
		return
	}
}

// Get возвращает детальную информацию об одном агенте по его ID.
// GET /v1/agents/{id}
func (h *AgentHandler) Get(w http.ResponseWriter, r *http.Request) {
	// Извлекаем ID из URL-параметра {id}, который задан в роутере chi
	agentID := chi.URLParam(r, "id")
	if agentID == "" {
		h.sendError(w, "Agent ID is required", nil, http.StatusBadRequest)
		return
	}

	// Запрашиваем данные у сервиса
	agent, err := h.service.GetAgent(r.Context(), agentID)
	if err != nil {
		h.sendError(w, "Failed to retrieve agent details", err, http.StatusInternalServerError)
		return
	}

	// Если сервис вернул nil — значит такого агента нет в БД
	if agent == nil {
		h.sendError(w, "Agent not found", nil, http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(agent); err != nil {
		h.sendError(w, "Encoding error", err, http.StatusInternalServerError)
	}
}

// Block — эндпоинт для мгновенной блокировки агента.
// POST /v1/agents/{id}/block
func (h *AgentHandler) Block(w http.ResponseWriter, r *http.Request) {
	// Извлекаем ID из параметров пути chi роутера
	agentID := chi.URLParam(r, "id")
	if agentID == "" {
		h.sendError(w, "Agent ID is required", fmt.Errorf("missing path parameter"), http.StatusBadRequest)
		return
	}

	// Вызываем сервис, который обновит БД и отправит сигнал в Redis
	if err := h.service.BlockAgent(r.Context(), agentID); err != nil {
		// Если сервис вернул ошибку (например, агент не найден или база упала)
		h.sendError(w, "Failed to execute Kill-Switch", err, http.StatusInternalServerError)
		return
	}

	// Для успешной мутирующей операции (POST) без возврата данных
	// используем 204 No Content или 200 OK с пустым телом.
	w.WriteHeader(http.StatusNoContent)

	// Senior-акцент: логируем действие админа для внутреннего аудита консоли
	h.logger.Info("agent blocked successfully", zap.String("agent_id", agentID))
}

// Unblock — эндпоинт для снятия блокировки с агента.
// POST /v1/agents/{id}/unblock
func (h *AgentHandler) Unblock(w http.ResponseWriter, r *http.Request) {
	agentID := chi.URLParam(r, "id")
	if agentID == "" {
		h.sendError(w, "Agent ID is required", nil, http.StatusBadRequest)
		return
	}

	if err := h.service.UnblockAgent(r.Context(), agentID); err != nil {
		h.sendError(w, "Failed to unblock agent", err, http.StatusInternalServerError)
		return
	}

	h.logger.Info("agent unblocked", zap.String("agent_id", agentID))
	w.WriteHeader(http.StatusNoContent)
}
