package infra

import "fmt"

const (
	// RedisNamespace Базовый префикс для изоляции данных проекта в Redis
	RedisNamespace = "devit"
)

// Ключи для Sets (состояние)
const (
	RedisKeyBlockedAgents         = RedisNamespace + ":agents:blocked_set"
	RedisKeySandboxAgents         = RedisNamespace + ":agents:sandbox_set"
	RedisKeyQuarantineAgents      = RedisNamespace + ":agents:quarantine_set"
	RedisKeyLockBlocked           = RedisNamespace + ":lock:warmup:blocked"
	RedisKeyLockBlockedSandbox    = RedisNamespace + ":lock:warmup_sandbox:blocked"
	RedisKeyLockBlockedQuarantine = RedisNamespace + ":lock:warmup_quarantine:blocked"
	RedisKeyLockApprovalsExec     = RedisNamespace + ":approvals:execution:"
)

// Каналы Pub/Sub (события)
const (
	// RedisChanApprovalDecisions — канал для трансляции решений оператора (HITL).
	RedisChanApprovalDecisions = RedisNamespace + ":approvals"
	RedisChanKillSwitch        = RedisNamespace + ":agents:kill-switch-signal"
	RedisChanSandbox           = RedisNamespace + ":agents:sandbox-signal"
	RedisChanQuarantine        = RedisNamespace + ":agents:quarantine-signal"
	RedisChanPolicyUpdate      = RedisNamespace + ":agents:policy-update"
)

// GetWarmupLockKey Генератор ключей для блокировок (если нужны динамические)
func GetWarmupLockKey(resource string) string {
	return fmt.Sprintf("%s:lock:warmup:%s", RedisNamespace, resource)
}
