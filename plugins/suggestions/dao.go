package suggestions

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/appbaseio-confidential/reactivesearch/plugins/analytics"
	"github.com/appbaseio-confidential/reactivesearch/plugins/openai"
	"github.com/kr/pretty"
	log "github.com/sirupsen/logrus"

	"github.com/appbaseio-confidential/reactivesearch/plugins/querytranslate"
	"github.com/appbaseio-confidential/reactivesearch/util"
	es7 "github.com/olivere/elastic/v7"
)

type elasticsearch struct {
	indexSuffix    string
	indexConfigEs6 string
	indexConfigEs7 string
}

// Custom external suggestions
type ExternalSuggestion struct {
	Key          string                   `json:"key"`
	Count        *int64                   `json:"count"`
	Indices      []string                 `json:"indices"`
	Meta         map[string]interface{}   `json:"meta,omitempty"`
	CustomEvents map[string][]interface{} `json:"customEvents,omitempty"`
}

type Suggestion struct {
	ID           string                   `json:"id"`
	Key          string                   `json:"key"`
	Count        *int64                   `json:"count"`
	Indices      []string                 `json:"indices"`
	Meta         map[string]interface{}   `json:"meta,omitempty"`
	QueryLength  *int                     `json:"search_characters_length,omitempty"`
	CustomEvents map[string][]interface{} `json:"customEvents,omitempty"`
}

// createSuggestionsIndex creates a suggestions index with the provided name if it doesn't exist
func createSuggestionsIndex(indexWithSuffix, indexConfigEs6, indexConfigEs7 string) (*elasticsearch, bool, error) {
	ctx := context.Background()

	es := &elasticsearch{indexWithSuffix, indexConfigEs6, indexConfigEs7}

	// Check if the index already exists
	exists, err := util.GetClient7().IndexExists(indexWithSuffix).Do(ctx)
	if err != nil {
		return nil, false, fmt.Errorf("error while checking if index already exists: %v", err)
	}
	if exists {
		log.Println(logTag, ": index named", indexWithSuffix, "already exists, skipping...")
		return es, true, nil
	}

	replicas := util.GetReplicas()
	var indexConfig string
	switch util.GetVersion() {
	case 6:
		indexConfig = indexConfigEs6
	default:
		indexConfig = indexConfigEs7
	}
	settings := fmt.Sprintf(indexConfig, util.HiddenIndexSettings(), replicas)

	// index does not exists, create a new one
	_, err = util.GetClient7().CreateIndex(indexWithSuffix).Body(settings).Do(ctx)
	if err != nil {
		return nil, false, fmt.Errorf("error while creating index named %s: %v", indexWithSuffix, err)
	}

	log.Println(logTag, ": successfully created index named", indexWithSuffix)
	return es, false, nil
}

// createSuggestionsIndexZinc creates a suggestions index with the
// provided name in Zinc.
func createSuggestionsIndexZinc(indexWithSuffix string) (*util.ZincClient, bool, error) {
	zincClient := util.GetZincClient()

	// Check if the index already exists
	// Make a request to the get settings endpoint of Zinc
	// and check if the status code is 200 to know if it exists
	// or not.
	existsEndpointZinc := fmt.Sprintf("api/%s/_settings", indexWithSuffix)
	existsResponse, existsResponseErr := zincClient.MakeRequest(existsEndpointZinc, http.MethodGet, []byte(""), nil)

	if existsResponseErr != nil {
		return nil, false, fmt.Errorf("error while checking if index already exists: %v", existsResponseErr)
	}

	if existsResponse == nil || existsResponse.StatusCode == http.StatusOK {
		log.Infoln(logTag, ": index named", indexWithSuffix, "already exists, skipping...")
		return zincClient, true, nil
	}

	indexCreateBody := fmt.Sprintf(indexConfigZinc, indexWithSuffix)
	log.Debugln(logTag, ": zinc create body: ", indexCreateBody)

	// Send a create request for the index
	// with the mapping and name of the index present in the body
	indexCreateResponse, indexCreateErr := zincClient.MakeRequest("api/index", http.MethodPost, []byte(indexCreateBody), nil)

	if indexCreateErr != nil {
		return nil, false, fmt.Errorf("error while creating index named: %s, %v", indexWithSuffix, indexCreateErr)
	}

	// Check status code and handle errors accordingly, if any
	log.Debugln(logTag, "index create status code returned is: ", indexCreateResponse.StatusCode)
	if indexCreateResponse.StatusCode != http.StatusOK {
		useBody := false
		body, readErr := ioutil.ReadAll(indexCreateResponse.Body)
		if readErr == nil {
			useBody = true
		}
		errMsg := fmt.Sprintf("non OK status code received while creating index named `%s` with status code: %d", indexWithSuffix, indexCreateResponse.StatusCode)
		if useBody {
			errMsg += fmt.Sprintf(" and message: %s", string(body))
		}
		log.Warnln(logTag, ": ", errMsg)
		return nil, false, fmt.Errorf(errMsg)
	}

	log.Println(logTag, ": successfully created index named", indexWithSuffix)
	return zincClient, false, nil
}

// Create or update the popular suggestions preferences
func (es *elasticsearch) savePopularSuggestionsPreferences(ctx context.Context, record PopularPreferences) (*es7.IndexResponse, error) {
	// Validate external suggestions if present
	if record.ExternalSuggestions != nil && len(record.ExternalSuggestions) != 0 {
		for _, element := range record.ExternalSuggestions {
			if element.Key == "" {
				return nil, errors.New("key value can not be empty in external_suggestions")
			}
			if element.Count == nil {
				return nil, errors.New("count value can not be empty in external_suggestions")
			}
		}
	}
	if record.MinCount < 0 || record.MinCount > 1000 {
		return nil, errors.New("minimum count must be in range between 0 to 1000")
	}

	if record.NumberOfDays < 1 || record.NumberOfDays > 365 {
		return nil, fmt.Errorf("number of days must be in range between 1 to 365, current is %d", record.NumberOfDays)
	}

	if record.MinHits < 0 {
		return nil, errors.New("minimum hits must be greater than 0")
	}

	if record.MinChars < 0 {
		return nil, errors.New("minimum characters must be greater than 0")
	}

	return util.GetClient7().Index().
		Refresh("wait_for").
		Index(es.indexSuffix).
		Id(popularPreferenceDocID).
		BodyJson(record).
		Do(ctx)
}

