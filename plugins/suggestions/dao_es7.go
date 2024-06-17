package suggestions

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	log "github.com/sirupsen/logrus"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"

	"github.com/appbaseio-confidential/reactivesearch/util"
	es7 "github.com/olivere/elastic/v7"
)

func (es *elasticsearch) updateLastSyncTimeEs7(ctx context.Context, record map[string]interface{}) (interface{}, error) {
	return util.GetClient7().
		Update().
		Index(es.indexSuffix).
		Id(popularPreferenceDocID).
		Doc(record).
		Do(ctx)
}

// Query the .analytics index to get the query suggestions
func (es *elasticsearch) querySuggestionsEs7(ctx context.Context, preferences PopularPreferences, customEvents []string, afterKey map[string]interface{}) (*es7.SearchResult, error) {
	query := es7.NewBoolQuery()
	// Add min_hits constraint
	query = query.Must(es7.NewRangeQuery("total_hits").Gte(preferences.MinHits))
	var dateRange = "now-" + strconv.Itoa(int(preferences.NumberOfDays)) + "d/d"
	// Add number_of_days constraint
	query = query.Must(es7.NewRangeQuery("timestamp").Gte(dateRange))
	// Apply indices constraint
	if preferences.Indices != nil && len(preferences.Indices) > 0 {
		// ignore wildcard index
		if preferences.Indices[0] != "*" {
			util.GetIndexFilterQueryEs7(query, preferences.Indices...)
		}
	}
	// Aggregation query
	aggr := es7.NewCompositeAggregation().
		AggregateAfter(afterKey).
		Size(1000).
		Sources(es7.NewCompositeAggregationTermsValuesSource("queries").
			Field("search_query.keyword")).
		SubAggregation("distinct-ip-count", es7.NewCardinalityAggregation().
			Field("ip.keyword")).
		SubAggregation("indices", es7.NewTermsAggregation().Field("indices.keyword"))
	// Add sub aggs for each custom event
	for _, event := range customEvents {
		field := event + ".keyword"
		aggr.SubAggregation(event, es7.NewTermsAggregation().Field(field))
	}
	aggrResult, err := util.GetClient7().Search(".analytics").
		Query(query).
		Aggregation("top-terms", aggr).
		Size(0).
		Do(ctx)
	return aggrResult, err
}

