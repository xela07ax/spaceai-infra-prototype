package handler

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/xela07ax/spaceai-infra-prototype/internal/console/service"
	"github.com/xela07ax/spaceai-infra-prototype/internal/domain"
)

type PolicyHandler struct {
	service *service.PolicyService
}

func NewPolicyHandler(s *service.PolicyService) *PolicyHandler {
	return &PolicyHandler{service: s}
}

// Get возвращает детали конкретной политики по её ID.
// GET /v1/policies/{id}
func (h *PolicyHandler) Get(w http.ResponseWriter, r *http.Request) {
	// Извлекаем ID из параметров пути chi
	id := chi.URLParam(r, "id")
	if id == "" {
		http.Error(w, "Policy ID is required", http.StatusBadRequest)
		return
	}

	// Запрашиваем политику через сервис
	policy, err := h.service.GetByID(r.Context(), id)
	if err != nil {
		// Если это внутренняя ошибка или ошибка БД
		http.Error(w, "Failed to retrieve policy: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Если политика не найдена (nil), возвращаем 404
	if policy == nil {
		http.Error(w, "Policy not found", http.StatusNotFound)
		return
	}

	// Устанавливаем заголовки и отправляем JSON
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(policy); err != nil {
		http.Error(w, "Encoding error", http.StatusInternalServerError)
	}
}

// List возвращает все политики для админки
func (h *PolicyHandler) List(w http.ResponseWriter, r *http.Request) {
	policies, err := h.service.GetAll(r.Context())
	if err != nil {
		http.Error(w, "Failed to fetch policies", http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(policies)
}

// Create создает новую политику (включая Wildcard '*')
func (h *PolicyHandler) Create(w http.ResponseWriter, r *http.Request) {
	var p domain.Policy
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if err := h.service.Create(r.Context(), &p); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusCreated)
}

// Update обновляет существующую политику (например, меняет Conditions)
func (h *PolicyHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var p domain.Policy
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	p.ID = id

	if err := h.service.Update(r.Context(), &p); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Delete удаляет политику и инициирует инвалидацию кэша
func (h *PolicyHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.service.Delete(r.Context(), id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
