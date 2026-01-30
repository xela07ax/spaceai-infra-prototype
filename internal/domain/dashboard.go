package domain

type UnifiedDashboard struct {
	Activity  ActivityStats `json:"activity"`  // Нагрузка и трафик
	Risks     RiskStats     `json:"risks"`     // Проверки и HITL
	Incidents IncidentStats `json:"incidents"` // Блокировки и сбои
	Quality   QualityStats  `json:"quality"`   // SLO/SLI (Latency)
}

type ActivityStats struct {
	RPS           float64 `json:"rps"`
	TotalRequests int64   `json:"total_requests"`
	ActiveAgents  int     `json:"active_agents"`
}

type RiskStats struct {
	QuarantineRequests int `json:"quarantine_requests"`  // Ждут апрува
	HighRiskDetections int `json:"high_risk_detections"` // Сработки по Conditions
}

type IncidentStats struct {
	BlockedAgents int `json:"blocked_agents"`
	SystemErrors  int `json:"system_errors"` // Ошибки коннекторов/базы
}

type QualityStats struct {
	P95Latency float64 `json:"p95_latency_ms"`
	Uptime     float64 `json:"uptime_percentage"`
}