// Update the index suggestions preferences
func (es *elasticsearch) saveIndexSuggestionsPreferences(ctx context.Context, record IndexPreferences) (*es7.IndexResponse, error) {
	if record.Size != nil {
		if *record.Size <= 0 {
			return nil, errors.New("size must be greater than 0")
		}
		if *record.Size > 100 {
			return nil, errors.New("size must be less than 100")
		}
	}

	if record.MaxPredictedWords != nil {
		if *record.MaxPredictedWords <= 0 {
			return nil, errors.New("maxPredictedWords must be greater than 0")
		}
		if *record.MaxPredictedWords > 5 {
			return nil, errors.New("maxPredictedWords must not be greater than 5")
		}
	}

	if record.CustomQuery != nil && *record.CustomQuery == "" {
		return nil, errors.New("customQuery must not be empty")
	}

	if record.CategoryField != nil && *record.CategoryField == "" {
		return nil, errors.New("categoryField  must not be empty")
	}

	return util.GetClient7().Index().
		Refresh("wait_for").
		Index(es.indexSuffix).
		Id(indexPreferenceDocID).
		BodyJson(record).
		Do(ctx)
}

// Update the recent suggestions preferences
func (es *elasticsearch) saveRecentSuggestionsPreferences(ctx context.Context, record RecentPreferences) (*es7.IndexResponse, error) {
	if record.Size != nil {
		if *record.Size <= 0 {
			return nil, errors.New("size must be greater than 0")
		}
		if *record.Size > 100 {
			return nil, errors.New("size must be less than 100")
		}
	}
	if record.MinHits != nil && *record.MinHits < 0 {
		return nil, errors.New("minimum hits must be greater than 0")
	}

	return util.GetClient7().Index().
		Refresh("wait_for").
		Index(es.indexSuffix).
		Id(recentPreferenceDocID).
		BodyJson(record).
		Do(ctx)
}

// Sets the last time synced in suggestions
func (es *elasticsearch) updateLastSyncTime(ctx context.Context) (interface{}, error) {
	lastSyncTime := time.Now().Unix()
	record := map[string]interface{}{
		"lastSyncedTime": lastSyncTime,
	}
	var err error
	_, err = es.updateLastSyncTimeEs7(ctx, record)

	if err != nil {
		log.Errorln(logTag, ": error updating last sync time:", err)
		return false, err
	}

	// Update in-memory cache
	popularPrefences := GetPopularPreferences()
	popularPrefences.LastSyncTime = lastSyncTime
	SetPopularPreferences(popularPrefences)

	return true, nil
}

// Sets the last time synced in suggestions for Zinc
func (es *elasticsearch) updateLastSyncTimeZinc(zc *util.ZincClient) (interface{}, error) {
	lastSyncTime := time.Now().Unix()
	record := map[string]interface{}{
		"lastSyncedTime": lastSyncTime,
	}

	endpointToUpdateDoc := fmt.Sprintf("/es/%s/_doc/%s", es.indexSuffix, popularPreferenceDocID)

	recordMarshalled, recordMarshalErr := json.Marshal(record)
	if recordMarshalErr != nil {
		log.Errorln(logTag, ": error marshalling last sync time body, ", recordMarshalErr)
		return false, recordMarshalErr
	}

	updateResponse, updateErr := zc.MakeRequest(endpointToUpdateDoc, http.MethodPut, recordMarshalled, nil)
	if updateErr != nil {
		log.Warnln(logTag, ": error while updating last sync time, ", updateErr)
		return false, updateErr
	}

	// TODO: Check updateResponse status code as well
	log.Debugln(logTag, ": update response for last synced time status code: ", updateResponse.StatusCode)

	// Update in-memory cache
	popularPrefences := GetPopularPreferences()
	popularPrefences.LastSyncTime = lastSyncTime
	SetPopularPreferences(popularPrefences)

	return true, nil
}

// Get saved preferences for popular suggestions
func (es *elasticsearch) getPopularSuggestionsPreferences(ctx context.Context) (PopularPreferences, error) {
	var record PopularPreferences
	response, err := util.GetClient7().Get().
		Index(es.indexSuffix).
		Id(popularPreferenceDocID).
		Do(ctx)
	if err != nil {
		if es7.IsNotFound(err) {
			return PopularPreferences{}, nil
		}
		log.Errorln(logTag, ": error retriving popular suggestions preferences", err)
		return record, err
	}
	err = json.Unmarshal(response.Source, &record)
	if err != nil {
		log.Errorln(logTag, ": error un-marshalling popular suggestions preferences", err)
		return record, err
	}
	return record, nil
}

// Returns the custom events from analytics index
func getCustomEvents() ([]string, error) {
	var customEvents []string
	var indexName = ".analytics"
	// Fetch analytics mapping to find the custom events
	response, err := util.GetIndexMapping(indexName, context.Background())
	if err != nil {
		log.Errorln(logTag, ":", err)
		return customEvents, err
	}
	customEventsMap := make(map[string]interface{})
	for _, value := range response {
		mappings, ok := value.(map[string]interface{})
		if ok {
			var properties map[string]interface{}
			if mappings != nil && mappings["mappings"] != nil {
				switch util.GetVersion() {
				case 6:
					if mappings["mappings"].(map[string]interface{})["_doc"] != nil && mappings["mappings"].(map[string]interface{})["_doc"].(map[string]interface{})["properties"] != nil {
						properties = mappings["mappings"].(map[string]interface{})["_doc"].(map[string]interface{})["properties"].(map[string]interface{})
					}
				default:
					if mappings["mappings"].(map[string]interface{})["properties"] != nil {
						properties = mappings["mappings"].(map[string]interface{})["properties"].(map[string]interface{})
					}
				}
			}
			for field := range properties {
				if strings.HasPrefix(field, analytics.CustomEventsPrefix) {
					customEventsMap[field] = field
				}
			}
		}
	}
	for field := range customEventsMap {
		customEvents = append(customEvents, field)
	}
	// Add Pre-defined term filters
	for _, field := range analytics.PreDefinedTermFilters {
		customEvents = append(customEvents, field)
	}
	return customEvents, nil
}

