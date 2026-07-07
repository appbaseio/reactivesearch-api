package analytics

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/appbaseio/reactivesearch-api/model/index"
	"github.com/appbaseio/reactivesearch-api/plugins/analyticsrequest"
	"github.com/appbaseio/reactivesearch-api/plugins/querytranslate"
	"github.com/buger/jsonparser"
	log "github.com/sirupsen/logrus"
)

type searchResponseEs6 struct {
	Took float64 `json:"took"`
	Hits struct {
		Total int64 `json:"total"`
		Hits  []struct {
			Type  string `json:"_type"`
			ID    string `json:"_id"`
			Index string `json:"_index"`
		} `json:"hits"`
	} `json:"hits"`
}

type mSearchResponseEs6 struct {
	Responses []searchResponseEs6 `json:"responses"`
}

func (a *Analytics) recordAnalyticsES6(esResponse searchResponseEs6, docID, searchID string, isUsingExistingSearchID bool, isRSAPI bool, rsRequestBody *querytranslate.RSQuery, r *http.Request, rules *[]string) {
	var record analyticsrequest.Record
	analyticsRecord, _ := analyticsrequest.FromContext(r.Context())
	if analyticsRecord != nil {
		record = *analyticsRecord
	} else {
		record = analyticsrequest.Record{}
	}
	record.Took = esResponse.Took
	record.TotalHits = &esResponse.Hits.Total
	var searchQuery string
	if isRSAPI {
		if record.SearchQuery != nil {
			searchQuery = *record.SearchQuery
		}
		if rules != nil {
			record.QueryRules = rules
		}
	} else {
		searchQuery = r.Header.Get(XSearchQuery)
	}
	if searchID == "" {
		if searchQuery == "" {
			// We need to store `search_query` as `nil` value to make the `missing_label` work in terms aggregation.
			record.SearchQuery = nil
		} else {
			record.SearchQuery = &searchQuery
		}
	}
	ctxIndices, err := index.FromContext(r.Context())
	if err != nil {
		log.Errorln(logTag, ": cannot fetch indices from request context,", err)
		return
	}

	record.Indices = ctxIndices
	record.TimeStamp = time.Now().Format(time.RFC3339)
	record.IP = extractIPFromRequest(r)
	location := extractLocationFromRequest(r)
	record.Location = location.Coordinates
	record.Country = location.Country
	record.City = location.City
	method := r.Method
	record.Method = &method
	url := r.Host + r.RequestURI
	record.URL = &url

	if isRSAPI {
		calculateAnalyticsForRSAPI(rsRequestBody, &record)
	} else {
		calculateAnalytics(r, &record)
	}

	if a.es == nil {
		return
	}

	err2 := a.es.updateRecord(context.Background(), docID, record)
	if err2 != nil {
		return
	}
	var numberOfFilters int
	var isClicked bool
	if record.SearchFilters != nil {
		numberOfFilters = len(*record.SearchFilters)
	}
	// A click event has happened if suggestions or results clicks are present
	if len(record.ResultClickObjectIds) != 0 || len(record.SuggestionsClickObjectIds) != 0 {
		isClicked = true
	}
	// record the user session
	go a.recordUserSession(RecordUserSessionConfig{
		userID:                  record.UserID,
		isUsingExistingSearchID: isUsingExistingSearchID,
		numberOfFilters:         numberOfFilters,
		isClicked:               isClicked,
		r:                       r,
		customEvents:            record.CustomEvents,
		queryID:                 docID,
	})
}
func (a *Analytics) recordResponseEs6(docID, searchID string, isUsingExistingSearchID bool, isRSAPI bool, rsRequestBody *querytranslate.RSQuery, r *http.Request, responseBody []byte) {
	var esResponse searchResponseEs6
	if isRSAPI {
		took, err := jsonparser.GetFloat(responseBody, "settings", "took")
		if err != nil {
			log.Warnln(logTag, "unable to parse took value", err)
			return
		}
		var total int64
		err2 := jsonparser.ObjectEach(responseBody, func(key []byte, value []byte, dataType jsonparser.ValueType, offset int) error {
			if !contains(querytranslate.RESERVED_KEYS_IN_RESPONSE, string(key)) {
				queryType := getQueryTypeByID(string(key), *rsRequestBody)
				if queryType != nil && *queryType == querytranslate.Suggestion {
					hits, _, _, err := jsonparser.Get(value, "hits", "hits")
					if err != nil {
						log.Warnln(logTag, "unable to read hits value for suggestions", err)
					} else {
						var suggestions []querytranslate.SuggestionHIT
						err := json.Unmarshal(hits, &suggestions)
						if err != nil {
							log.Warnln(logTag, "unable to read suggestions response", err)
						} else {
							total = getTotalCountSuggestions(suggestions)
						}
					}
				} else {
					totalHits, err := jsonparser.GetInt(value, "hits", "total")
					if err != nil {
						log.Warnln(logTag, "unable to parse total hits value", err)
					} else {
						total = totalHits
					}
				}
			}
			return nil
		})
		if err2 != nil {
			log.Warnln(logTag, "unable to parse response", err)
			return
		}
		esResponse.Took = took
		esResponse.Hits.Total = total
		var rules []string
		appliedRules, dataType, _, err3 := jsonparser.Get(responseBody, "settings", "queryRules")
		if err != nil {
			log.Warnln(logTag, "unable to read applied rules", err3)
		}

		if dataType != jsonparser.NotExist {
			err5 := json.Unmarshal(appliedRules, &rules)
			if err5 != nil {
				log.Warnln(logTag, "unable to unmarshal applied rules", err5)
			}
		}

		a.recordAnalyticsES6(esResponse, docID, searchID, isUsingExistingSearchID, isRSAPI, rsRequestBody, r, &rules)
		return
	} else {
		if strings.Contains(r.RequestURI, "_msearch") {
			var m mSearchResponseEs6
			err := json.Unmarshal(responseBody, &m)
			if err != nil {
				log.Errorln(logTag, `: can't unmarshal "_msearch" response : `, err)
				return
			}
			// TODO: why record only the first _msearch response?
			if len(m.Responses) > 0 {
				esResponse = m.Responses[0]
			}
		} else {
			err := json.Unmarshal(responseBody, &esResponse)
			if err != nil {
				log.Errorln(logTag, `: can't unmarshal "_search" response, unable to record es response`, ":", err)
				return
			}
		}
		a.recordAnalyticsES6(esResponse, docID, searchID, isUsingExistingSearchID, isRSAPI, rsRequestBody, r, nil)
		return
	}
}
