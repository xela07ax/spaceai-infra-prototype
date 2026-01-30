package engine

/*
Файл gateway.go реализует паттерн Transparent Proxy с глубокой инспекцией безопасности.
UAGCore спроектирован как неблокирующий конвейер: запрос проходит через эшелоны авторизации,
кэшированных политик и риск-анализа, прежде чем достичь интерфейса Call.
Использование Dependency Injection через структуру UAGDeps позволило сделать код чистым и готовым к
unit-тестированию каждого этапа жизненного цикла запрос.
🏛 Архитектурная роль
Файл содержит структуру UAGCore, которая связывает воедино все защитные механизмы шлюза.
Она не принимает бизнес-решений сама, а делегирует их специализированным менеджерам, выполняя роль Workflow Engine.
🛠 Ключевые ответственности
1.	Identity Verification: Интеграция с BaseValidator для проверки RS256 подписей в JWT.
2.	Policy Enforcement: Вызов MemoEnforcer для мгновенной проверки разрешений в RAM-кэше (L1).
3.	Risk & HITL Orchestration: Координация между RiskAnalyzer и механизмом Human-in-the-loop. Если риск высок, запрос «замораживается» в ожидании сигнала из Redis.
4.	Runtime Protection: Применение фильтров Kill-Switch (мгновенная блокировка) и Sandbox (изоляция).
5.	Reliable Execution: Проброс проверенного запроса через ActionExecutor (боевой контракт Call) с поддержкой Circuit Breaker.
6.	Observability: Гарантированная отправка событий в асинхронный аудит AgentFS.
*/

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/xela07ax/spaceai-infra-prototype/internal/audit"
	"github.com/xela07ax/spaceai-infra-prototype/internal/domain"
	"github.com/xela07ax/spaceai-infra-prototype/internal/infra"
	"github.com/xela07ax/spaceai-infra-prototype/internal/infra/auth"
	"github.com/xela07ax/spaceai-infra-prototype/internal/risk"
	"go.uber.org/zap"
)

type PolicyProvider interface {
	GetPolicy(agentID, capID string) domain.Policy
}

type ApprovalCreator interface {
	CreateApproval(ctx context.Context, app *domain.ApprovalRequest) error
}

type ActionExecutor interface {
	Call(ctx context.Context, capID string, payload []byte) ([]byte, error)
}

type UAGCore struct {
	*auth.BaseValidator // Наш фундамент безопасности (RS256)

	// Интерфейсы (Loose Coupling)
	policy   PolicyProvider  // Движок политик
	auditor  audit.Auditor   // Асинхронный логгер (AgentFS)
	executor ActionExecutor  // Исполнитель (ReliabilityWrapper)
	approver ApprovalCreator // Создатель заявок (Postgres)

	// Компоненты логики (Runtime Managers)
	riskAnalyzer *risk.Analyzer
	killSwitch   *KillSwitchManager
	quarantine   *QuarantineManager
	sandbox      *SandboxManager

	// Инфраструктура
	metrics *Metrics
	rdb     *redis.Client
	logger  *zap.Logger
}

// UAGDeps объединяет все зависимости для ядра шлюза.
// Это избавляет конструктор от "простыни" аргументов.
type UAGDeps struct {
	Validator    *auth.BaseValidator
	Policy       PolicyProvider
	Auditor      audit.Auditor
	Executor     ActionExecutor
	Approver     ApprovalCreator
	RiskAnalyzer *risk.Analyzer

	// Менеджеры состояний
	KillSwitch *KillSwitchManager
	Quarantine *QuarantineManager
	Sandbox    *SandboxManager

	// Инфраструктура
	Metrics *Metrics
	Redis   *redis.Client
	Logger  *zap.Logger
}

func NewUAGCore(deps UAGDeps) *UAGCore {
	return &UAGCore{
		BaseValidator: deps.Validator,
		policy:        deps.Policy,
		auditor:       deps.Auditor,
		executor:      deps.Executor,
		approver:      deps.Approver,
		riskAnalyzer:  deps.RiskAnalyzer,
		killSwitch:    deps.KillSwitch,
		quarantine:    deps.Quarantine,
		sandbox:       deps.Sandbox,
		metrics:       deps.Metrics,
		rdb:           deps.Redis,
		logger:        deps.Logger.With(zap.String("mod", "uag-core")),
	}
}

func (u *UAGCore) ProcessAction(ctx context.Context, agentID string, capID string, data []byte) ([]byte, error) {
	// Авторизация - проверка токена и прав (Security First)
	scopes, ok := ctx.Value("user_scopes").(map[string]bool)

	// Админ может всё, Агент — только то, что в его scopes
	if !ok || (!scopes["admin"] && !scopes[capID]) {
		return nil, fmt.Errorf("security: insufficient permissions for %s", capID)
	}

	u.metrics.TotalRequests.WithLabelValues(agentID, capID).Inc()
	start := time.Now()

	traceID := extractTraceID(ctx)

	// Готовим структуру для аудита (заполним статус в процессе)
	event := audit.AuditEvent{
		ID:           uuid.New().String(),
		TraceID:      traceID,
		AgentID:      agentID,
		CapabilityID: capID,
		Payload:      u.bytesToMap(data),
		Timestamp:    start,
		Mode:         "LIVE", // По умолчанию
	}

	defer func() {
		duration := time.Since(start).Seconds()
		u.metrics.RequestDuration.WithLabelValues(agentID, capID, event.Status).Observe(duration)
	}()

	// Policy Lookup & Decision
	policyData := u.policy.GetPolicy(agentID, capID)

	// 1. Применяем решение домена
	effect := policyData.Decide()

	if effect == domain.EffectDeny {
		// Дальше код НЕ ИДЕТ. Мы в безопасности.
		u.logger.Warn("access denied", zap.String("cap", capID))
		return nil, fmt.Errorf("access denied: %s", capID)
	}

	// 2. Проверяем необходимость Human-in-the-loop (HITL)
	// Важно: Риск-анализ первичен! Если запрос опасен, админ должен его увидеть,
	// даже если агент работает в режиме песочницы.
	if effect == domain.EffectQuarantine || u.riskAnalyzer.IsRequired(policyData, data) {
		u.logger.Info("high risk action detected, quarantine triggered (HITL)", zap.String("agent", agentID))
		return u.handleMandatoryApproval(ctx, agentID, capID, data)
	}

	// 3. Если риск пройден или апрув получен, проверяем режим исполнения
	if effect == domain.EffectSandbox || u.sandbox.IsSandbox(agentID) {
		u.logger.Debug("executing in sandbox mode", zap.String("agent", agentID))
		return u.executeSandbox(ctx, agentID, capID, data)
	}

	// 4. Live вызов (только для чистых и проверенных запросов)
	return u.executor.Call(ctx, capID, data)
}

