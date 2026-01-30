package engine

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"github.com/google/uuid"
	"github.com/sony/gobreaker"
	"golang.org/x/time/rate"
)

type ProtectedConnector struct {
	next    ActionExecutor
	limiter *rate.Limiter
	cb      *gobreaker.CircuitBreaker
}

// Тип для ключа в контексте (избегаем коллизий)
type ctxKey string

const traceIDKey ctxKey = "trace_id"

// TracingMiddleware инициализирует Trace-ID для каждого запроса
func TracingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 1. Пытаемся достать ID из заголовка (если пришел от агента/прокси)
		traceID := r.Header.Get("X-Trace-ID")

		// 2. Если его нет — генерируем новый
		if traceID == "" {
			traceID = uuid.New().String()
		}

		// 3. Кладем в контекст
		ctx := context.WithValue(r.Context(), traceIDKey, traceID)

		// 4. Добавляем в ответ, чтобы клиент тоже знал ID своего запроса
		w.Header().Set("X-Trace-ID", traceID)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// extractTraceID помогает безопасно достать ID в любом месте кода
func extractTraceID(ctx context.Context) string {
	if id, ok := ctx.Value(traceIDKey).(string); ok {
		return id
	}
	return "00000000-0000-0000-0000-000000000000" // Fallback
}

func (p *ProtectedConnector) Call(ctx context.Context, capID string, payload []byte) ([]byte, error) {
	// 1. Rate Limiting
	if err := p.limiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("rate limit exceeded")
	}

	// 2. Circuit Breaker
	result, err := p.cb.Execute(func() (interface{}, error) {
		return p.next.Call(ctx, capID, payload)
	})

	return result.([]byte), err
}

// Middleware Интегрируем проверку блокировки Агента в HTTP-пайплайн шлюза.
func (ksm *KillSwitchManager) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Достаем ID агента (например, из заголовка или контекста после Auth)
		agentID := r.Header.Get("X-DevIT-Agent-ID")
		if agentID == "" {
			next.ServeHTTP(w, r)
			return
		}

		if ksm.IsBlocked(agentID) {
			// Важно: логируем попытку доступа заблокированного агента
			log.Printf("Intercepted blocked agent request: %s", agentID)

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			w.Write([]byte(`{"error": "agent_quarantined", "reason": "security_kill_switch"}`))
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (sm *SandboxManager) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		agentID := r.Header.Get("X-Agent-ID")
		ctx := r.Context()
		if sm.IsSandbox(agentID) {
			ctx = context.WithValue(ctx, "is_sandbox", true)
			w.Header().Set("X-DevIT-Mode", "Sandbox")
		}
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
