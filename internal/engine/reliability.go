package engine

import (
	"context"
	"errors"
	"fmt"

	"github.com/xela07ax/spaceai-infra-prototype/internal/connectors"

	"time"

	"github.com/avast/retry-go/v5"
	"github.com/sony/gobreaker"
	"golang.org/x/time/rate"
)

type ReliabilityWrapper struct {
	next    ExecutionProvider
	cb      *gobreaker.CircuitBreaker
	limiter *rate.Limiter
}

func NewReliabilityWrapper(next ExecutionProvider) *ReliabilityWrapper {
	// Настройка предохранителя
	cb := gobreaker.NewCircuitBreaker(gobreaker.Settings{
		Name:        "uag-connector",
		MaxRequests: 3,
		Interval:    5 * time.Second,
		Timeout:     30 * time.Second, // Время, через которое CB попробует "закрыться"
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			// Если более 5 ошибок подряд — открываемся (блокируем трафик)
			return counts.ConsecutiveFailures > 5
		},
	})

	// Настройка лимитера (например, 100 запросов в секунду)
	limiter := rate.NewLimiter(rate.Limit(100), 20)

	return &ReliabilityWrapper{
		next:    next,
		cb:      cb,
		limiter: limiter,
	}
}

func (w *ReliabilityWrapper) Call(ctx context.Context, capID string, payload []byte) (res []byte, err error) {
	// 1. Rate Limiter
	if err := w.limiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("rate limit exceeded: %w", err)
	}

	var finalData []byte

	// 2. Circuit Breaker
	cbResult, err := w.cb.Execute(func() (interface{}, error) {
		r := retry.New(
			retry.Context(ctx),
			retry.Attempts(3),
			// Умный расчет задержки
			retry.DelayType(func(n uint, err error, config retry.DelayContext) time.Duration {
				// Если коннектор вернул ThrottleError (например, считал Retry-After заголовок)
				var tErr *connectors.ThrottleError
				if errors.As(err, &tErr) {
					return tErr.RetryAfter
				}

				// В остальных случаях (сетевой лаг, 500-ка) — стандартный экспоненциальный бэкофф
				return retry.BackOffDelay(n, err, config)
			}),
		)

		retryErr := r.Do(func() error {
			tCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			defer cancel()

			var callErr error
			finalData, callErr = w.next.Call(tCtx, capID, payload)
			return callErr
		})

		return finalData, retryErr
	})

	if err != nil {
		return nil, err
	}

	return cbResult.([]byte), nil
}