// Get saved preferences for index suggestions
func (es *elasticsearch) getIndexSuggestionsPreferences(ctx context.Context) (IndexPreferences, error) {
	var record IndexPreferences
	response, err := util.GetClient7().Get().
		Index(es.indexSuffix).
		Id(indexPreferenceDocID).
		Do(ctx)
	if err != nil {
		if es7.IsNotFound(err) {
			return IndexPreferences{}, nil
		}
		log.Errorln(logTag, ": error retriving index suggestions preferences", err)
		return record, err
	}
	err = json.Unmarshal(response.Source, &record)
	if err != nil {
		log.Errorln(logTag, ": error un-marshalling index suggestions preferences", err)
		return record, err
	}

	return record, nil
}

func applyCustomEventsEs7(query *es7.BoolQuery, filters map[string]interface{}) {
	if len(filters) > 0 {
		var filterQueries []es7.Query
		for filter, value := range filters {
			field := analytics.AddEventPrefix(filter) + ".keyword"
			if analytics.IsPreDefinedTermFilter(filter) {
				// Don't add prefix for filters like `user_id` or `ip`
				field = filter + ".keyword"
			}
			query := es7.NewTermQuery(field, value)
			filterQueries = append(filterQueries, query)
		}
		query = query.Must(filterQueries...)
	}
}

func applyCustomEventsZinc(mustArray []interface{}, filters map[string]interface{}) []interface{} {
	if len(filters) > 0 {
		var filterQueries = make([]interface{}, 0)
		for filter, value := range filters {
			field := analytics.AddEventPrefix(filter)
			if analytics.IsPreDefinedTermFilter(filter) {
				// Don't add prefix for filters like `user_id` or `ip`
				field = filter
			}
			query := map[string]interface{}{
				"term": map[string]interface{}{
					field: value,
				},
			}
			filterQueries = append(filterQueries, query)
		}
		mustArray = append(mustArray, filterQueries...)
	}

	return mustArray
}

// Get saved preferences for recent suggestions
func (es *elasticsearch) getRecentSuggestionsPreferences(ctx context.Context) (RecentPreferences, error) {
	var record RecentPreferences
	response, err := util.GetClient7().Get().
		Index(es.indexSuffix).
		Id(recentPreferenceDocID).
		Do(ctx)
	if err != nil {
		if es7.IsNotFound(err) {
			return RecentPreferences{}, nil
		}
		log.Errorln(logTag, ": error retriving recent suggestions preferences", err)
		return record, err
	}
	err = json.Unmarshal(response.Source, &record)
	if err != nil {
		log.Errorln(logTag, ": error un-marshalling recent suggestions preferences", err)
		return record, err
	}
	return record, nil
}

// sets alias for timestamped index to .suggestions index
// removes any previous indexes aliased to .suggestions index
func (es *elasticsearch) setAlias(ctx context.Context, originalIndex, currentDayIndex string) (interface{}, error) {

	var actions []interface{}
	body := make(map[string]interface{})

	// Add an action to add an alias to the .suggestions index based on the current day's timestamp index
	addAction := make(map[string]interface{})
	addAction["add"] = map[string]interface{}{
		"alias": originalIndex,
		"index": currentDayIndex,
	}
	actions = append(actions, addAction)

	// Check if the previous indices exists
	aliasResult, err := util.GetClient7().Aliases().Do(ctx)
	if err != nil {
		log.Errorln(logTag, ": error while getting aliases:", err)
		return false, err
	}
	previousIndices := aliasResult.IndicesByAlias(originalIndex)
	// Remove the current day index
	previousIndices = remove(previousIndices, currentDayIndex)
	previousIndicesExists := len(previousIndices) > 0

	// If any previous index exists, Add an action to remove the saved alias from the .suggestions index
	if previousIndicesExists {
		removeAction := make(map[string]interface{})
		removeAction["remove"] = map[string]interface{}{
			"alias":   originalIndex,
			"indices": previousIndices,
		}
		actions = append(actions, removeAction)
	}

	body["actions"] = actions
	opt := es7.PerformRequestOptions{
		Method:      "POST",
		Path:        "/_aliases",
		Body:        body,
		ContentType: "application/json",
	}
	// Swap alias
	_, err1 := util.GetClient7().PerformRequest(ctx, opt)
	if err1 != nil {
		log.Errorln(logTag, ": error while updating alias:", err)
		return false, err1
	}
	// Delete the old existing indices
	if previousIndicesExists {
		indicesToDelete := strings.Join(previousIndices[:], ",")
		_, err3 := util.GetClient7().DeleteIndex(indicesToDelete).Do(ctx)
		if err3 != nil {
			log.Errorln(logTag, ": error while deleting", indicesToDelete, " index", err3)
			return false, err3
		}
	}

	return true, nil
}

func getSuggestionsIndex() string {
	suggestionsIndex := os.Getenv(envSuggestionsEsIndex)
	if suggestionsIndex == "" {
		suggestionsIndex = defaultSuggestionsEsIndex
	}
	return suggestionsIndex
}

