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
		EcommIntegration,
	}

	// NOTE: we are storing the address of the isAdmin variable in the user
	isAdminTrue  = true
	isAdminFalse = false
)
