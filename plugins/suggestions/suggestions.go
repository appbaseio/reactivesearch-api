package suggestions

import (
	"context"
	"os"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/appbaseio/reactivesearch-api/middleware"
	"github.com/appbaseio/reactivesearch-api/plugins"
	"github.com/appbaseio/reactivesearch-api/util"
	"github.com/robfig/cron/v3"
)

const (
	logTag                             = "[suggestions]"
	defaultSuggestionsMetaEsIndex      = ".suggestions_meta"
	defaultSuggestionsPreferencesIndex = ".suggestions_preferences"
	envSuggestionsMetaEsIndex          = "SUGGESTIONS_META_ES_INDEX"
	envSuggestionsPreferencesEsIndex   = "SUGGESTIONS_PREFERENCES_ES_INDEX"
	defaultSuggestionsEsIndex          = ".suggestions"
	typeName                           = "_doc"
	envSuggestionsEsIndex              = "SUGGESTIONS_ES_INDEX"
	indexConfigEs6                     = `
	{
		"settings":{
			%s
		   "index.number_of_shards": 1,
		   "index.number_of_replicas":%d,
		   "analysis":{
			  "analyzer":{
					"autosuggest_analyzer":{
						"filter":[
							"lowercase",
							"asciifolding",
							"autosuggest_filter"
						],
						"tokenizer":"standard",
						"type":"custom"
					}
			  },
			  "filter":{
					"autosuggest_filter":{
							"max_gram":"20",
							"min_gram":"1",
							"token_chars":[
							"letter",
							"digit",
							"punctuation",
							"symbol"
						],
						"type":"edge_ngram"
					}
			  }
		   }
		},
		"mappings":{
		   "_doc": {
				"properties":{
					"key":{
						"type":"text",
						"fields":{
							"autosuggest":{
								"type":"text",
								"analyzer":"autosuggest_analyzer"
							},
							"keyword":{
								"type":"keyword",
								"ignore_above":256
							}
						}
					},
					"count":{
						"type":"integer"
					}
				}
		   }
		}
	 }
	`
	indexConfigEs7 = `
	{
		"settings":{
			%s
		   "index.number_of_shards": 1,
		   "index.number_of_replicas": %d,
		   "analysis":{
			  "analyzer":{
					"autosuggest_analyzer":{
						"filter":[
							"lowercase",
							"asciifolding",
							"autosuggest_filter"
						],
						"tokenizer":"standard",
						"type":"custom"
					}
			  },
			  "filter":{
					"autosuggest_filter":{
						"max_gram":"20",
						"min_gram":"1",
						"token_chars":[
							"letter",
							"digit",
							"punctuation",
							"symbol"
						],
						"type":"edge_ngram"
					}
			  }
		   }
		},
		"mappings":{
			"properties":{
				"key":{
				"type":"text",
				"fields":{
					"autosuggest":{
						"type":"text",
						"analyzer":"autosuggest_analyzer"
					},
					"keyword":{
						"type":"keyword",
						"ignore_above":256
					}
				}
				},
				"count":{
					"type":"integer"
				}
			}
	}
	 }
	`
	indexConfigZinc = `
	{
		"name": "%s",
		"storage_type": "disk",
		"mappings":{
			"properties":{
				"key":{
				"type":"text",
				"fields":{
					"autosuggest":{
						"type":"text",
						"analyzer":"autosuggest_analyzer"
					},
					"keyword":{
						"type":"keyword",
						"ignore_above":256
					}
				}
				},
				"count":{
					"type":"integer"
				},
				"search_characters_length": {
                    "type": "integer"
                }
			}
		}
	 }
	`
	indexSettingsZinc = `
	{
		"settings":{
			%s
		   "index.number_of_shards": 1,
		   "index.number_of_replicas": %d,
		   "analysis":{
			  "analyzer":{
					"autosuggest_analyzer":{
						"filter":[
							"lowercase",
							"asciifolding",
							"autosuggest_filter"
						],
						"tokenizer":"standard",
						"type":"custom"
					}
			  },
			  "filter":{
					"autosuggest_filter":{
						"max_gram":"20",
						"min_gram":"1",
						"token_chars":[
							"letter",
							"digit",
							"punctuation",
							"symbol"
						],
						"type":"edge_ngram"
					}
			  }
		   }
		}
	 }
	`
	// preference document ID (older)
	preferenceDocID = "_preferences"
	// popular preferences document ID
	popularPreferenceDocID = "popular"
	// index preferences document ID
	indexPreferenceDocID = "index"
	// recent preferences document ID
	recentPreferenceDocID = "recent"
	// default preference values
	minCount     = 1
	minChars     = 3
	minHits      = 5
	NumberOfDays = 30
)