func syncAnalyticsToSuggestionsZinc(s *suggestions, indexToUse string) (interface{}, error) {
	// Only sync for users having a valid plan
	if util.ValidatePlans(validPlans, util.GetFeatureSuggestions()) {
		context := context.Background()
		suggestionsIndex := getSuggestionsIndex()

		// If indexToUse is not passed then create the timestamped index
		indexWithTimeStamp := indexToUse

		if indexWithTimeStamp == "" {
			indexWithTimeStamp = getTimestampedIndex(suggestionsIndex, time.Now())
		}

		var err error
		var exists bool

		zc := new(util.ZincClient)

		zc, exists, err = createSuggestionsIndexZinc(indexWithTimeStamp)
		if err != nil {
			log.Errorln(logTag, ": error creating suggestions index, ", indexWithTimeStamp, ":", err)
			return false, err
		}
		if exists {
			// get a future timestamp to populate in a fresh index
			// using 72h to avoid name clash(es) with tomorrow's index
			// 72h index will be reclaimed either tomorrow or when next time the popular suggestions preferences are saved again
			timeFuture := time.Now().Add(72 * time.Hour)
			indexWithTimeStamp = getTimestampedIndex(suggestionsIndex, timeFuture)
			zc, exists, err = createSuggestionsIndexZinc(indexWithTimeStamp)
			if err != nil {
				log.Errorln(logTag, ": error creating a future timestamp suggestions index, ", indexWithTimeStamp, ":", err)
				return false, err
			}
		}

		s.es = &elasticsearch{indexWithTimeStamp, indexConfigEs6, indexConfigEs7}
		// Get the current popular preferences from cache
		currentPreferences := GetPopularPreferences()

		// a fresh popular suggestions index should now reasonably exist
		if !exists {
			// Step2: Populate index with analytics results + Apply External Suggestions
			log.Debug(logTag, ": Populating popular suggestions index in zinc")
			s.es.populateTimeStampedIndexZinc(context, indexWithTimeStamp, zc)

			// Step3: Add an alias with time-stamped index to the main suggestions(.suggestions) index
			log.Debug(logTag, ": aliased index is: ", indexWithTimeStamp)
			currentPreferences.AliasToIndex = indexWithTimeStamp

			// Update ES and cache of all nodes
			_, updateESErr := s.esMeta.savePopularSuggestionsPreferences(context, currentPreferences)
			if updateESErr != nil {
				log.Errorln(logTag, ": error while updating popular preferences, ", indexWithTimeStamp, ":", updateESErr)
				return false, updateESErr
			}

			// Update cache in all nodes
			SetPopularPreferences(currentPreferences)

			// Update all other nodes if necessary
			if util.ShouldProxyToACCAPI() {
				marshalledRequestBody, err5 := json.Marshal(currentPreferences)
				if err5 != nil {
					return false, fmt.Errorf("error while marshalling preference body to save alias, %v", err5)
				}
				var bodyJSON map[string]interface{}
				err6 := json.Unmarshal(marshalledRequestBody, &bodyJSON)
				if err6 != nil {
					return false, fmt.Errorf("error while unmarshalling preference body to save alias, %v", err6)
				}

				res, err := util.ProxyACCAPI(util.ProxyConfig{
					Method: http.MethodPut,
					URL:    "/_popular_suggestions/preferences",
					Body:   bodyJSON, // forward body
				})

				if res != nil || err != nil {
					return false, fmt.Errorf("error while updating other nodes with updated alias name, got response code: %d, and err: %v", res.StatusCode, err)
				}
			}

			// Make the following request with Zinc
			// Update last synced time with zinc
			s.esMeta.updateLastSyncTime(context)
		} else if currentPreferences.AliasToIndex == "" {
			// Update the aliasToIndex value with the indexTimestamp value
			currentPreferences.AliasToIndex = indexWithTimeStamp

			// Update ES and cache of all nodes
			_, updateESErr := s.esMeta.savePopularSuggestionsPreferences(context, currentPreferences)
			if updateESErr != nil {
				log.Errorln(logTag, ": error while updating popular preferences, ", indexWithTimeStamp, ":", updateESErr)
				return false, updateESErr
			}

			// Update cache in all nodes
			SetPopularPreferences(currentPreferences)

			// Update all other nodes if necessary
			if util.ShouldProxyToACCAPI() {
				marshalledRequestBody, err5 := json.Marshal(currentPreferences)
				if err5 != nil {
					return false, fmt.Errorf("error while marshalling preference body to save alias, %v", err5)
				}
				var bodyJSON map[string]interface{}
				err6 := json.Unmarshal(marshalledRequestBody, &bodyJSON)
				if err6 != nil {
					return false, fmt.Errorf("error while unmarshalling preference body to save alias, %v", err6)
				}

				res, err := util.ProxyACCAPI(util.ProxyConfig{
					Method: http.MethodPut,
					URL:    "/_popular_suggestions/preferences",
					Body:   bodyJSON, // forward body
				})

				if res != nil || err != nil {
					return false, fmt.Errorf("error while updating other nodes with updated alias name, got response code: %d, and err: %v", res.StatusCode, err)
				}
			}
		} else {
			log.Println(logTag, ": Query suggestions are already in sync, skipping...")
		}
	}
	return true, nil
}

type Error struct {
	Code  int
	Error error
}

// GetIndexFilterQueryZinc apply the index filtering logic
func GetIndexFilterQueryZinc(query []interface{}, indices ...string) []interface{} {
	if indices != nil && len(indices) > 0 {
		var indexQueries = make([]interface{}, 0)
		for _, index := range indices {
			indexQueries = append(indexQueries, map[string]interface{}{
				"match": map[string]interface{}{
					"indices": index,
				},
			})
		}
		query = append(query, indexQueries...)
	}
	return query
}