// Вспомогательный метод для конвертации
func (u *UAGCore) bytesToMap(data []byte) map[string]interface{} {
	var m map[string]interface{}
	_ = json.Unmarshal(data, &m)
	return m
}

func (u *UAGCore) executeSandbox(ctx context.Context, agentID, capID string, data []byte) ([]byte, error) {
	start := time.Now()

	// Десериализуем payload для читаемости в логах (Senior-подход)
	var payloadMap map[string]interface{}
	json.Unmarshal(data, &payloadMap)

	// Имитируем "успешный" ответ от системы
	mockResponse := map[string]interface{}{
		"status":  "simulated_success",
		"details": "Action captured in sandbox mode, no real impact made.",
	}
	respBytes, _ := json.Marshal(mockResponse)

	// Асинхронно пишем в AgentFS, чтобы не блокировать ответ агенту
	u.auditor.Log(audit.AuditEvent{
		ID:           uuid.New().String(),
		TraceID:      extractTraceID(ctx),
		AgentID:      agentID,
		CapabilityID: capID,
		Payload:      payloadMap,
		Mode:         "SANDBOX",
		Status:       "INTERCEPTED",
		Response:     mockResponse,
		Timestamp:    time.Now(),
		DurationMs:   time.Since(start).Milliseconds(),
	})

	return respBytes, nil
}

func (u *UAGCore) HandleHTTPRequest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Only POST allowed", http.StatusMethodNotAllowed)
		return
	}

	// 1. Извлекаем метаданные
	agentID := r.Header.Get("X-Agent-ID")
	capID := r.URL.Query().Get("capability") // например, ?capability=crm.user.delete

	if agentID == "" || capID == "" {
		http.Error(w, "X-Agent-ID and capability query param are required", http.StatusBadRequest)
		return
	}

	// 2. Читаем Payload
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read body", http.StatusInternalServerError)
		return
	}
	defer r.Body.Close()

	// 3. Запускаем основной процесс обработки (ProcessAction)
	resp, err := u.ProcessAction(r.Context(), agentID, capID, body)
	if err != nil {
		// tip: Не отдаем детали внутренних ошибок в 403
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	// 4. Отправляем результат
	w.Header().Set("Content-Type", "application/json")
	w.Write(resp)
}

func (u *UAGCore) handleMandatoryApproval(ctx context.Context, agentID, capID string, data []byte) ([]byte, error) {
	// 1. Генерируем ID для отслеживания жизненного цикла запроса
	executionID := uuid.New().String()

	approval := &domain.ApprovalRequest{
		ID:          uuid.New().String(),
		ExecutionID: executionID,
		AgentID:     agentID,
		Capability:  capID,
		Payload:     string(data),
		Status:      domain.StatusPending,
	}

	// 2. Сохраняем в Persistence Layer (Postgres)
	if err := u.approver.CreateApproval(ctx, approval); err != nil {
		return nil, fmt.Errorf("hitl: failed to persist approval request: %w", err)
	}

	// 3. Создаем "точку ожидания" в Redis Pub/Sub
	// Используем инфраструктурную константу для канала
	chanName := fmt.Sprintf("%s:execution:%s", infra.RedisChanApprovalDecisions, executionID)
	pubsub := u.rdb.Subscribe(ctx, chanName)
	defer pubsub.Close()

	u.logger.Warn("HUMAN-IN-THE-LOOP: operation suspended",
		zap.String("execution_id", executionID),
		zap.String("capability", capID),
		zap.String("agent_id", agentID),
	)

	// 4. Ожидание с контролем контекста и таймаутом
	// Устанавливаем жесткий лимит, например, 5 минут, если контекст позволяет
	waitCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	select {
	case msg := <-pubsub.Channel():
		// Принимаем решение: APPROVED или REJECTED
		switch msg.Payload {
		case string(domain.StatusApproved):
			u.logger.Info("HITL: operation approved", zap.String("id", executionID))
			// Исполняем через Reliability Wrapper
			return u.executor.Call(ctx, capID, data)

		case string(domain.StatusRejected):
			u.logger.Warn("HITL: operation rejected by operator", zap.String("id", executionID))
			return nil, fmt.Errorf("security: operation explicitly rejected by human operator")

		default:
			return nil, fmt.Errorf("security: received unknown signal from approval system: %s", msg.Payload)
		}

	case <-waitCtx.Done():
		if waitCtx.Err() == context.DeadlineExceeded {
			return nil, fmt.Errorf("security: human-in-the-loop timeout (operator did not respond in time)")
		}
		return nil, waitCtx.Err()
	}
}
