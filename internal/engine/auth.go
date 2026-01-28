package engine

import (
	"context"
	"net/http"
	"strings"
)

// AuthMiddleware проверяет наличие Scoped-токена и прокидывает права в контекст
func (u *UAGCore) AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Извлекаем токен (в жизни JWT это заголовок Authorization)
		token := r.Header.Get("X-DevAI-Token")
		if token == "" {
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"error": "security_violation", "message": "missing access token"}`))
			return
		}

		// Имитируем парсинг разрешенных систем из токена,
		// просто парсим строку, токен должен содержать права на действие
		// Например: token "scope:jira.read,slack.send"
		// В реальности здесь будет jwt.Parse и проверка подписи
		scopes := make(map[string]bool)
		for _, s := range strings.Split(token, ",") {
			scopes[strings.TrimSpace(s)] = true
		}

		// Обогащаем контекст правами для дальнейшего использования в ProcessAction
		ctx := context.WithValue(r.Context(), "user_scopes", scopes)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
