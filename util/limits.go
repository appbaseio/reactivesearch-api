package util

// LimitValue will contain the limit value
type LimitValue struct {
	Value   int    `json:"value"`
	Unit    string `json:"string"`
	NoLimit bool   `json:"no_limit"`
}

// PlanLimit will indicate the limit for every plan
type PlanLimit struct {
	DataUsage           LimitValue `json:"data_usage"`
	AnalyticsAndLogsTTL LimitValue `json:"analytics_and_logs_ttl"`
	Indexes             LimitValue `json:"indexes"`
	Pipelines           LimitValue `json:"pipelines"`
	QueryRules          LimitValue `json:"query_rules"`
	Storage             LimitValue `json:"storage"`
	Requests            LimitValue `json:"requests"`
}

// IsLimitExceeded will check if the passed limit exceeds the
// allowed limit for the plan
func (l LimitValue) IsLimitExceeded(value int) bool {
	// If plan doesn't have a limit, always return false
	if l.NoLimit || l.Value == -1 {
		return false
	}

	return value > l.Value
}
