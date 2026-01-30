package domain

type GlobalStats struct {
	TotalActions    int64            `json:"total_actions"`
	BlockedActions  int64            `json:"blocked_actions"`
	RiskRatio       float64          `json:"risk_ratio"`
	TopCapabilities map[string]int64 `json:"top_capabilities"`
	HourlyActivity  []ActivityPoint  `json:"hourly_activity"`
}

type ActivityPoint struct {
	Hour  string `json:"hour"`
	Count int64  `json:"count"`
}