// To get popular suggestions
func GetPopularSuggestions(config querytranslate.PopularSuggestionsOptions, value string, indices []string) ([]querytranslate.SuggestionHIT, *Error) {
	var suggestions = make([]querytranslate.SuggestionHIT, 0)
	if util.ValidatePlans(validPlans, util.GetFeatureSuggestions()) {
		// Consider the size if passed
		size := 5
		if config.Size != nil && *config.Size != 0 {
			size = *config.Size
		}

		// If index is passed, use it instead of default indices
		if config.Index != nil {
			indices = strings.Split(*config.Index, ",")
		}

		// Get the index aliased
		popularPreferences := GetPopularPreferences()
		log.Debug(logTag, ": popular preferences: ", pretty.Formatter(popularPreferences))
		aliasedIndex := popularPreferences.AliasToIndex

		if aliasedIndex == "" {
			errMsg := "error while getting aliased index name from preferences"
			log.Warnln(logTag, ": ", errMsg)
			return suggestions, &Error{Code: http.StatusInternalServerError, Error: fmt.Errorf(errMsg)}
		}

		// Create the query as a map
		//
		// query should be passed in the `query` key in the final
		// body sent to Zinc
		query := make(map[string]interface{})

		boolQuery := make(map[string]interface{})
		mustQuery := make([]interface{}, 0)
		filterQuery := make([]interface{}, 0)

		// Add the value inside the mustQuery if present
		if value != "" {
			mustQuery = append(mustQuery, map[string]interface{}{"prefix": map[string]interface{}{
				"key": value,
			}})
		} else {
			mustQuery = append(mustQuery, map[string]interface{}{"match_all": map[string]interface{}{}})
		}

		// Consider the minCount value
		if config.MinCount != nil {
			mustQuery = append(mustQuery, map[string]interface{}{"range": map[string]interface{}{
				"count": map[string]interface{}{
					"gte": *config.MinCount,
				},
			}})
		}

		// Consider the MinChars
		if config.MinChars != nil {
			filterQuery = append(filterQuery, map[string]interface{}{"range": map[string]interface{}{
				"search_characters_length": map[string]interface{}{
					"gte": *config.MinChars,
				},
			}})
		}

		if config.ShowGlobal == nil || !*config.ShowGlobal {
			// Avoid filtering when select all pattern is present
			// Filter uses the term query which would yield no results for `*`
			if !util.Contains(indices, "*") {
				mustQuery = GetIndexFilterQueryZinc(mustQuery, indices...)
			}
		}

		// Filter by custom events
		mustQuery = applyCustomEventsZinc(mustQuery, config.CustomEvents)

		if len(filterQuery) > 0 {
			boolQuery["filter"] = filterQuery
		}

		boolQuery["must"] = mustQuery
		query["bool"] = boolQuery

		finalBodyToSend := map[string]interface{}{
			"query": query,
			"size":  size,
		}

		bodyAsString, marshalErr := json.Marshal(finalBodyToSend)
		if marshalErr != nil {
			log.Warnln(logTag, ": error while marshaling search source, ", marshalErr)
			return suggestions, &Error{Code: http.StatusInternalServerError, Error: marshalErr}
		}

		log.Debugln(logTag, ": zinc body being sent is: ", pretty.Formatter(string(bodyAsString)))

		zincEndpointToHit := fmt.Sprintf("es/%s/_search", aliasedIndex)
		log.Debugln(logTag, ": zinc endpoint: ", zincEndpointToHit)

		zc := util.GetZincClient()

		searchResponse, searchErr := zc.MakeRequest(zincEndpointToHit, http.MethodPost, bodyAsString, nil)

		if searchErr != nil {
			errMsg := fmt.Sprint("error while hitting zinc to get popular suggestions, ", searchErr)
			log.Warnln(logTag, ": ", errMsg)
			return suggestions, &Error{Code: http.StatusInternalServerError, Error: fmt.Errorf(errMsg)}
		}

		// Return error if status code is not 200
		if searchResponse.StatusCode != http.StatusOK {
			errMsg := fmt.Sprint("Zinc returned a non OK status code: ", searchResponse.StatusCode)
			log.Warnln(logTag, ": ", errMsg)
			return suggestions, &Error{Code: http.StatusInternalServerError, Error: fmt.Errorf(errMsg)}
		}

		res := new(es7.SearchResult)
		tempMap := make(map[string]interface{})

		defer searchResponse.Body.Close()
		rawSearchBody, readErr := io.ReadAll(searchResponse.Body)

		if readErr != nil {
			log.Warnln(logTag, ": ", readErr)
			return suggestions, &Error{Code: http.StatusInternalServerError, Error: readErr}
		}

		// Marshal into a map and make sure error is not an empty string, if so then
		// just set it to nil.
		//
		// This is necessary because ES returns errors as nil instead of empty string.
		unmarshalToMapErr := json.Unmarshal(rawSearchBody, &tempMap)

		if unmarshalToMapErr != nil {
			errMsg := fmt.Sprint("error while unmarshalling Zinc response to map to modify it, ", unmarshalToMapErr)
			log.Errorln(logTag, ": ", errMsg)
			return suggestions, &Error{Code: http.StatusInternalServerError, Error: fmt.Errorf(errMsg)}
		}

		if tempMap["error"] != nil {
			errorAsStr, errorAsStrOk := tempMap["error"].(string)
			if errorAsStrOk && errorAsStr == "" {
				tempMap["error"] = nil
			}
		}

		// Marshal the map back
		finalMapToUnmarshal, marshalErr := json.Marshal(tempMap)
		if marshalErr != nil {
			errMsg := fmt.Sprint("error while marshalling modified response to unmarshal into ES searchResult, ", marshalErr)
			log.Errorln(logTag, ": ", errMsg)
			return suggestions, &Error{Code: http.StatusInternalServerError, Error: fmt.Errorf(errMsg)}
		}

		log.Debugln(logTag, ": response code recieved: ", searchResponse.StatusCode)
		log.Debugln(logTag, ": response recieved from zinc is: ", pretty.Formatter(string(finalMapToUnmarshal)))

		unmarshalErr := json.Unmarshal(finalMapToUnmarshal, &res)

		if unmarshalErr != nil {
			errMsg := fmt.Sprint("error while unmarshalling Zinc response into ES hit body, ", unmarshalErr)
			log.Warnln(logTag, ": ", errMsg)
			return suggestions, &Error{Code: http.StatusInternalServerError, Error: fmt.Errorf(errMsg)}
		}

		for _, v := range res.Hits.Hits {
			var source map[string]interface{}
			err := json.Unmarshal(v.Source, &source)
			if err != nil {
				log.Errorln(logTag, ":", err.Error())
				return suggestions, &Error{Code: http.StatusInternalServerError, Error: err}
			}
			var count int
			if source["count"] != nil {
				countFloat, ok := source["count"].(float64)
				if ok {
					count = int(countFloat)
				}
			}
			var key string
			if source["key"] != nil {
				keyString, ok := source["key"].(string)
				if ok {
					key = keyString
				}
			}
			if key != "" {
				var score float64
				if v.Score != nil {
					score = *v.Score
				}
				sectionId := "popular"
				suggestions = append(suggestions, querytranslate.SuggestionHIT{
					Label:        key,
					Value:        key,
					Count:        &count,
					Type:         querytranslate.Popular,
					SectionLabel: config.SectionLabel,
					SectionId:    &sectionId,
					// ES response properties
					Id:     v.Id,
					Index:  &v.Index,
					Score:  score,
					Source: source,
				})
			}
		}
		return suggestions, nil
	}
	msg := "Popular suggestions feature is not available for the free plan users, please upgrade to a paid plan or set `enablePopularSuggestions` to `false`."
	if util.GetTier() != nil {
		msg = "Popular suggestions is not available for the " + util.GetTier().String() + " plan users, please upgrade to a higher plan or set `enablePopularSuggestions` to `false`."
	}
	return suggestions, &Error{Code: http.StatusPaymentRequired, Error: errors.New(msg)}
}