func (es *elasticsearch) populateTimeStampedIndexZinc(ctx context.Context, timestampedIndex string, zc *util.ZincClient) (interface{}, error) {
	// Get the preferences
	preferences := GetPopularPreferences()
	var afterKeyString string
	var isAppliedExternalSuggestions = false
	// retrieve the custom events
	customEvents, err := getCustomEvents()
	if err != nil {
		return false, err
	}
	for {
		var suggestions []Suggestion
		var afterKey map[string]interface{}
		// Set after key if present
		if afterKeyString != "" {
			afterKey = make(map[string]interface{})
			afterKey["queries"] = afterKeyString
		}
		// Start the process
		aggrResult, err2 := es.querySuggestionsEs7(ctx, preferences, customEvents, afterKey)
		if err2 != nil {
			log.Errorln(logTag, ": error executing request:", err2)
			return false, err2
		}

		var filteredSuggestions []Suggestion

		// Apply external suggestions without any filtering
		if !isAppliedExternalSuggestions && preferences.ExternalSuggestions != nil && len(preferences.ExternalSuggestions) != 0 {
			for _, externalSuggestion := range preferences.ExternalSuggestions {
				queryLength := len(externalSuggestion.Key)
				newSuggestion := Suggestion{
					ID:           externalSuggestion.Key,
					Key:          externalSuggestion.Key,
					Count:        externalSuggestion.Count,
					Indices:      externalSuggestion.Indices,
					Meta:         externalSuggestion.Meta,
					CustomEvents: externalSuggestion.CustomEvents,
					QueryLength:  &queryLength,
				}
				filteredSuggestions = append(filteredSuggestions, newSuggestion)
			}
			isAppliedExternalSuggestions = true
		}

		topHits, _ := aggrResult.Aggregations.Composite("top-terms")

		afterKeyString, _ = topHits.AfterKey["queries"].(string)
		if len(filteredSuggestions) == 0 && len(topHits.Buckets) == 0 {
			// Terminate the loop, empty bucket means there won't we any results next
			break
		}

		for _, bucket := range topHits.Buckets {
			str, _ := bucket.Key["queries"].(string)
			count, _ := bucket.Cardinality("distinct-ip-count")
			indicesBucket, _ := bucket.Terms("indices")
			indices := make([]string, 0)
			if indicesBucket != nil {
				indexMap := make(map[string]bool)
				for _, v := range indicesBucket.Buckets {
					index, ok := v.Key.(string)
					if ok {
						indexMap[index] = true
					}
				}
				for k := range indexMap {
					indices = append(indices, k)
				}
			}

			suggestionCustomEvents := make(map[string][]interface{})
			// Extract custom events
			for _, customEvent := range customEvents {
				customEventBucket, _ := bucket.Terms(customEvent)
				if customEventBucket != nil {
					customEventMap := make(map[interface{}]bool)
					for _, v := range customEventBucket.Buckets {
						customEventMap[v.Key] = true
					}
					customEventArray := make([]interface{}, 0)
					for k := range customEventMap {
						customEventArray = append(customEventArray, k)
					}
					suggestionCustomEvents[customEvent] = customEventArray
				}
			}
			var y int = int(*count.Value)
			intCount := int64(y)
			// Remove whitespaces
			queryKey := strings.TrimSpace(str)
			// transform to lower case
			queryKey = strings.ToLower(queryKey)
			// remove diacritics
			if preferences.TransformDiacritics {
				t := transform.Chain(norm.NFD, transform.RemoveFunc(isMn), norm.NFC)
				queryKey, _, _ = transform.String(t, queryKey)
			}
			queryLength := len(queryKey)
			newSuggestion := Suggestion{
				ID:           queryKey,
				Key:          queryKey,
				Count:        &intCount,
				Indices:      indices,
				QueryLength:  &queryLength,
				CustomEvents: suggestionCustomEvents,
			}
			suggestions = append(suggestions, newSuggestion)
		}

		// Filter results by min_count & blacklist
		for _, suggestion := range suggestions {
			// Filter suggestions by blacklist items
			if preferences.Blacklist != nil && len(preferences.Blacklist) != 0 {
				if Contains(preferences.Blacklist, suggestion.Key) {
					continue
				}
			}
			// Filter suggestions by min_count
			if *suggestion.Count < preferences.MinCount {
				continue
			}
			// Filter suggestions by min_chars
			if int64(len(suggestion.Key)) < preferences.MinChars {
				continue
			}
			filteredSuggestions = append(filteredSuggestions, suggestion)
		}

		// Create the bulk request body manually by separating it with
		// a \n.
		bulkRequestEachArr := make([]string, 0)
		bulkRequestEs := util.GetClient7().Bulk()

		for _, filteredSuggestion := range filteredSuggestions {
			byteRecord, err := json.Marshal(filteredSuggestion)
			if err != nil {
				log.Errorln(logTag, ":", err)
				return false, err
			}
			var suggestionDoc map[string]interface{}
			err2 := json.Unmarshal(byteRecord, &suggestionDoc)
			if err2 != nil {
				log.Errorln(logTag, ":", err2)
				return false, err2
			}
			// remove custom events key
			delete(suggestionDoc, "customEvents")
			// remove ip key
			delete(suggestionDoc, "ip")
			// remove user_id key
			delete(suggestionDoc, "user_id")

			// Create the index body
			indexBody := map[string]interface{}{
				"index": map[string]interface{}{
					"_index": timestampedIndex,
					"_id":    filteredSuggestion.ID,
				},
			}
			indexBodyAsStr, marshalErr := json.Marshal(indexBody)
			if marshalErr != nil {
				log.Errorln(logTag, ": error while marshalling index body, ", marshalErr)
				return false, marshalErr
			}

			docBodyMarshalled, docBodyMarshalErr := json.Marshal(suggestionDoc)
			if docBodyMarshalErr != nil {
				log.Errorln(logTag, ": error while marshalling suggestion doc in bulk request, ", docBodyMarshalErr)
				return false, docBodyMarshalErr
			}

			bulkRequestEachArr = append(bulkRequestEachArr, string(indexBodyAsStr))
			bulkRequestEachArr = append(bulkRequestEachArr, string(docBodyMarshalled))

			br := es7.NewBulkIndexRequest().
				Index(timestampedIndex).
				Id(filteredSuggestion.ID).
				Doc(suggestionDoc)
			bulkRequestEs.Add(br)
		}

		// Execute bulk request
		if len(filteredSuggestions) != 0 {
			// Join the bulk request array using `\n`
			bulkRequestStr := strings.Join(bulkRequestEachArr, "\n")
			bulkReqResponse, bulkRequestErr := zc.MakeRequest("es/_bulk", http.MethodPost, []byte(bulkRequestStr), nil)
			if bulkRequestErr != nil {
				log.Errorln(logTag, ": error while sending bulk request to Zinc, ", bulkRequestErr)
				return false, bulkRequestErr
			}

			// Check the status code
			log.Debugln(logTag, ": bulk request endpoint status code: ", bulkReqResponse.StatusCode)
			if bulkReqResponse.StatusCode != http.StatusOK {
				errMsg := fmt.Sprint("bulk request to Zinc returned a non OK status code: ", bulkReqResponse.StatusCode)
				log.Errorln(logTag, ": ", errMsg)
				return false, errors.New(errMsg)
			}

			// Make the bulk call to ES as well
			_, err3 := bulkRequestEs.Do(ctx)
			if err3 != nil {
				log.Warnln(logTag, ": error executing suggestions bulk request for ES:", err3)

				// No need to stop execution as this is a backup sync for better
				// usability.
				return true, nil
			}
		}
	}
	return true, nil
}