var (
	singleton *suggestions
	once      sync.Once
)

type suggestions struct {
	es     suggestionService
	esMeta suggestionMetaService
}

// Use only this function to fetch the instance of suggestions from within
// this package to avoid creating stateless duplicates of the plugin.
func Instance() *suggestions {
	once.Do(func() { singleton = &suggestions{} })
	return singleton
}

func (rx *suggestions) Name() string {
	return logTag
}

func (r *suggestions) InitFunc() error {
	// Create suggestions preferences index if not exists
	indexPreferencesSuffix := os.Getenv(envSuggestionsPreferencesEsIndex)
	if indexPreferencesSuffix == "" {
		indexPreferencesSuffix = defaultSuggestionsPreferencesIndex
	}

	var err error
	var exists bool
	r.esMeta, exists, err = createSuggestionsIndex(indexPreferencesSuffix, indexConfigEs6, indexConfigEs7)
	if err != nil {
		return err
	}

	// Only create document only when a fresh index has been created
	if !exists {
		// Create default preferences
		var record = PopularPreferences{}
		record.MinChars = minChars
		record.MinCount = minCount
		record.MinHits = minHits
		record.NumberOfDays = NumberOfDays
		_, err2 := r.esMeta.savePopularSuggestionsPreferences(context.Background(), record)
		if err2 != nil {
			return err2
		}
	}

	// Add suggestions preferences migration script
	m := SuggestionsPreferencesMigration{
		es: r.esMeta.(*elasticsearch),
	}
	util.AddMigrationScript(m)

	// Sync preferences cache at init
	popularPreferences, err := r.esMeta.getPopularSuggestionsPreferences(context.Background())
	if err != nil {
		return err
	}
	indexPreferences, err := r.esMeta.getIndexSuggestionsPreferences(context.Background())
	if err != nil {
		return err
	}
	recentPreferences, err := r.esMeta.getRecentSuggestionsPreferences(context.Background())
	if err != nil {
		return err
	}

	SetPopularPreferences(popularPreferences)
	SetIndexPreferences(indexPreferences)
	SetRecentPreferences(recentPreferences)

	// if .suggestions index already exists, don't populate suggestions on server restart
	exists = popularPreferences.AliasToIndex != ""
	suggestionsIndex := ""

	if exists {
		// Check if the index exists in ES as well
		existsOnES, existsCheckErr := util.GetClient7().IndexExists(popularPreferences.AliasToIndex).Do(context.Background())
		if existsCheckErr != nil {
			log.Errorln(logTag, ": error while checking if suggestions index exists in ES: ", existsCheckErr)
			return existsCheckErr
		}

		exists = existsOnES
		suggestionsIndex = popularPreferences.AliasToIndex
	}

	if !exists {
		// Create suggestions index at init time only if one for the same day doesn't exist
		// This will prevent multiple server nodes or server restarts from re-creating pop suggestions index
		_, err := syncAnalyticsToSuggestions(r, suggestionsIndex)
		if err != nil {
			return err
		}
	}

	// Add cron job to sync suggestions daily
	// Note: Use UTC timezone and run at 23:30:00 to avoid conflicts with snapshots
	cronjob := cron.New(
		cron.WithLocation(time.UTC))
	cronjob.AddFunc("30 23 * * *", func() {
		_, err4 := syncAnalyticsToSuggestions(r, "")
		if err4 != nil {
			log.Errorln(logTag, ": sync process failed for suggestions, reason:", err4)
		}
	})
	cronjob.Start()

	// Set plugin cache sync script
	s := CacheSyncScript{
		index: indexPreferencesSuffix,
	}
	util.AddSyncScript(s)

	return nil
}

func (rx *suggestions) Routes() []plugins.Route {
	return rx.routes()
}

// Default empty middleware array function
func (rx *suggestions) ESMiddleware() []middleware.Middleware {
	return make([]middleware.Middleware, 0)
}

// Default empty middleware array function
func (rx *suggestions) RSMiddleware() []middleware.Middleware {
	return []middleware.Middleware{
		rx.intercept,
	}
}

// Expose plugin specific routes
func (rx *suggestions) AlternateRoutes() []plugins.Route {
	return make([]plugins.Route, 0)
}
