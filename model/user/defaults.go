package user

var (
	adminActions = []UserAction{
		Develop,
		Analytics,
		CuratedInsights,
		SearchRelevancy,
		AccessControl,
		UserManagement,
		Billing,
		DowntimeAlerts,
		UIBuilder,
		Speed,
		Pipelines,
	}
	defaultSources = []string{"0.0.0.0/0"}

	// NOTE: we are storing the address of the isAdmin variable in the user
	isAdminTrue  = true
	isAdminFalse = false
)