// GetFAQSuggestions will return the FAQ suggestions based on the passed
// value
func GetFAQSuggestions(config querytranslate.FAQSuggestionsOptions, value string, searchboxId *string) ([]querytranslate.SuggestionHIT, *Error) {
	// We only allow this suggestion for users that have a valid OpenAI plan, so
	// we will return empty hits otherwise
	if !util.ValidatePlans(openai.GetValidOpenAIPlans(), util.GetFeatureOpenAI()) {
		log.Info(logTag, ": skipping FAQ suggestions since the plan does not allow it!")
		return make([]querytranslate.SuggestionHIT, 0), nil
	}

	// If searchboxId is nil or empty, return 0 results
	if searchboxId == nil || strings.TrimSpace(*searchboxId) == "" {
		errMsg := fmt.Sprint("`searchboxId` needs to be valid")
		return make([]querytranslate.SuggestionHIT, 0), &Error{
			Error: fmt.Errorf(errMsg),
			Code:  http.StatusBadRequest,
		}
	}

	zincClient := util.GetZincClient()

	// If size is specified in the config, use it
	size := 5
	if config.Size != nil && *config.Size != 0 {
		size = *config.Size
	}

	var suggestions = make([]querytranslate.SuggestionHIT, 0)

	mustQuery := make([]interface{}, 0)

	// Add the main match query
	if value != "" {
		mustQuery = append(mustQuery, map[string]interface{}{
			"match": map[string]interface{}{
				"question": value,
			},
		})
	} else {
		mustQuery = append(mustQuery, map[string]interface{}{
			"match_all": map[string]interface{}{},
		})
	}

	shouldQuery := make([]interface{}, 0)

	// Inject the searchbox filter
	shouldQuery = append(shouldQuery, map[string]interface{}{
		"term": map[string]interface{}{
			"searchboxId.keyword": searchboxId,
		},
	})

	// Put the should query inside the must
	mustQuery = append(mustQuery, map[string]interface{}{
		"bool": map[string]interface{}{
			"should": shouldQuery,
		},
	})

	query := map[string]interface{}{
		"query": map[string]interface{}{
			"bool": map[string]interface{}{
				"must": mustQuery,
			},
		},
		"size": size,
	}

	// If sectionLabel is not passed, set default sectionLabel
	if config.SectionLabel == nil {
		sectionLabel := "<b>FAQs</b>"
		config.SectionLabel = &sectionLabel
	}

	// Marshal the body into bytes
	queryInBytes, marshalErr := json.Marshal(query)
	if marshalErr != nil {
		errMsg := fmt.Sprint("error while marshalling query into bytes: ", marshalErr.Error())
		return suggestions, &Error{
			Error: errors.New(errMsg),
			Code:  http.StatusInternalServerError,
		}
	}

	searchURL := fmt.Sprintf("/es/%s/_search", ".ai_faqs")
	searchResponse, searchErr := zincClient.MakeRequest(searchURL, http.MethodPost, queryInBytes, nil)

	if searchErr != nil {
		errMsg := fmt.Sprint("Error while making search request to Zinc: ", searchErr.Error())
		return suggestions, &Error{
			Error: errors.New(errMsg),
		}
	}

	// If search response is not OK, we need to throw error as well
	if searchResponse.StatusCode != http.StatusOK {
		errMsg := fmt.Sprint("error while getting response from Zinc for FAQ with status code: ", searchResponse.StatusCode)
		bodyInBytes, readErr := ioutil.ReadAll(searchResponse.Body)
		if readErr == nil {
			errMsg += " " + " with body: " + string(bodyInBytes)
		}

		return suggestions, &Error{
			Error: errors.New(errMsg),
		}
	}

	// Since response was okay, read it and return it

	res := new(es7.SearchResult)
	tempMap := make(map[string]interface{})

	defer searchResponse.Body.Close()
	rawSearchBody, readErr := io.ReadAll(searchResponse.Body)

	if readErr != nil {
		log.Warnln(logTag, ": ", readErr)
		return suggestions, &Error{Code: http.StatusInternalServerError, Error: readErr}
	}

	// Marshal into a map and make sure error is not an empty string, if so then
	// just set it to nil.
	//
	// This is necessary because ES returns errors as nil instead of empty string.
	unmarshalToMapErr := json.Unmarshal(rawSearchBody, &tempMap)

	if unmarshalToMapErr != nil {
		errMsg := fmt.Sprint("error while unmarshalling Zinc response to map to modify it, ", unmarshalToMapErr)
		log.Errorln(logTag, ": ", errMsg)
		return suggestions, &Error{Code: http.StatusInternalServerError, Error: fmt.Errorf(errMsg)}
	}

	if tempMap["error"] != nil {
		errorAsStr, errorAsStrOk := tempMap["error"].(string)
		if errorAsStrOk && errorAsStr == "" {
			tempMap["error"] = nil
		}
	}

	// Marshal the map back
	finalMapToUnmarshal, marshalErr := json.Marshal(tempMap)
	if marshalErr != nil {
		errMsg := fmt.Sprint("error while marshalling modified response to unmarshal into ES searchResult, ", marshalErr)
		log.Errorln(logTag, ": ", errMsg)
		return suggestions, &Error{Code: http.StatusInternalServerError, Error: fmt.Errorf(errMsg)}
	}

	log.Debugln(logTag, ": response code recieved: ", searchResponse.StatusCode)
	log.Debugln(logTag, ": response recieved from zinc is: ", pretty.Formatter(string(finalMapToUnmarshal)))

	unmarshalErr := json.Unmarshal(finalMapToUnmarshal, &res)

	if unmarshalErr != nil {
		errMsg := fmt.Sprint("error while unmarshalling Zinc response into ES hit body, ", unmarshalErr)
		log.Warnln(logTag, ": ", errMsg)
		return suggestions, &Error{Code: http.StatusInternalServerError, Error: fmt.Errorf(errMsg)}
	}

	for _, v := range res.Hits.Hits {
		var source map[string]interface{}
		err := json.Unmarshal(v.Source, &source)
		if err != nil {
			log.Errorln(logTag, ":", err.Error())
			return suggestions, &Error{Code: http.StatusInternalServerError, Error: err}
		}

		questionAsStr, asStrOk := source["question"].(string)
		if !asStrOk {
			continue
		}

		answerAsStr, asStrOk := source["answer"].(string)
		if !asStrOk {
			continue
		}

		if questionAsStr == "" || answerAsStr == "" {
			continue
		}

		var score float64
		if v.Score != nil {
			score = *v.Score
		}
		sectionId := "faqs"
		suggestions = append(suggestions, querytranslate.SuggestionHIT{
			Label:        questionAsStr,
			Value:        strings.ToLower(questionAsStr),
			Type:         querytranslate.FAQ,
			SectionLabel: config.SectionLabel,
			SectionId:    &sectionId,
			Answer:       &answerAsStr,
			// ES response properties
			Id:     v.Id,
			Index:  &v.Index,
			Score:  score,
			Source: source,
		})
	}
	return suggestions, nil
}

