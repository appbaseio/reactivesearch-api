package suggestions

var indexSuggestionsPreferences IndexPreferences
var popularSuggestionsPreferences PopularPreferences
var recentSuggestionsPreferences RecentPreferences

// Sets the index preferences
func SetIndexPreferences(pref IndexPreferences) {
	indexSuggestionsPreferences = pref
}

// Sets the popular preferences
func SetPopularPreferences(pref PopularPreferences) {
	// don't override last sync time
	if pref.LastSyncTime == 0 {
		pref.LastSyncTime = popularSuggestionsPreferences.LastSyncTime
	}
	popularSuggestionsPreferences = pref
}

// Sets the recent preferences
func SetRecentPreferences(pref RecentPreferences) {
	recentSuggestionsPreferences = pref
}

// To get the index preferences
func GetIndexPreferences() IndexPreferences {
	return indexSuggestionsPreferences
}

// To get the popular preferences
func GetPopularPreferences() PopularPreferences {
	return popularSuggestionsPreferences
}

// To get the recent preferences
func GetRecentPreferences() RecentPreferences {
	return recentSuggestionsPreferences
}
