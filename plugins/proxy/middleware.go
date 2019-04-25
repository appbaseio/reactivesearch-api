package proxy

type getArcDetails struct {
	Message    string               `json:"message"`
	ArcRecords []arcInstanceDetails `json:"arc_records"`
}

type arcInstanceDetails struct {
	NodeCount      int64                  `json:"node_count"`
	Description    string                 `json:"description"`
	SubscriptionID string                 `json:"subscription_id"`
	Trial          bool                   `json:"trial"`
	TrialValidity  int64                  `json:"trial_validity"`
	CreatedAt      int64                  `json:"created_at"`
	Tier           string                 `json:"tier"`
	TierValidity   int64                  `json:"tier_validity"`
	MetaData       map[string]interface{} `json:"metadata"`
}
