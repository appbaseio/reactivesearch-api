package util

import (
	"os"
	"strings"

	log "github.com/sirupsen/logrus"
)

const envSetupProfile = "RS_SETUP_PROFILE"

// SetupProfile controls which meta (system) Elasticsearch indices are created
// at startup. Defaults to full (current behavior) when unset.
type SetupProfile string

const (
	SetupProfileFull     SetupProfile = "full"
	SetupProfileStandard SetupProfile = "standard"
	SetupProfileMinimal  SetupProfile = "minimal"
)

// Meta index keys used by ShouldCreateMetaIndex. Each key maps to the minimum
// setup profile required to create that index.
const (
	MetaIndexUsers                = "users"
	MetaIndexPermissions          = "permissions"
	MetaIndexPipelines            = "pipelines"
	MetaIndexPipelineVars         = "pipeline_vars"
	MetaIndexSynonyms             = "synonyms"
	MetaIndexSearchRelevancy      = "searchrelevancy"
	MetaIndexLogs                 = "logs"
	MetaIndexPipelineLogs         = "pipeline_logs"
	MetaIndexPipelineInvocations  = "pipeline_invocations"
	MetaIndexAnalytics            = "analytics"
	MetaIndexUserSessions         = "user_sessions"
	MetaIndexAnalyticsPreferences = "analytics_preferences"
	MetaIndexPublicKey            = "publickey"
	MetaIndexAnalyticsInsights    = "actionableinsights"
	MetaIndexSavedSearches        = "saved_searches"
	MetaIndexFavorites            = "favorites"
	MetaIndexRecentDocuments      = "documents"
	MetaIndexNodes                = "nodes"
	MetaIndexOpenAI               = "openai"
	MetaIndexAIAnalytics          = "ai_analytics"
	MetaIndexAIFAQs               = "ai_faqs"
	MetaIndexSearchGrader         = "searchgrader"
	MetaIndexSyncPreferences      = "sync_preferences"
	MetaIndexUIBuilderPreferences = "uibuilder_preferences"
	MetaIndexSearchBox            = "searchbox"
	MetaIndexFeaturedSuggestions  = "featured_suggestions"
	MetaIndexRules                = "rules"
	MetaIndexCache                = "cache"
	MetaIndexSuggestionsPrefs     = "suggestions_preferences"
	MetaIndexStoredQuery          = "storedquery"
	MetaIndexCacheStats           = "cache_stats"
)

var metaIndexMinProfile = map[string]SetupProfile{
	MetaIndexUsers:                SetupProfileMinimal,
	MetaIndexPermissions:          SetupProfileMinimal,
	MetaIndexPipelines:            SetupProfileMinimal,
	MetaIndexPipelineVars:         SetupProfileMinimal,
	MetaIndexSynonyms:             SetupProfileStandard,
	MetaIndexSearchRelevancy:      SetupProfileStandard,
	MetaIndexLogs:                 SetupProfileStandard,
	MetaIndexPipelineLogs:         SetupProfileStandard,
	MetaIndexPipelineInvocations:  SetupProfileStandard,
	MetaIndexAnalytics:            SetupProfileStandard,
	MetaIndexUserSessions:         SetupProfileStandard,
	MetaIndexAnalyticsPreferences: SetupProfileStandard,
}

// GetSetupProfile returns the active setup profile from RS_SETUP_PROFILE.
// Unset or unknown values default to full for backward compatibility.
func GetSetupProfile() SetupProfile {
	raw := strings.TrimSpace(strings.ToLower(os.Getenv(envSetupProfile)))
	switch SetupProfile(raw) {
	case SetupProfileMinimal, SetupProfileStandard, SetupProfileFull:
		return SetupProfile(raw)
	case "":
		return SetupProfileFull
	default:
		log.Warnln("unknown RS_SETUP_PROFILE value, using full:", raw)
		return SetupProfileFull
	}
}

func profileRank(p SetupProfile) int {
	switch p {
	case SetupProfileMinimal:
		return 1
	case SetupProfileStandard:
		return 2
	case SetupProfileFull:
		return 3
	default:
		return 3
	}
}

// ShouldCreateMetaIndex reports whether the given meta index should be created
// for the active setup profile.
func ShouldCreateMetaIndex(key string) bool {
	minProfile, ok := metaIndexMinProfile[key]
	if !ok {
		return profileRank(GetSetupProfile()) >= profileRank(SetupProfileFull)
	}
	return profileRank(GetSetupProfile()) >= profileRank(minProfile)
}

// MetaIndexShards returns the number of primary shards to use for a meta index.
// Minimal and standard profiles always use 1; full uses the plugin default.
func MetaIndexShards(defaultShards int) int {
	if GetSetupProfile() == SetupProfileFull {
		return defaultShards
	}
	return 1
}

// IsAnalyticsPluginEnabled reports whether any analytics meta index is enabled
// for the active setup profile.
func IsAnalyticsPluginEnabled() bool {
	return ShouldCreateMetaIndex(MetaIndexAnalytics) ||
		ShouldCreateMetaIndex(MetaIndexUserSessions) ||
		ShouldCreateMetaIndex(MetaIndexAnalyticsPreferences) ||
		ShouldCreateMetaIndex(MetaIndexAnalyticsInsights) ||
		ShouldCreateMetaIndex(MetaIndexSavedSearches) ||
		ShouldCreateMetaIndex(MetaIndexFavorites) ||
		ShouldCreateMetaIndex(MetaIndexRecentDocuments)
}
