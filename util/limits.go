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