// To get recent suggestions
func GetRecentSuggestions(q querytranslate.Query, config querytranslate.RecentSuggestionsOptions, indices []string) ([]querytranslate.SuggestionHIT, *Error) {
	var suggestions = make([]querytranslate.SuggestionHIT, 0)
	if util.ValidatePlans(validPlans, util.GetFeatureSuggestions()) {
		query := es7.NewBoolQuery()
		size := 5
		if config.Size != nil && *config.Size != 0 {
			size = *config.Size
		}
		if config.Index != nil {
			indices = strings.Split(*config.Index, ",")
		}

		if config.MinHits != nil {
			query = query.Must(es7.NewRangeQuery("total_hits").Gte(*config.MinHits))
		}

		if config.MinChars != nil {
			minCharQuery := es7.NewRangeQuery("search_characters_length").Gte(*config.MinChars)
			query.Filter(minCharQuery)
		}

		// filter by custom events
		applyCustomEventsEs7(query, config.CustomEvents)

		var value string
		if q.Value != nil {
			valueAsString, ok := (*q.Value).(string)
			if ok {
				value = valueAsString
			}
		}

		if value != "" {
			query.Must(
				es7.NewMultiMatchQuery(value).
					Type("phrase").
					Operator("and").Field("search_query"),
			)
		}

		// apply index filtering
		// Avoid filtering when select all pattern is present
		// Filter uses the term query which would yield no results for `*`
		if !util.Contains(indices, "*") {
			util.GetIndexFilterQueryEs7(query, indices...)
		}

		aggr := es7.NewTermsAggregation().
			Field("search_query.keyword").
			Size(size).
			OrderByAggregation("timestamp_aggs.value", false).
			SubAggregation("timestamp_aggs", es7.NewMaxAggregation().Field("timestamp"))

		result, err := util.GetClient7().Search(analytics.GetAnalyticsIndex()).
			Query(query).
			Size(0).
			Aggregation("recent_searches_aggr", aggr).
			Do(context.Background())
		if err != nil {
			return suggestions, &Error{
				Code:  http.StatusInternalServerError,
				Error: fmt.Errorf("unable to fetch recent searches response from es: %v", err),
			}
		}
		aggrResult, found := result.Aggregations.Terms("recent_searches_aggr")
		if !found {
			return suggestions, &Error{
				Code:  http.StatusInternalServerError,
				Error: fmt.Errorf("unable to fetch aggregation value from 'recent_searches_aggr'"),
			}
		}
		for _, bucket := range aggrResult.Buckets {
			key, ok := bucket.Key.(string)
			if ok && key != "" {
				count := int(bucket.DocCount)
				sectionId := "recent"
				suggestions = append(suggestions, querytranslate.SuggestionHIT{
					Label:        key,
					Value:        key,
					Count:        &count,
					Type:         querytranslate.Recent,
					SectionLabel: config.SectionLabel,
					SectionId:    &sectionId,
					// ES response properties
					Source: make(map[string]interface{}),
				})
			}
		}

		return suggestions, nil
	}
	msg := "Recent suggestions feature is not available for the free plan users, please upgrade to a paid plan or set `enableRecentSuggestions` to `false`."
	if util.GetTier() != nil {
		msg = "Recent suggestions feature is not available for the " + util.GetTier().String() + " plan users, please upgrade to a higher plan or set `enableRecentSuggestions` to `false`."
	}
	return suggestions, &Error{Code: http.StatusPaymentRequired, Error: errors.New(msg)}
}

