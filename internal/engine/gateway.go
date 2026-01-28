package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/xela07ax/spaceai-infra-prototype/internal/audit"
	"github.com/xela07ax/spaceai-infra-prototype/internal/policy"

	"github.com/google/uuid"
)

type ExecutionProvider interface {
	Call(ctx context.Context, capID string, payload []byte) ([]byte, error)
}

type UAGCore struct {
	pdp        policy.Enforcer
	auditor    *audit.AgentFS
	executor   ExecutionProvider
	killSwitch *KillSwitchManager
	quarantine *QuarantineManager
	sandbox    *SandboxManager
	metrics    *Metrics
}

func NewUAGCore(pdp policy.Enforcer, auditor *audit.AgentFS, exec ExecutionProvider, ks *KillSwitchManager, qm *QuarantineManager, sb *SandboxManager, metrics *Metrics) *UAGCore {
	return &UAGCore{
		pdp:        pdp,
		auditor:    auditor,
		executor:   exec,
		killSwitch: ks,
		quarantine: qm,
		sandbox:    sb,
		metrics:    metrics,
	}
}

func (u *UAGCore) ProcessAction(ctx context.Context, agentID string, capID string, data []byte) ([]byte, error) {
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

	// ПРОВЕРКА ПРАВ ИЗ ТОКЕНА (Scopes)
	if scopes, ok := ctx.Value("user_scopes").(map[string]bool); ok {
		if !scopes[capID] {
			return nil, fmt.Errorf("security: token does not grant permission for %s", capID)
		}
	} else {
		return nil, fmt.Errorf("security: unauthorized access attempt")
	}

	// 1. Проверка Kill-Switch (Мгновенная блокировка)(Самый дешевый - In-memory)
	if u.killSwitch.IsBlocked(agentID) {
		event.Status = "BLOCKED"
		u.auditor.Log(event)
		return nil, fmt.Errorf("security: agent %s is blocked", agentID)
	}

	// ШАГ 1.5: Проверка Карантина
	if u.quarantine.IsQuarantined(agentID) {
		// В карантине мы принудительно отправляем запрос на Approval
		return u.handleMandatoryApproval(ctx, agentID, capID, data)
	}

	// ШАГ 0: Проверка токена и прав (Security First)
	scopes, ok := ctx.Value("user_scopes").(map[string]bool)
	if !ok || !scopes[capID] {
		event.Status = "SECURITY_VIOLATION"
		u.auditor.Log(event)
		return nil, fmt.Errorf("security: token does not grant capability %s", capID)
	}

	// 2. Policy Enforcement (PDP)
	allowed, err := u.pdp.Authorize(ctx, agentID, capID, data)
	if err != nil || !allowed {
		event.Status = "DENIED"
		u.auditor.Log(event)
		return nil, fmt.Errorf("policy: access denied for capability %s", capID)
	}

	// 3. Выбор режима: Sandbox vs Live
	var resp []byte
	var execErr error

	if u.sandbox.IsSandbox(agentID) {
		event.Mode = "SANDBOX"
		resp, execErr = u.executeSandbox(ctx, agentID, capID, data)
	} else {
		// Реальное выполнение
		// Вызов через ReliabilityWrapper (Retries/CB/Timeouts)
		resp, execErr = u.executor.Call(ctx, capID, data)
	}

	// 4. Финальный аудит результата
	event.DurationMs = time.Since(start).Milliseconds()
	if execErr != nil {
		event.Status = "FAILED"
		event.Error = execErr.Error()
	} else {
		event.Status = "SUCCESS"
		// Десериализуем ответ для сохранения в аудит
		json.Unmarshal(resp, &event.Response)
	}

	// Асинхронная запись в AgentFS
	u.auditor.Log(event)
	return resp, execErr
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
	// Логируем попытку действия в карантине
	u.auditor.Log(audit.AuditEvent{
		AgentID: agentID,
		Status:  "QUARANTINE_PENDING",
		Mode:    "MANDATORY_APPROVAL",
	})

	// Возвращаем специальный ответ: "Запрос отправлен на проверку ИБ"
	return []byte(`{"status": "pending", "reason": "agent_in_quarantine", "approval_required": true}`), nil
}
