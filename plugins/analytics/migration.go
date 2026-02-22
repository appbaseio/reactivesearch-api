package analytics

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/appbaseio/reactivesearch-api/model/reindex"
	"github.com/appbaseio/reactivesearch-api/util"
	log "github.com/sirupsen/logrus"
)

type MappingsMigration struct {
	NewMapping string
	es         *elasticsearch
}

func (m MappingsMigration) ConditionCheck() (bool, *util.Error) {
	errorMsg := `Error occurred while checking condition for analytics mappings update. 
	Try restarting once if it doesn't fix the issue then please contact us by opening an issue on the GitHub repository.`
	// Only run migration script when nested mapping is not present for search filters
	indices := m.es.getSortedIndices(m.es.analyticsIndex)
	var indexName = m.es.analyticsIndex
	if len(indices) > 0 {
		indexName = indices[len(indices)-1]
	}
	response, err := util.GetIndexMapping(indexName, context.Background())
	if err != nil {
		log.Errorln(logTag, ":", err)
		return false, &util.Error{
			Message: errorMsg,
			Err:     err,
		}
	}
	var properties map[string]interface{}
	if indexName != "" {
		if response[indexName] != nil && response[indexName].(map[string]interface{})["mappings"] != nil {
			switch util.GetVersion() {
			case 6:
				if response[indexName].(map[string]interface{})["mappings"].(map[string]interface{})["_doc"] != nil && response[indexName].(map[string]interface{})["mappings"].(map[string]interface{})["_doc"].(map[string]interface{})["properties"] != nil {
					properties = response[indexName].(map[string]interface{})["mappings"].(map[string]interface{})["_doc"].(map[string]interface{})["properties"].(map[string]interface{})
				}
			default:
				if response[indexName].(map[string]interface{})["mappings"].(map[string]interface{})["properties"] != nil {
					properties = response[indexName].(map[string]interface{})["mappings"].(map[string]interface{})["properties"].(map[string]interface{})
				}
			}
		}
	}
	shouldRunMigrationScript := true

	// Check nested mappings for search_filters
	if properties != nil && properties["search_filters"] != nil {
		searchFiltersAsMap, ok := properties["search_filters"].(map[string]interface{})
		if !ok {
			// Run the migration script
			return true, nil
		}
		if searchFiltersAsMap["type"] == "nested" {
			// Mappings is already updated no need to run the script
			shouldRunMigrationScript = false
		} else {
			// Run the migration script
			return true, nil
		}
	}

	// Check nested mappings for hits_in_response
	if properties != nil && properties["hits_in_response"] != nil {
		hitsInResponseAsMap, ok := properties["hits_in_response"].(map[string]interface{})
		if !ok {
			// Run the migration script
			return true, nil
		}
		if hitsInResponseAsMap["type"] == "nested" {
			// Mappings is already updated no need to run the script
			shouldRunMigrationScript = false
		} else {
			// Run the migration script
			return true, nil
		}
	}

	// Check mappings for search_characters_length
	if properties != nil {
		searchCharsAsMap, ok := properties["search_characters_length"].(map[string]interface{})
		if !ok {
			// Run the migration script
			return true, nil
		}
		if searchCharsAsMap["type"] == "integer" {
			// Mappings is already updated no need to run the script
			shouldRunMigrationScript = false
		} else {
			// Run the migration script
			return true, nil
		}
	}

	return shouldRunMigrationScript, nil
}

