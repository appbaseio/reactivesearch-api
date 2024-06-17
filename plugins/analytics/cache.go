package analytics

// cachedPreferences will contain the preferences for analytics
// for quick access.
var cachedPreferences AnalyticsPreferences = defaultPreferences()

// defaultPreferences will return the default analytics
// preferences
func defaultPreferences() AnalyticsPreferences {
	defaultEnable := true
	return AnalyticsPreferences{
		Enable: &defaultEnable,
	}
}

// GetPreferencesFromCache will return the analytics
// preferences from cache.
func GetPreferencesFromCache() AnalyticsPreferences {
	return cachedPreferences
}

// SetPreferencesInCache will set the analytics preferences
// in cache.
func SetPreferencesInCache(a AnalyticsPreferences) {
	cachedPreferences = a
}

// IsAnalyticsEnabled will check if analytics is enabled or not
// and accordingly return the status.
func IsAnalyticsEnabled() bool {
	prefs := GetPreferencesFromCache()

	// Defaults to `true`
	if prefs.Enable == nil {
		return true
	}

	return *prefs.Enable
}
