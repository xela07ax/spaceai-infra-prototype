package handler

import (
	"encoding/json"
	"net/http"

	"github.com/xela07ax/spaceai-infra-prototype/internal/console/service"
	"github.com/xela07ax/spaceai-infra-prototype/internal/domain"
)

type AuthHandler struct {
	service *service.AuthService
}

func NewAuthHandler(s *service.AuthService) *AuthHandler {
	return &AuthHandler{service: s}
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req domain.LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	resp, err := h.service.GenerateToken(r.Context(), req.Username, req.Password)
	if err != nil {
		// не уточняем, что именно неверно (логин или пароль) для защиты от перебора
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