func (m MappingsMigration) Script() *util.Error {
	log.Println(logTag, "Running migration script for analytics....This process may take some time.")
	errorMsg := `Error occurred while re-indexing the analytics index mapping. 
	Try restarting once if it doesn't fix the issue then please contact us by opening an issue on the GitHub repository.`
	indices := m.es.getSortedIndices(m.es.analyticsIndex)
	var mappingAsMap map[string]interface{}
	err := json.Unmarshal([]byte(m.NewMapping), &mappingAsMap)
	if err != nil {
		log.Errorln(logTag, ":", err)
		return &util.Error{
			Message: errorMsg,
			Err:     err,
		}
	}
	analyticsIndexPrefix := m.es.analyticsIndex + "-"
	reindexConfig := reindex.ReindexConfig{
		Mappings: mappingAsMap,
	}
	// If analytics alias points to more than one index then re-index the last two indices in the increasing sequence
	// For an example, if the last two indices are `.analytics-000002` and `.analytics-000003`
	// then the new indices will be `.analytics-000004` and `.analytics-000005` respectively
	if len(indices) > 0 {
		lastIndexName := indices[len(indices)-1]
		indexNumber := int64(0)
		splitedIndex := strings.Split(lastIndexName, "-")
		if len(splitedIndex) > 1 {
			var err error
			indexNumber, err = strconv.ParseInt(splitedIndex[1], 10, 64)
			if err != nil {
				log.Errorln(logTag, ":", err)
				return &util.Error{
					Message: errorMsg,
					Err:     err,
				}
			}
		}
		// reindex the last two indices
		if len(indices) > 1 {
			// Re-index the last index
			sourceIndexLast := lastIndexName
			destinationIndexLast := analyticsIndexPrefix + fmt.Sprintf("%06d", indexNumber+2)
			log.Println("R-indexing => Source:" + sourceIndexLast + " Dest: " + destinationIndexLast)
			taskDetails, err := reindex.Reindex(context.Background(), sourceIndexLast, &reindexConfig, false, destinationIndexLast)
			if err != nil {
				log.Errorln(logTag, ":", err)
				return &util.Error{
					Message: errorMsg,
					Err:     err,
				}
			}
			go reindex.TrackReindex(reindex.SetAliasConfig{
				AliasName:    m.es.analyticsIndex,
				NewIndex:     destinationIndexLast,
				OldIndex:     sourceIndexLast,
				IsWriteIndex: true,
			}, taskDetails)
			// Re-index the second last index
			sourceIndexSecondLast := indices[len(indices)-2]
			destinationIndexSecondLast := analyticsIndexPrefix + fmt.Sprintf("%06d", indexNumber+1)
			log.Println("R-indexing => Source:" + sourceIndexSecondLast + " Dest: " + destinationIndexSecondLast)
			taskDetails, err2 := reindex.Reindex(context.Background(), sourceIndexSecondLast, &reindexConfig, false, destinationIndexSecondLast)
			if err2 != nil {
				log.Errorln(logTag, ":", err2)
				return &util.Error{
					Message: errorMsg,
					Err:     err2,
				}
			}
			go reindex.TrackReindex(reindex.SetAliasConfig{
				AliasName: m.es.analyticsIndex,
				NewIndex:  destinationIndexSecondLast,
				OldIndex:  sourceIndexSecondLast,
			}, taskDetails)
		} else {
			// Just reindex the first index to a destination index with increased sequence number
			sourceIndex := indices[0]
			destinationIndex := analyticsIndexPrefix + fmt.Sprintf("%06d", indexNumber+1)
			taskDetails, err := reindex.Reindex(context.Background(), sourceIndex, &reindexConfig, false, destinationIndex)
			if err != nil {
				log.Errorln(logTag, ":", err)
				return &util.Error{
					Message: errorMsg,
					Err:     err,
				}
			}
			go reindex.TrackReindex(reindex.SetAliasConfig{
				AliasName:    m.es.analyticsIndex,
				NewIndex:     destinationIndex,
				OldIndex:     sourceIndex,
				IsWriteIndex: true,
			}, taskDetails)
		}
	} else {
		// Rollover has not happened, Just reindex to `analytics-000001`
		sourceIndex := m.es.analyticsIndex
		destinationIndex := analyticsIndexPrefix + "000001"
		taskDetails, err := reindex.Reindex(context.Background(), sourceIndex, &reindexConfig, false, destinationIndex)
		if err != nil {
			log.Errorln(logTag, ":", err)
			return &util.Error{
				Message: errorMsg,
				Err:     err,
			}
		}
		go reindex.TrackReindex(reindex.SetAliasConfig{
			AliasName:    m.es.analyticsIndex,
			NewIndex:     destinationIndex,
			OldIndex:     sourceIndex,
			IsWriteIndex: true,
		}, taskDetails)
	}
	return nil
}

func (m MappingsMigration) IsAsync() bool {
	return true
}

// Returns the analytics mappings
func getAnalyticsMappings() string {
	properties := `
	"properties": {
        "conversion_count": {
            "type": "long"
        },
        "hits_in_response": {
            "type": "nested",
            "properties": {
                "click": {
                    "type": "boolean"
                },
                "click_position": {
                    "type": "long"
                },
                "conversion": {
                    "type": "boolean"
                },
                "id": {
                    "type": "text",
                    "fields": {
                        "keyword": {
                            "type": "keyword",
                            "ignore_above": 256
                        }
                    }
                },
                "index": {
                    "type": "text",
                    "fields": {
                        "keyword": {
                            "type": "keyword",
                            "ignore_above": 256
                        }
                    }
                }
            }
        },
        "indices": {
            "type": "text",
            "fields": {
                "keyword": {
                    "type": "keyword",
                    "ignore_above": 256
                }
            }
        },
        "ip": {
            "type": "text",
            "fields": {
                "keyword": {
                    "type": "keyword",
                    "ignore_above": 256
                }
            }
        },
        "location": {
            "type": "text",
            "fields": {
                "keyword": {
                    "type": "keyword",
                    "ignore_above": 256
                }
            }
        },
        "result_click_count": {
            "type": "long"
        },
        "search_filters": {
            "type": "nested",
            "properties": {
                "key": {
                    "type": "text",
                    "fields": {
                        "keyword": {
                            "type": "keyword",
                            "ignore_above": 256
                        }
                    }
                },
                "value": {
                    "type": "text",
                    "fields": {
                        "keyword": {
                            "type": "keyword",
                            "ignore_above": 256
                        }
                    }
                }
            }
        },
        "search_query": {
            "type": "text",
            "fields": {
                "keyword": {
                    "type": "keyword",
                    "ignore_above": 256
                }
            }
        },
        "search_query_length": {
            "type": "long"
        },
        "search_state": {
            "type": "text",
            "fields": {
                "keyword": {
                    "type": "keyword",
                    "ignore_above": 256
                }
            }
        },
        "suggestion_click_count": {
            "type": "long"
        },
        "suggestion_click_object_ids": {
            "type": "text",
            "fields": {
                "keyword": {
                    "type": "keyword",
                    "ignore_above": 256
                }
            }
        },
        "suggestion_click_position_ids": {
            "type": "long"
        },
        "timestamp": {
            "type": "date"
        },
        "took": {
            "type": "long"
        },
        "total_hits": {
            "type": "long"
		},
		"search_characters_length": {
            "type": "integer"
        },
        "user_id": {
            "type": "text",
            "fields": {
                "keyword": {
                    "type": "keyword",
                    "ignore_above": 256
                }
            }
        }
    }
	`
	switch util.GetVersion() {
	case 6:
		return fmt.Sprintf(`{ "_doc":  { %s } }`, properties)
	default:
		return fmt.Sprintf(`{ %s }`, properties)
	}
}
