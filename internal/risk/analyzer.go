package risk

import (
	"encoding/json"

	"github.com/xela07ax/spaceai-infra-prototype/internal/domain"
	"go.uber.org/zap"
)

// KillSwitchProvider описывает возможности, необходимые анализатору.
// Реализовывать этот интерфейс будет KillSwitchManager из пакета engine.
type KillSwitchProvider interface {
	MarkAsBlocked(agentID string)
}

type Analyzer struct {
	ksm    KillSwitchProvider
	logger *zap.Logger
}

func NewAnalyzer(ksm KillSwitchProvider, logger *zap.Logger) *Analyzer {
	return &Analyzer{ksm: ksm, logger: logger.Named("analyzer")}
}

// IsRequired проверяет, нужно ли отправлять запрос на апрув (HITL)
func (a *Analyzer) IsRequired(p domain.Policy, payload []byte) bool {
	// 1. Быстрая проверка на обязательный карантин
	if p.Effect == domain.EffectQuarantine {
		return true
	}

	// 2. Если эффект ALLOW, проверяем динамические лимиты
	if p.Effect == domain.EffectAllow && len(p.Conditions) > 0 {
		// Описываем структуру условий из БД
		var cond struct {
			RiskField string  `json:"risk_field"`
			Threshold float64 `json:"threshold"`
		}

		if err := json.Unmarshal(p.Conditions, &cond); err != nil {
			return false // Если условия не заданы или битые, пропускаем как обычный ALLOW
		}

		// Если в политике не указано, какое поле проверять — выходим
		if cond.RiskField == "" {
			return false
		}

		// 3. ПАРСИНГ PAYLOAD: Универсальный подход для любого коннектора
		var requestData map[string]interface{}
		if err := json.Unmarshal(payload, &requestData); err != nil {
			a.logger.Error("failed to unmarshal request payload for risk analysis", zap.Error(err))
			return false
		}

		// Пытаемся достать рисковое поле (например, "amount")
		if rawValue, ok := requestData[cond.RiskField]; ok {
			// В JSON числа всегда парсятся в float64
			if val, ok := rawValue.(float64); ok {
				if val > cond.Threshold {
					a.logger.Warn("DYNAMIC APPROVAL TRIGGERED",
						zap.String("field", cond.RiskField),
						zap.Float64("value", val),
						zap.Float64("threshold", cond.Threshold),
					)
					return true // Сумма превышена — включаем HITL
				}
			}
		}
	}

	return false
}