// GenerateDocumentSuggestionsQuery will generate the document suggestions
// query based on the input values and accordingly return the built query
func GenerateDocumentSuggestionsQuery(q querytranslate.Query, value string, config querytranslate.RecentDocumentSuggestionsOptions, userId string, indices []string) (*es7.SearchService, *es7.BoolQuery) {
	// If `maxChars` is set, check it and return accordingly
	if config.MaxChars == nil {
		defaultMaxChars := 6
		config.MaxChars = &defaultMaxChars
	}

	if *config.MaxChars != 0 {
		if len(value) > *config.MaxChars {
			return nil, nil
		}
	}

	// If `value` is empty, we need to do a match_all query for the results.
	// We basically need to omit the extra filters for the query.
	mustArray := make([]es7.Query, 0)

	dfBasedQuery := make([]es7.Query, 0)

	if value != "" {
		// We also need to consider the `dataFields` passed here as we cannot
		// search otherwise.
		normalizedDfs := querytranslate.NormalizedDataFields(q.DataField, q.FieldWeights)
		if len(normalizedDfs) > 0 {
			// We can add the filters now.
			//
			// We will need to add the `source` keyword before the field
			// because we are searching on that.
			for _, nDf := range normalizedDfs {
				dfBasedQuery = append(dfBasedQuery, es7.NewMatchQuery(fmt.Sprintf("source.%s", nDf.Field), value))
			}
		}
	}

	// Add the should query for datafields
	if len(dfBasedQuery) > 0 {
		mustArray = append(mustArray, es7.NewBoolQuery().Should(dfBasedQuery...))
	}

	// If indexes are present, add them in the filter as well
	for _, index := range indices {
		mustArray = append(mustArray, es7.NewTermQuery("index.keyword", index))
	}

	// Add the filter for `userId`
	mustArray = append(mustArray, es7.NewExistsQuery(fmt.Sprintf("users.%s", userId)))

	finalQuery := es7.NewBoolQuery().Must(mustArray...)
	searchQuery := util.GetClient7().Search(analytics.GetDocumentSuggestionsIndex()).Query(finalQuery)

	// Add support for `includeFields` from the query.
	if q.IncludeFields != nil && len(*q.IncludeFields) != 0 {
		fieldsToInclude := make([]string, 0)
		for _, fieldToInclude := range *q.IncludeFields {
			fieldsToInclude = append(fieldsToInclude, fmt.Sprintf("source.%s", fieldToInclude))
		}

		// Add the other fields
		fieldsToInclude = append(fieldsToInclude, []string{
			"document_id",
			"users",
			"index",
		}...)

		searchQuery.FetchSourceContext(es7.NewFetchSourceContext(true).Include(fieldsToInclude...))
	}

	// Add the `size` filter.
	if config.Size == nil || *config.Size == 0 {
		defaultSize := 5
		config.Size = &defaultSize
	}
	searchQuery.Size(*config.Size)

	// Add the `from` filter.
	if config.From == nil || *config.From < 0 {
		defaultFrom := 0
		config.From = &defaultFrom
	}
	searchQuery.From(*config.From)

	// Add sorting based on the last access time
	sortQuery := es7.NewFieldSort(fmt.Sprintf("users.%s", userId)).Order(false).UnmappedType("long")
	searchQuery.SortBy(sortQuery)

	return searchQuery, finalQuery
}

// GetDocumentSuggestions will get the document suggestions based on the
// current user.
func GetDocumentSuggestions(q querytranslate.Query, value string, config querytranslate.RecentDocumentSuggestionsOptions, userId string, indices []string) ([]querytranslate.SuggestionHIT, *Error) {
	// Get the generated query
	searchQuery, _ := GenerateDocumentSuggestionsQuery(q, value, config, userId, indices)

	if searchQuery == nil {
		return make([]querytranslate.SuggestionHIT, 0), nil
	}

	// Parse the results and format them properly
	searchResponse, searchErr := searchQuery.Do(context.Background())
	if searchErr != nil {
		return make([]querytranslate.SuggestionHIT, 0), &Error{
			Code:  http.StatusInternalServerError,
			Error: fmt.Errorf("unable to fetch document searches response from es: %v", searchErr),
		}
	}

	suggestions := make([]querytranslate.SuggestionHIT, 0)

	for _, v := range searchResponse.Hits.Hits {
		var source map[string]interface{}
		err := json.Unmarshal(v.Source, &source)
		if err != nil {
			log.Errorln(logTag, ":", err.Error())
			return suggestions, &Error{Code: http.StatusInternalServerError, Error: err}
		}

		var score float64
		if v.Score != nil {
			score = *v.Score
		}
		sectionId := "document"

		storedSourceAsMap, asMapOk := source["source"].(map[string]interface{})
		if !asMapOk {
			log.Warnln(logTag, ": Skipping doc since source is not map")
			continue
		}

		// Parse the last accessed timestamp and inject it in the source.
		var timestamp int64 = 0
		usersAsMap, usersMapOk := source["users"].(map[string]interface{})
		if usersMapOk {
			lastAccessedTime, isPresent := usersAsMap[userId]
			if isPresent {
				lastAccessedAsFloat, asFloatOk := lastAccessedTime.(float64)
				if asFloatOk {
					timestamp = int64(lastAccessedAsFloat)
				}
			}
		}

		// Inject the timestamp into source if it is present
		if timestamp != 0 {
			storedSourceAsMap["_timestamp"] = timestamp
		}

		// Remove the index name from the ID by splitting it based on a __
		idSplitted := strings.Split(v.Id, "__")
		idWithoutIndexName := v.Id
		if len(idSplitted) > 1 {
			idWithoutIndexName = idSplitted[0]
		}

		suggestions = append(suggestions, querytranslate.SuggestionHIT{
			Value:        v.Id,
			Type:         querytranslate.Document,
			SectionLabel: config.SectionLabel,
			SectionId:    &sectionId,
			// ES response properties
			Id:     idWithoutIndexName,
			Index:  &v.Index,
			Score:  score,
			Source: storedSourceAsMap,
		})
	}
	return suggestions, nil
}
