package analytics

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/antonmedv/expr"
	"github.com/appbaseio/reactivesearch-api/plugins/analyticsrequest"
	log "github.com/sirupsen/logrus"

	"github.com/appbaseio/reactivesearch-api/middleware/classify"
	"github.com/appbaseio/reactivesearch-api/model/index"
	"github.com/appbaseio/reactivesearch-api/model/reindex"
	"github.com/appbaseio/reactivesearch-api/plugins/querytranslate"
	"github.com/appbaseio/reactivesearch-api/util"
	"github.com/appbaseio/reactivesearch-api/util/escompat"
	es7 "github.com/olivere/elastic/v7"
)

type elasticsearch struct {
	analyticsIndex     string
	logsIndex          string
	userSessionIndex   string
	insightsIndex      string
	userIndex          string
	savedSearchesIndex string
	favoritesIndex     string
	preferencesIndex   string
}

var emptyQueryLabel = "<empty_query>"

func initPlugin(analyticsAlias, logsIndex, usersIndex, userSessionIndex, insightsIndex, savedSearchesIndex, favoritesIndex, mapping, analyticsMapping, preferencesIndex, preferencesMapping string) (*elasticsearch, error) {
	ctx := context.Background()

	es := &elasticsearch{analyticsAlias, logsIndex, userSessionIndex, insightsIndex, usersIndex, savedSearchesIndex, favoritesIndex, preferencesIndex}

	// Check if alias exists instead of index and create first index if not exists with `${alias}-000001`
	res, err := util.GetClient7().Aliases().Index("_all").Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("error while checking if index already exists: %v", err)
	}
	indices := res.IndicesByAlias(analyticsAlias)
	isAnalyticsIndexExist := false
	if len(indices) > 0 {
		isAnalyticsIndexExist = true
	}

	// Check if user sessions index exists
	isUserSessionIndexExist, err1 := util.GetClient7().IndexExists(userSessionIndex).Do(ctx)
	if err1 != nil {
		return nil, fmt.Errorf("error while checking if index already exists: %v", err1)
	}
	// Check if analytics insights index exists
	isInsightsIndexExist, err1 := util.GetClient7().IndexExists(insightsIndex).Do(ctx)
	if err1 != nil {
		return nil, fmt.Errorf("error while checking if index already exists: %v", err1)
	}

	// Check if saved searches index exists
	isSavedSearchesIndexExist, err1 := util.GetClient7().IndexExists(savedSearchesIndex).Do(ctx)
	if err1 != nil {
		return nil, fmt.Errorf("error while checking if index already exists: %v", err1)
	}

	// Check if favorites index exists
	isFavoritesIndexExist, err1 := util.GetClient7().IndexExists(favoritesIndex).Do(ctx)
	if err1 != nil {
		return nil, fmt.Errorf("error while checking if index already exists: %v", err1)
	}

	// Check if preferences index exists
	isPreferencesIndexExist, prefErr := util.GetClient7().IndexExists(preferencesIndex).Do(ctx)
	if prefErr != nil {
		return nil, fmt.Errorf("error while checking if preferences index already exists: %v", prefErr)
	}

	if isAnalyticsIndexExist && isUserSessionIndexExist && isInsightsIndexExist && isSavedSearchesIndexExist && isFavoritesIndexExist && isPreferencesIndexExist {
		log.Println(logTag, ": index named", analyticsAlias, "already exists, skipping...")
		log.Println(logTag, ": index named", userSessionIndex, "already exists, skipping...")
		log.Println(logTag, ": index named", insightsIndex, "already exists, skipping...")
		log.Println(logTag, ": index named", savedSearchesIndex, "already exists, skipping...")
		log.Println(logTag, ": index named", favoritesIndex, "already exists, skipping...")
		log.Println(logTag, ": index named", preferencesIndex, "already exists, skipping...")
		return es, nil
	}

	replicas := util.GetReplicas()
	// Analytics index does not exists, create a new one
	if !isAnalyticsIndexExist {
		settings := util.AdaptIndexBody(fmt.Sprintf(analyticsMapping, analyticsAlias, getAnalyticsMappings(), util.HiddenIndexSettings(), replicas))
		analyticsIndex := analyticsAlias + `-000001`
		_, err = util.GetClient7().CreateIndex(analyticsIndex).
			Body(settings).
			Do(ctx)
		if err != nil {
			log.Errorln(logTag, " : ", fmt.Errorf("error while creating index named \"%s\" %v", analyticsIndex, err))
			isAliasExistsAsIndex, err := util.GetClient7().IndexExists(analyticsAlias).Do(ctx)
			if err != nil {
				return nil, fmt.Errorf("error while checking if index already exists: %v", err)
			}
			if !isAliasExistsAsIndex {
				return nil, fmt.Errorf("error while creating index named \"%s\" %v", analyticsIndex, err)
			}
			// If .analytics exists as an index then perform following steps:
			// 1. Re-index `.analytics` to `.analytics-000001`
			// 2. Delete ``.analytics` and continue
			sourceIndex := analyticsAlias
			destinationIndex := analyticsIndex
			var mappingAsMap map[string]interface{}
			err1 := json.Unmarshal([]byte(getAnalyticsMappings()), &mappingAsMap)
			if err1 != nil {
				log.Errorln(logTag, ":", err1)
				return nil, fmt.Errorf("error while un-marshalling analytics mappings %v", err1)
			}
			reIndexConfig := reindex.ReindexConfig{
				Mappings: mappingAsMap,
			}
			taskDetails, err := reindex.Reindex(context.Background(), sourceIndex, &reIndexConfig, false, destinationIndex)
			if err != nil {
				log.Errorln(logTag, ":", err)
				return nil, fmt.Errorf("error while re-indexing .analytics index %v", err)
			}
			// Re-index synchronously
			reindex.TrackReindex(reindex.SetAliasConfig{
				AliasName:    sourceIndex,
				NewIndex:     destinationIndex,
				OldIndex:     sourceIndex,
				IsWriteIndex: true,
			}, taskDetails)
		}
		classify.SetIndexAlias(analyticsIndex, analyticsAlias)
		classify.SetAliasIndex(analyticsAlias, analyticsIndex)
		log.Println(logTag, ": successfully created index named", analyticsAlias)
	}

	settings := util.AdaptIndexBody(fmt.Sprintf(mapping, util.HiddenIndexSettings(), replicas))

	// User session index does not exists, create a new one
	if !isUserSessionIndexExist {
		settings := util.AdaptIndexBody(fmt.Sprintf(userSessionMapping, getUserSessionMappings(), util.HiddenIndexSettings(), replicas))
		_, err = util.GetClient7().CreateIndex(userSessionIndex).Body(settings).Do(ctx)
		if err != nil {
			return nil, fmt.Errorf("error while creating index named %s: %v", userSessionIndex, err)
		}
		log.Println(logTag, ": successfully created index named", userSessionIndex)
	}

	// Analytics insights index does not exists, create a new one
	if !isInsightsIndexExist {
		_, err = util.GetClient7().CreateIndex(insightsIndex).Body(settings).Do(ctx)
		if err != nil {
			return nil, fmt.Errorf("error while creating index named %s: %v", insightsIndex, err)
		}
		log.Println(logTag, ": successfully created index named", insightsIndex)
	}

	// Analytics saved searches index does not exists, create a new one
	if !isSavedSearchesIndexExist {
		_, err = util.GetClient7().CreateIndex(savedSearchesIndex).Body(settings).Do(ctx)
		if err != nil {
			return nil, fmt.Errorf("error while creating index named %s: %v", savedSearchesIndex, err)
		}
		log.Println(logTag, ": successfully created index named", savedSearchesIndex)
	}

	// Analytics favorites index does not exists, create a new one
	if !isFavoritesIndexExist {
		_, err = util.GetClient7().CreateIndex(favoritesIndex).Body(settings).Do(ctx)
		if err != nil {
			return nil, fmt.Errorf("error while creating index named %s: %v", favoritesIndex, err)
		}
		log.Println(logTag, ": successfully created index named", favoritesIndex)
	}

	// If preferences index doesn't exist, create it.
	if !isPreferencesIndexExist {
		prefsSettings := util.AdaptIndexBody(fmt.Sprintf(preferencesMapping, util.HiddenIndexSettings(), replicas))
		_, err = util.GetClient7().CreateIndex(preferencesIndex).Body(prefsSettings).Do(ctx)
		if err != nil {
			return nil, fmt.Errorf("error while creating index named %s: %v", preferencesIndex, err)
		}
		log.Println(logTag, ": successfully created index named", preferencesIndex)
	}

	return es, nil
}

type Error struct {
	message error
	code    int
}

func (es *elasticsearch) updateConversion(ctx context.Context, docID string, record analyticsrequest.Record) *Error {
	indices := es.getSortedIndices(es.analyticsIndex)

	if len(indices) > 0 {
		// multiple indices found
		// check the presence of document
		for _, index := range indices {
			res, err := util.GetClient7().Get().Index(index).Id(docID).Do(ctx)
			if err != nil {
				if es7.IsNotFound(err) {
					continue
				}
				return &Error{
					message: err,
					code:    http.StatusInternalServerError,
				}
			}
			var analyticsRecord map[string]interface{}
			err2 := json.Unmarshal(res.Source, &analyticsRecord)
			if err2 != nil {
				return &Error{
					message: err2,
					code:    http.StatusInternalServerError,
				}
			}
			// delete the record from original index
			_, err3 := util.GetClient7().Delete().Index(index).Id(docID).Do(ctx)
			if err3 != nil {
				return &Error{
					message: err3,
					code:    http.StatusInternalServerError,
				}
			}
			// post the record to analytics alias without any change
			response, err4 := util.GetClient7().Index().
				Index(es.analyticsIndex).
				BodyJson(analyticsRecord).
				Refresh("wait_for").
				Id(docID).
				Do(ctx)
			code := http.StatusInternalServerError
			if response != nil {
				code = response.Status
			}
			if err4 != nil {
				return &Error{
					message: err4,
					code:    code,
				}
			}

			// Update record with script to apply conversion or clicks
			return es.updateRecord(ctx, docID, record)
		}
		// throw 404 error
		return &Error{
			message: errors.New("record not found for query_id " + docID),
			code:    http.StatusNotFound,
		}
	} else {
		// check for the presence of query_id in `.analytics` alias
		_, err := util.GetClient7().Get().Index(es.analyticsIndex).Id(docID).Do(ctx)
		if err != nil {
			if es7.IsNotFound(err) {
				return &Error{
					message: err,
					code:    http.StatusNotFound,
				}
			}
			return &Error{
				message: err,
				code:    http.StatusInternalServerError,
			}
		}
		return es.updateRecord(ctx, docID, record)
	}
}

func (es *elasticsearch) updateRecord(ctx context.Context, docID string, record analyticsrequest.Record) *Error {
	// Store calculated field `search_query_length`
	if record.SearchQuery != nil {
		lowerCaseQuery := strings.ToLower(*record.SearchQuery)
		// Use lowercase for query term
		record.SearchQuery = &lowerCaseQuery
		words := strings.Fields(*record.SearchQuery)
		queryLength := len(words)
		record.SearchQueryLength = &queryLength
		charsLength := len(*record.SearchQuery)
		record.QueryLength = &charsLength
	}
	normalizedRecord := getESRecord(record)
	scriptParams := map[string]interface{}{
		"impressions":                   normalizedRecord["hits_in_response"],
		"index":                         normalizedRecord["index"],
		"suggestion_click_object_ids":   normalizedRecord["suggestion_click_object_ids"],
		"result_click_object_ids":       normalizedRecord["result_click_object_ids"],
		"suggestion_click_position_ids": normalizedRecord["suggestion_click_position_ids"],
		"result_click_position_ids":     normalizedRecord["result_click_position_ids"],
		"conversion_object_ids":         normalizedRecord["conversion_object_ids"],
		"storedqueries":                 normalizedRecord["storedqueries"],
		"queryrules":                    normalizedRecord["queryrules"],
	}
	// Delete properties from map
	delete(normalizedRecord, "hits_in_response")
	delete(normalizedRecord, "index")
	delete(normalizedRecord, "suggestion_click_object_ids")
	delete(normalizedRecord, "result_click_object_ids")
	delete(normalizedRecord, "suggestion_click_position_ids")
	delete(normalizedRecord, "result_click_position_ids")
	delete(normalizedRecord, "conversion_object_ids")
	delete(normalizedRecord, "storedqueries")
	delete(normalizedRecord, "queryrules")

	/* The below script performs the following tasks.
	1. Sets the `hits_in_response` to empty array if value is `null` so array operations can work smoothly.
	2. Adds the impressions to the `hits_in_response` array.
	   2.1. Handles the `null` value of impressions params
	   2.2. Checks if an impression id is already present in `hits_in_response`
			2.2.3. If yes, then ignore the impression update otherwise add the impression.
	3. Sets the `suggestion_click_object_ids`, `suggestion_click_position_ids` and `suggestion_click_count`
		3.1 Checks if the object id is already present or not
			3.1.1 If yes, then update  `suggestion_click_object_ids` and `suggestion_click_position_ids`
			fields and increment the `suggestion_click_count`counter by no. of objects.
	4. Sets the `click` and `click_position` property in `hits_in_response` array for a particular
	impression object.
		4.1 Sets the `result_click_count` to 0 if field doesn't exist
		4.2 For each result_object_id do the following:
			4.2.1 Try to find the impression object
			4.2.2 If impression object exists
				4.2.2.1 Check if `click` value is already `true`
					4.2.2.1.1 If Yes, then ignore update
					4.2.2.1.2 If no, then set the `click` and `click_position` and update the `result_click_count`
			4.2.3 If impression object does not exist
				4.2.3.1 Add a new impression object with `id`, `index`, `click` and `click_position`
	5. Sets the `conversion` property in `hits_in_response` array for a particular impression object.
		5.1 Sets the `conversion_click_count` to 0 if field doesn't exist
		5.2 For each conversion_object_id do the following:
			5.2.1 Try to find the impression object
			5.2.2 If impression object exists
				5.2.2.1 Check if `conversion` value is already `true`
					5.2.2.1.1 If Yes, then ignore update
					5.2.2.1.2 If no, then set the `conversion` and update the `conversion_count`
			5.2.3 If impression object does not exist
				5.2.3.1 Add a new impression object with `id`, `index` and `conversion`
	*/

	script := `
		if(ctx._source.hits_in_response == null) {
			ctx._source.hits_in_response = [];
		}
		if(params.impressions != null) {
			for (int i = 0; i < params.impressions.length; i++) { 
				boolean isExist = false; 
				for (int j = 0; j < ctx._source.hits_in_response.length; j++) { 
					if(params.impressions[i].id == ctx._source.hits_in_response[j].id) { 
						isExist = true; 
					}
				}
				if (!isExist) { 
					ctx._source.hits_in_response.add(params.impressions[i]); 
				}
			}
		}
		if (params.suggestion_click_object_ids != null 
			&& params.suggestion_click_position_ids != null
		) { 
			if(ctx._source.suggestion_click_object_ids == null) { 
				ctx._source.suggestion_click_object_ids = []; 
			} 
			if(ctx._source.suggestion_click_position_ids == null) { 
				ctx._source.suggestion_click_position_ids = []; 
			} 
			for (int i = 0; i < params.suggestion_click_object_ids.length; i++) { 
				boolean isExist = false; 
				for (int j = 0; j < ctx._source.suggestion_click_object_ids.length; j++) { 
					if(params.suggestion_click_object_ids[i] == ctx._source.suggestion_click_object_ids[j]) { 
						isExist = true; 
					}
				} 
				if (!isExist) { 
					ctx._source.suggestion_click_object_ids.add(params.suggestion_click_object_ids[i]); 
					ctx._source.suggestion_click_position_ids.add(params.suggestion_click_position_ids[i]); 
					ctx._source.suggestion_click_count = ctx._source.suggestion_click_object_ids.length; 
				}
			} 
		} 
		if (params.result_click_object_ids != null && params.result_click_position_ids != null) {
			if(ctx._source.result_click_count == null) {
				ctx._source.result_click_count = 0;
			} 
			for (int i = 0; i < params.result_click_object_ids.length; i++) { 
				boolean isExist = false;
				for (int j = 0; j < ctx._source.hits_in_response.length; j++) { 
					if(params.result_click_object_ids[i] == ctx._source.hits_in_response[j].id) { 
						isExist = true;
						if (ctx._source.hits_in_response[j].click != true) {
							ctx._source.hits_in_response[j].click = true;
							ctx._source.hits_in_response[j].click_position = params.result_click_position_ids[i];
							ctx._source.result_click_count++;
						}
						break; 
					}
				}
				if (!isExist) { 
					ctx._source.hits_in_response.add([ 
						"id": params.result_click_object_ids[i],
						"index": params.index,
						"click": true,
						"click_position": params.result_click_position_ids[i]
					]);
					ctx._source.result_click_count++;
				}
			} 
		}
		if (params.conversion_object_ids != null) {
			if(ctx._source.conversion_count == null) {
				ctx._source.conversion_count = 0;
			} 
			for (int i = 0; i < params.conversion_object_ids.length; i++) { 
				boolean isExist = false;
				for (int j = 0; j < ctx._source.hits_in_response.length; j++) { 
					if(params.conversion_object_ids[i] == ctx._source.hits_in_response[j].id) { 
						isExist = true;
						if (ctx._source.hits_in_response[j].conversion != true) {
							ctx._source.hits_in_response[j].conversion = true;
							ctx._source.conversion_count++;
						}
						break; 
					}
				}
				if (!isExist) { 
					ctx._source.hits_in_response.add([ 
						"id": params.conversion_object_ids[i],
						"index": params.index,
						"conversion": true
					]);
					ctx._source.conversion_count++;
				}
			} 
		}

		if (params.storedqueries != null) { 
			if(ctx._source.storedqueries == null) { 
				ctx._source.storedqueries = []; 
			}
			for (int i = 0; i < params.storedqueries.length; i++) { 
				boolean isExist = false; 
				for (int j = 0; j < ctx._source.storedqueries.length; j++) { 
					if(params.storedqueries[i] == ctx._source.storedqueries[j]) { 
						isExist = true; 
					}
				}
				if (!isExist) { 
					ctx._source.storedqueries.add(params.storedqueries[i]); 
				}
			} 
		}

		if (params.queryrules != null) { 
			if(ctx._source.queryrules == null) { 
				ctx._source.queryrules = []; 
			}
			for (int i = 0; i < params.queryrules.length; i++) { 
				boolean isExist = false; 
				for (int j = 0; j < ctx._source.queryrules.length; j++) { 
					if(params.queryrules[i] == ctx._source.queryrules[j]) { 
						isExist = true; 
					}
				}
				if (!isExist) { 
					ctx._source.queryrules.add(params.queryrules[i]); 
				}
			} 
		}
		`
	updateConfig := UpdateConfig{
		DocID:        docID,
		Record:       normalizedRecord,
		Script:       script,
		ScriptParams: scriptParams,
	}
	return es.updateRecordEs7(ctx, updateConfig)
}

func (es *elasticsearch) updateInsightStatus(ctx context.Context, docID string, insightID InsightType, insightStatus InsightStatusRequest) error {
	scriptParams := make(map[string]interface{})
	switch insightStatus.Status {
	case Saved:
		scriptParams["saved"] = insightID.String()
	case Deleted:
		scriptParams["deleted"] = insightID.String()
	case Read:
		scriptParams["read"] = insightID.String()
	}
	script := `
		if(ctx._source.saved == null) { 
			ctx._source.saved = []
		} 
		if(ctx._source.read == null) { 
			ctx._source.read = []
		} 
		if(ctx._source.deleted == null) { 
			ctx._source.deleted = []
		} 
		if (params.saved != null) {  
			if (ctx._source.saved.indexOf(params.saved) > -1) { 
				ctx._source.saved.remove(ctx._source.saved.indexOf(params.saved)); 
			} 
			if (ctx._source.read.indexOf(params.saved) > -1) { 
				ctx._source.read.remove(ctx._source.read.indexOf(params.saved)); 
			} 
			if (ctx._source.deleted.indexOf(params.saved) > -1) { 
				ctx._source.deleted.remove(ctx._source.deleted.indexOf(params.saved));
			} 
			ctx._source.saved.add(params.saved); 
		} 
		if (params.read != null) {  
			if (ctx._source.saved.indexOf(params.read) > -1) { 
				ctx._source.saved.remove(ctx._source.saved.indexOf(params.read)); 
			} 
			if (ctx._source.read.indexOf(params.read) > -1) { 
				ctx._source.read.remove(ctx._source.read.indexOf(params.read)); 
			} 
			if (ctx._source.deleted.indexOf(params.read) > -1) { 
				ctx._source.deleted.remove(ctx._source.deleted.indexOf(params.read)); 
			} 
			ctx._source.read.add(params.read); 
		}  
		if (params.deleted != null) { 
			if (ctx._source.saved.indexOf(params.deleted) > -1) { 
				ctx._source.saved.remove(ctx._source.saved.indexOf(params.deleted)); 
			} 
			if (ctx._source.read.indexOf(params.deleted) > -1) { 
				ctx._source.read.remove(ctx._source.read.indexOf(params.deleted)); 
			} 
			if (ctx._source.deleted.indexOf(params.deleted) > -1) { 
				ctx._source.deleted.remove(ctx._source.deleted.indexOf(params.deleted)); 
			} 
			ctx._source.deleted.add(params.deleted); 
		}`
	return es.updateInsightStatusEs7(ctx, docID, script, scriptParams)
}

func (es *elasticsearch) getInsightStatus(ctx context.Context, docID string) (InsightStatusEsDoc, error) {
	return es.getInsightStatusEs7(ctx, docID)
}

func (es *elasticsearch) updateUserSession(ctx context.Context, docID string, record UserSession, customEvents map[string]interface{}, queryID string) error {
	script := `
		if(ctx._source.custom_events == null) { 
			ctx._source.custom_events = []
		}
		if (params.custom_events != null && params.query_id != null && params.query_id != "") { 
			boolean isExist = false;
			for (int i = 0; i < ctx._source.custom_events.length; i++) {
				if(ctx._source.custom_events[i].query_id == params.query_id) {
					isExist = true;
					for (entry in params.custom_events.entrySet()) {
						ctx._source.custom_events[i].put(entry.getKey(), entry.getValue());
					}
				}
			}
			if (!isExist) {
				Map customEvents = new HashMap();
				customEvents.put("query_id", params.query_id);
				for (entry in params.custom_events.entrySet()) {
					customEvents.put(entry.getKey(), entry.getValue());
				}
				ctx._source.custom_events.add(customEvents); 
			}
		}
		if (params.start_time != null) {
			ctx._source.start_time = params.start_time;
		}
		if (params.last_interaction_time != null) {
			ctx._source.last_interaction_time = params.last_interaction_time;
		}
		if (params.duration != null) {
			ctx._source.duration = params.duration;
		}
		if (params.user_id != null) {
			ctx._source.user_id = params.user_id;
		}
		if (params.ip != null) {
			ctx._source.ip = params.ip;
		}
		if (params.indices != null) {
			ctx._source.indices = params.indices;
		}
		if (params.bounce != null) {
			ctx._source.bounce = params.bounce;
		}
		if (params.timestamp != null) {
			ctx._source.timestamp = params.timestamp;
		}
		`
	params := map[string]interface{}{
		"custom_events":         customEvents,
		"start_time":            *record.StartTime,
		"last_interaction_time": *record.LastInteractionTime,
		"duration":              *record.Duration,
		"user_id":               *record.UserID,
		"ip":                    *record.IP,
		"indices":               *record.Indices,
		"bounce":                *record.Bounce,
		"timestamp":             record.TimeStamp,
		"query_id":              queryID,
	}
	return es.updateUserSessionEs7(ctx, docID, record, script, params)
}

func (es *elasticsearch) deleteOldRecords() {
	body := `{ "query": { "range": { "timestamp": { "lt": "now-30d" } } } }`
	ticker := time.NewTicker(24 * time.Hour)
	for range ticker.C {
		_, err := util.GetClient7().
			DeleteByQuery().
			Index(es.analyticsIndex).
			Body(body).
			Do(context.Background())
		if err != nil {
			log.Errorln(logTag, ": error deleting old analytics records", err)
		}
	}
}

func (es *elasticsearch) analyticsOverview(ctx context.Context, queryParams QueryParams, clickAnalytics bool, filters map[string]interface{}, indices ...string) ([]byte, error) {
	var wg sync.WaitGroup
	out := make(chan interface{})

	wg.Add(1)
	go func(out chan<- interface{}) {
		defer wg.Done()
		popularSearches, err := es.popularSearches(ctx, queryParams.From, queryParams.To, queryParams.Size, clickAnalytics, filters, indices...)
		if err != nil {
			log.Errorln(logTag, ":", err)
			out <- map[string]interface{}{
				"popular_searches": []interface{}{},
			}
		} else {
			out <- popularSearches
		}
	}(out)

	wg.Add(1)
	go func(out chan<- interface{}) {
		defer wg.Done()
		noResultsSearches, err := es.noResultSearches(ctx, queryParams.From, queryParams.To, queryParams.Size, filters, indices...)
		if err != nil {
			log.Errorln(logTag, ":", err)
			out <- map[string]interface{}{
				"no_results_searches": []interface{}{},
			}
		} else {
			out <- noResultsSearches
		}
	}(out)

	wg.Add(1)
	go func(out chan<- interface{}) {
		defer wg.Done()
		searchHistogram, err := es.searchHistogram(ctx, queryParams, filters, indices...)
		if err != nil {
			log.Errorln(logTag, ":", err)
			out <- map[string]interface{}{
				"search_volume": []interface{}{},
			}
		} else {
			out <- searchHistogram
		}
	}(out)

	go func() {
		wg.Wait()
		close(out)
	}()

	var overview []interface{}
	for result := range out {
		overview = append(overview, result)
	}

	return json.Marshal(overview)
}

func (es *elasticsearch) storedQueriesUsage(ctx context.Context, queryParams QueryParams, filters map[string]interface{}) ([]byte, error) {
	return es.storedQueriesUsageEs7(ctx, queryParams.From, queryParams.To, queryParams.Size, filters)
}

func (es *elasticsearch) queryRulesUsage(ctx context.Context, queryParams QueryParams, filters map[string]interface{}) ([]byte, error) {
	return es.queryRulesUsageEs7(ctx, queryParams.From, queryParams.To, queryParams.Size, filters)
}

func (es *elasticsearch) queryOverview(ctx context.Context, queryParams QueryParams, query string, filters map[string]interface{}, indices ...string) ([]byte, error) {
	var wg sync.WaitGroup

	type result struct {
		field string
		value interface{}
		err   error
	}
	out := make(chan result)

	wg.Add(1)
	go func(out chan<- result) {
		defer wg.Done()
		queryHistogram, err := es.queryHistogramRaw(ctx, queryParams, query, filters, indices...)
		if err != nil {
			out <- result{
				field: "histogram",
				err:   err,
			}
		} else {
			out <- result{
				field: "histogram",
				value: queryHistogram,
			}
		}
	}(out)

	wg.Add(1)
	go func(out chan<- result) {
		defer wg.Done()
		topResults, err := es.topResultsRaw(ctx, queryParams.From, queryParams.To, query, queryParams.Size, filters, indices...)
		if err != nil {
			out <- result{
				field: "top_results",
				err:   err,
			}
		} else {
			out <- result{
				field: "top_results",
				value: topResults,
			}
		}
	}(out)

	wg.Add(1)
	go func(out chan<- result) {
		defer wg.Done()
		topClicks, err := es.topClicksRaw(ctx, queryParams.From, queryParams.To, query, queryParams.Size, filters, indices...)
		if err != nil {
			out <- result{
				field: "top_clicks",
				err:   err,
			}
		} else {
			out <- result{
				field: "top_clicks",
				value: topClicks,
			}
		}
	}(out)

	go func() {
		wg.Wait()
		close(out)
	}()

	var queryVolume = make(map[string]interface{})
	for result := range out {
		queryVolume[result.field] = result.value
		if result.err != nil {
			log.Errorln(logTag, ":", result.err)
			return nil, fmt.Errorf(`cannot fetch value for "%s"`, result.field)
		}
	}

	return json.Marshal(queryVolume)
}

func (es *elasticsearch) topClicksRaw(ctx context.Context, from, to, query string, size int, filters map[string]interface{}, indices ...string) ([]map[string]interface{}, error) {
	var wg sync.WaitGroup

	type result struct {
		field string
		value []map[string]interface{}
		err   error
	}
	out := make(chan result)

	wg.Add(1)
	go func(out chan<- result) {
		defer wg.Done()
		resultsClicks, err := es.topResultsClicksRaw(ctx, from, to, query, size, filters, indices...)
		if err != nil {
			out <- result{
				field: "results_clicks",
				err:   err,
			}
		} else {
			out <- result{
				field: "results_clicks",
				value: resultsClicks,
			}
		}
	}(out)

	go func() {
		wg.Wait()
		close(out)
	}()

	var normalizedClicks = make([]map[string]interface{}, 0)
	for result := range out {
		if result.err != nil {
			log.Errorln(logTag, ":", result.err)
			return nil, fmt.Errorf(`cannot fetch value for "%s"`, result.field)
		}
		if result.field == "results_clicks" {
			normalizedClicks = result.value

		}
	}

	return normalizedClicks, nil
}

func (es *elasticsearch) advancedAnalytics(ctx context.Context, queryParams QueryParams, clickAnalytics bool, filters map[string]interface{}, indices ...string) ([]byte, error) {
	var wg sync.WaitGroup
	out := make(chan interface{})

	wg.Add(1)
	go func(out chan<- interface{}) {
		defer wg.Done()
		popularSearches, err := es.popularSearches(ctx, queryParams.From, queryParams.To, queryParams.Size, clickAnalytics, filters, indices...)
		if err != nil {
			log.Errorln(logTag, ":", err)
			out <- map[string]interface{}{
				"popular_searches": []interface{}{},
			}
		} else {
			out <- popularSearches
		}
	}(out)

	wg.Add(1)
	go func(out chan<- interface{}) {
		defer wg.Done()
		popularResults, err := es.popularResults(ctx, queryParams.From, queryParams.To, queryParams.Size, clickAnalytics, filters, indices...)
		if err != nil {
			log.Errorln(logTag, ":", err)
			out <- map[string]interface{}{
				"popular_results": []interface{}{},
			}
		} else {
			out <- popularResults
		}
	}(out)

	wg.Add(1)
	go func(out chan<- interface{}) {
		defer wg.Done()
		popularFilters, err := es.popularFilters(ctx, queryParams.From, queryParams.To, queryParams.Size, clickAnalytics, filters, indices...)
		if err != nil {
			log.Errorln(logTag, ":", err)
			out <- map[string]interface{}{
				"popular_filters": []interface{}{},
			}
		} else {
			out <- popularFilters
		}
	}(out)

	wg.Add(1)
	go func(out chan<- interface{}) {
		defer wg.Done()
		noResultsSearches, err := es.noResultSearches(ctx, queryParams.From, queryParams.To, queryParams.Size, filters, indices...)
		if err != nil {
			log.Errorln(logTag, ":", err)
			out <- map[string]interface{}{
				"no_results_searches": []interface{}{},
			}
		} else {
			out <- noResultsSearches
		}
	}(out)

	wg.Add(1)
	go func(out chan<- interface{}) {
		defer wg.Done()
		searchHistogram, err := es.searchHistogram(ctx, queryParams, filters, indices...)
		if err != nil {
			log.Errorln(logTag, ":", err)
			out <- map[string]interface{}{
				"search_volume": []interface{}{},
			}
		} else {
			out <- searchHistogram
		}
	}(out)

	go func() {
		wg.Wait()
		close(out)
	}()

	var advancedAnalytics []interface{}
	for result := range out {
		advancedAnalytics = append(advancedAnalytics, result)
	}

	return json.Marshal(advancedAnalytics)
}

func (es *elasticsearch) popularSearches(ctx context.Context, from, to string, size int, clickAnalytics bool, filters map[string]interface{}, indices ...string) (interface{}, error) {
	raw, err := es.popularSearchesRaw(ctx, from, to, size, clickAnalytics, filters, indices...)
	if err != nil {
		return []interface{}{}, err
	}

	var response struct {
		PopularSearches []map[string]interface{} `json:"popular_searches"`
	}
	err = json.Unmarshal(raw, &response)
	if err != nil {
		return []interface{}{}, err
	}

	return response, nil
}

func (es *elasticsearch) popularSearchesRaw(ctx context.Context, from, to string, size int, clickAnalytics bool, filters map[string]interface{}, indices ...string) ([]byte, error) {
	return es.popularSearchesRawEs7(ctx, from, to, size, clickAnalytics, filters, indices...)
}

func (es *elasticsearch) getTotalUniqueSearches(ctx context.Context, from, to string, minDocCount int, filters map[string]interface{}, indices ...string) (totalUniqueSearches float64, avgClickRate float64, err error) {
	return es.getTotalUniqueSearchesEs7(ctx, from, to, minDocCount, filters, indices...)
}

func (es *elasticsearch) getTotalUniqueNoResultsSearches(ctx context.Context, from, to string, filters map[string]interface{}, indices ...string) (totalUniqueNoResultsSearches float64, err error) {
	return es.getTotalUniqueNoResultsSearchesEs7(ctx, from, to, filters, indices...)
}

func (es *elasticsearch) getTotalSearchesFromLogs(ctx context.Context, from, to string, minResponseTime *int, indices ...string) (float64, error) {
	return es.getTotalSearchesFromLogsEs7(ctx, from, to, minResponseTime, indices...)
}

func (es *elasticsearch) noResultSearches(ctx context.Context, from, to string, size int, filters map[string]interface{}, indices ...string) (interface{}, error) {
	raw, err := es.noResultSearchesRaw(ctx, from, to, size, filters, indices...)
	if err != nil {
		return []interface{}{}, err
	}

	var response struct {
		NoResultsSearches []map[string]interface{} `json:"no_results_searches"`
	}
	err = json.Unmarshal(raw, &response)
	if err != nil {
		return []interface{}{}, err
	}

	return response, nil
}

func (es *elasticsearch) noResultSearchesRaw(ctx context.Context, from, to string, size int, filters map[string]interface{}, indices ...string) ([]byte, error) {
	return es.noResultSearchesRawEs7(ctx, from, to, size, filters, indices...)
}

func (es *elasticsearch) noResultSearchesWithSummary(ctx context.Context, from, to string, size int, filters map[string]interface{}, indices ...string) ([]byte, error) {
	type result struct {
		field string
		value interface{}
		err   error
	}
	var wg sync.WaitGroup
	out := make(chan result)

	wg.Add(1)
	go func(out chan<- result) {
		defer wg.Done()
		noResultsSearches, err := es.noResultSearchesRaw(ctx, from, to, size, filters, indices...)
		if err != nil {
			out <- result{
				field: "no_results_searches",
				err:   err,
			}
		} else {
			out <- result{
				field: "no_results_searches",
				value: noResultsSearches,
			}
		}
	}(out)

	wg.Add(1)
	go func(out chan<- result) {
		defer wg.Done()
		totalSearchQueries, err := es.getTotalUniqueNoResultsSearches(ctx, from, to, filters, indices...)
		if err != nil {
			out <- result{
				field: "total_search_terms",
				err:   err,
			}
		} else {
			out <- result{
				field: "total_search_terms",
				value: totalSearchQueries,
			}
		}
	}(out)

	go func() {
		wg.Wait()
		close(out)
	}()

	var noResultsSearchesResponse = make(map[string]interface{})

	for result := range out {
		if result.err != nil {
			log.Errorln(logTag, ":", result.err)
			return nil, fmt.Errorf(`cannot fetch value for "%s"`, result.field)
		}
		switch result.field {
		case "no_results_searches":
			var noResultsSearches map[string]interface{}
			responseInBytes, ok := result.value.([]byte)
			if !ok {
				return nil, fmt.Errorf(`cannot parse value for "%s"`, result.field)
			}
			err := json.Unmarshal(responseInBytes, &noResultsSearches)
			if err != nil {
				log.Errorln(logTag, ":", err)
				return nil, fmt.Errorf(`cannot un-marshall value for "%s"`, result.field)
			}
			for k, v := range noResultsSearches {
				noResultsSearchesResponse[k] = v
			}
		case "total_search_terms":
			noResultsSearchesResponse["total_search_terms"] = result.value
		default:
			return nil, fmt.Errorf(`illegal field "%s" encountered`, result.field)
		}
	}
	return json.Marshal(noResultsSearchesResponse)
}

func (es *elasticsearch) popularFilters(ctx context.Context, from, to string, size int, clickAnalytics bool, filters map[string]interface{}, indices ...string) (interface{}, error) {
	raw, err := es.popularFiltersRaw(ctx, from, to, size, clickAnalytics, filters, indices...)
	if err != nil {
		return []interface{}{}, err
	}

	var response struct {
		PopularFilters []map[string]interface{} `json:"popular_filters"`
	}
	err = json.Unmarshal(raw, &response)
	if err != nil {
		return []interface{}{}, err
	}

	return response, nil
}

func (es *elasticsearch) popularFiltersWithSummary(ctx context.Context, from, to string, size int, clickAnalytics bool, filters map[string]interface{}, indices ...string) ([]byte, error) {
	type result struct {
		field string
		value interface{}
		err   error
	}

	var wg sync.WaitGroup
	out := make(chan result)

	wg.Add(1)
	go func(out chan<- result) {
		defer wg.Done()
		popularFilters, err := es.popularFiltersRaw(ctx, from, to, size, clickAnalytics, filters, indices...)
		if err != nil {
			out <- result{
				field: "popular_filters",
				err:   err,
			}
		} else {
			out <- result{
				field: "popular_filters",
				value: popularFilters,
			}
		}
	}(out)

	wg.Add(1)
	go func(out chan<- result) {
		defer wg.Done()
		totalUniqueFilters, err := es.getTotalUniquePopularFilters(ctx, from, to, filters, indices...)
		if err != nil {
			out <- result{
				field: "total_filters",
				err:   err,
			}
		} else {
			out <- result{
				field: "total_filters",
				value: totalUniqueFilters,
			}
		}
	}(out)

	wg.Add(1)
	go func(out chan<- result) {
		defer wg.Done()
		totalSelections, err := es.getTotalFiltersSelections(ctx, from, to, filters, indices...)
		if err != nil {
			out <- result{
				field: "total_selections",
				err:   err,
			}
		} else {
			out <- result{
				field: "total_selections",
				value: totalSelections,
			}
		}
	}(out)

	if clickAnalytics {
		wg.Add(1)
		go func(out chan<- result) {
			defer wg.Done()
			clickAnalyticsSummary, err := es.getClickAnalyticsSummaryForFilters(ctx, from, to, filters, indices...)
			if err != nil {
				out <- result{
					field: "click_analytics_summary",
					err:   err,
				}
			} else {
				out <- result{
					field: "click_analytics_summary",
					value: clickAnalyticsSummary,
				}
			}
		}(out)
	}

	go func() {
		wg.Wait()
		close(out)
	}()

	var popularFiltersResponse = make(map[string]interface{})
	var clickSummary ClickSummary

	for result := range out {
		if result.err != nil {
			log.Errorln(logTag, ":", result.err)
			return nil, fmt.Errorf(`cannot fetch value for "%s"`, result.field)
		}
		switch result.field {
		case "popular_filters":
			var popularFilters map[string]interface{}
			responseInBytes, ok := result.value.([]byte)
			if !ok {
				return nil, fmt.Errorf(`cannot parse value for "%s"`, result.field)
			}
			err := json.Unmarshal(responseInBytes, &popularFilters)
			if err != nil {
				log.Errorln(logTag, ":", err)
				return nil, fmt.Errorf(`cannot un-marshall value for "%s"`, result.field)
			}
			for k, v := range popularFilters {
				popularFiltersResponse[k] = v
			}
		case "total_filters", "total_selections":
			popularFiltersResponse[result.field] = result.value
		case "click_analytics_summary":
			var ok bool
			clickSummary, ok = result.value.(ClickSummary)
			if !ok {
				return nil, fmt.Errorf(`cannot parse value for "%s"`, result.field)
			}
		default:
			return nil, fmt.Errorf(`illegal field "%s" encountered`, result.field)
		}
	}

	if clickAnalytics {
		var avgClickRate, avgConversionRate float64
		totalSelections, ok := popularFiltersResponse["total_selections"].(float64)
		if !ok {
			return nil, fmt.Errorf("cannot parse value for total_selections")
		}
		if totalSelections != 0 {
			avgClickRate = clickSummary.TotalClicks / totalSelections * 100
			avgConversionRate = clickSummary.TotalConversions / totalSelections * 100
		}
		// Total Clicks
		popularFiltersResponse["total_clicks"] = clickSummary.TotalClicks

		// Avg. Click Position
		popularFiltersResponse["avg_click_position"] = clickSummary.AvgClickPosition

		// Avg. Click Rate
		popularFiltersResponse["avg_click_rate"] = util.WithPrecision(avgClickRate, 2)

		// Avg. Conversion Rate
		popularFiltersResponse["avg_conversion_rate"] = util.WithPrecision(avgConversionRate, 2)
	}

	return json.Marshal(popularFiltersResponse)
}

func (es *elasticsearch) popularFiltersRaw(ctx context.Context, from, to string, size int, clickAnalytics bool, filters map[string]interface{}, indices ...string) ([]byte, error) {
	return es.popularFiltersRawEs7(ctx, from, to, size, clickAnalytics, filters, indices...)
}

func (es *elasticsearch) getTotalUniquePopularFilters(ctx context.Context, from, to string, filters map[string]interface{}, indices ...string) (float64, error) {
	return es.getTotalUniquePopularFiltersEs7(ctx, from, to, filters, indices...)
}

func (es *elasticsearch) getTotalFiltersSelections(ctx context.Context, from, to string, filters map[string]interface{}, indices ...string) (float64, error) {
	return es.getTotalFiltersSelectionsEs7(ctx, from, to, filters, indices...)
}

func (es *elasticsearch) popularResults(ctx context.Context, from, to string, size int, clickAnalytics bool, filters map[string]interface{}, indices ...string) (interface{}, error) {
	raw, err := es.popularResultsRaw(ctx, from, to, size, clickAnalytics, filters, indices...)
	if err != nil {
		return []interface{}{}, err
	}

	var response struct {
		PopularResults []map[string]interface{} `json:"popular_results"`
	}
	err = json.Unmarshal(raw, &response)
	if err != nil {
		return []interface{}{}, err
	}

	return response, nil
}

func (es *elasticsearch) popularResultsWithSummary(ctx context.Context, from, to string, size int, clickAnalytics bool, filters map[string]interface{}, indices ...string) ([]byte, error) {
	type result struct {
		field string
		value interface{}
		err   error
	}

	var wg sync.WaitGroup
	out := make(chan result)

	wg.Add(1)
	go func(out chan<- result) {
		defer wg.Done()
		popularResults, err := es.popularResultsRaw(ctx, from, to, size, clickAnalytics, filters, indices...)
		if err != nil {
			out <- result{
				field: "popular_results",
				err:   err,
			}
		} else {
			out <- result{
				field: "popular_results",
				value: popularResults,
			}
		}
	}(out)

	wg.Add(1)
	go func(out chan<- result) {
		defer wg.Done()
		totalImpressionsCount, err := es.totalResultsCount(ctx, from, to, filters, indices...)
		if err != nil {
			out <- result{
				field: "total_impressions",
				err:   err,
			}
		} else {
			out <- result{
				field: "total_impressions",
				value: totalImpressionsCount,
			}
		}
	}(out)

	wg.Add(1)
	go func(out chan<- result) {
		defer wg.Done()
		totalUniqueResultsCount, err := es.totalUniqueResultsCount(ctx, from, to, filters, indices...)
		if err != nil {
			out <- result{
				field: "total_results",
				err:   err,
			}
		} else {
			out <- result{
				field: "total_results",
				value: totalUniqueResultsCount,
			}
		}
	}(out)

	if clickAnalytics {
		wg.Add(1)
		go func(out chan<- result) {
			defer wg.Done()
			clickAnalyticsSummary, err := es.getClickAnalyticsSummaryForResults(ctx, from, to, filters, indices...)
			if err != nil {
				out <- result{
					field: "click_analytics_summary",
					err:   err,
				}
			} else {
				out <- result{
					field: "click_analytics_summary",
					value: clickAnalyticsSummary,
				}
			}
		}(out)
	}

	go func() {
		wg.Wait()
		close(out)
	}()

	var popularResultsResponse = make(map[string]interface{})
	var clickSummary ClickSummary
	for result := range out {
		if result.err != nil {
			log.Errorln(logTag, ":", result.err)
			return nil, fmt.Errorf(`cannot fetch value for "%s"`, result.field)
		}
		switch result.field {
		case "popular_results":
			var popularResults map[string]interface{}
			responseInBytes, ok := result.value.([]byte)
			if !ok {
				return nil, fmt.Errorf(`cannot parse value for "%s"`, result.field)
			}
			err := json.Unmarshal(responseInBytes, &popularResults)
			if err != nil {
				log.Errorln(logTag, ":", err)
				return nil, fmt.Errorf(`cannot un-marshall value for "%s"`, result.field)
			}
			for k, v := range popularResults {
				popularResultsResponse[k] = v
			}
		case "total_results", "total_impressions":
			popularResultsResponse[result.field] = result.value
		case "click_analytics_summary":
			var ok bool
			clickSummary, ok = result.value.(ClickSummary)
			if !ok {
				return nil, fmt.Errorf(`cannot parse value for "%s"`, result.field)
			}
		default:
			return nil, fmt.Errorf(`illegal field "%s" encountered`, result.field)
		}
	}
	if clickAnalytics {
		var avgClickRate, avgConversionRate float64
		totalImpressions, ok := popularResultsResponse["total_impressions"].(float64)
		if !ok {
			return nil, fmt.Errorf("cannot parse value for total_impressions")
		}
		if totalImpressions != 0 {
			avgClickRate = clickSummary.TotalClicks / totalImpressions * 100
			avgConversionRate = clickSummary.TotalConversions / totalImpressions * 100
		}
		// Total Clicks
		popularResultsResponse["total_clicks"] = clickSummary.TotalClicks

		// Avg. Click Position
		popularResultsResponse["avg_click_position"] = clickSummary.AvgClickPosition

		// Avg. Click Rate
		popularResultsResponse["avg_click_rate"] = util.WithPrecision(avgClickRate, 2)

		// Avg. Conversion Rate
		popularResultsResponse["avg_conversion_rate"] = util.WithPrecision(avgConversionRate, 2)
	}
	return json.Marshal(popularResultsResponse)
}

func (es *elasticsearch) recentResults(ctx context.Context, from, to string, size int, filters map[string]interface{}, indices ...string) ([]byte, error) {
	return es.recentResultsEs7(ctx, from, to, size, filters, indices...)
}

func (es *elasticsearch) popularResultsRaw(ctx context.Context, from, to string, size int, clickAnalytics bool, filters map[string]interface{}, indices ...string) ([]byte, error) {
	return es.popularResultsRawEs7(ctx, from, to, size, clickAnalytics, filters, indices...)
}

func (es *elasticsearch) geoRequestsDistributionWithSummary(ctx context.Context, from, to string, size int, filters map[string]interface{}, indices ...string) ([]byte, error) {
	type result struct {
		field string
		value interface{}
		err   error
	}
	var wg sync.WaitGroup
	out := make(chan result)

	wg.Add(1)
	go func(out chan<- result) {
		defer wg.Done()
		geoRequestsDistribution, err := es.geoRequestsDistribution(ctx, from, to, size, filters, indices...)
		if err != nil {
			out <- result{
				field: "geo_distribution",
				err:   err,
			}
		} else {
			out <- result{
				field: "geo_distribution",
				value: geoRequestsDistribution,
			}
		}
	}(out)

	wg.Add(1)
	go func(out chan<- result) {
		defer wg.Done()
		totalSearches, err := es.totalSearches(ctx, from, to, filters, indices...)
		if err != nil {
			out <- result{
				field: "total_searches",
				err:   err,
			}
		} else {
			out <- result{
				field: "total_searches",
				value: totalSearches,
			}
		}
	}(out)

	wg.Add(1)
	go func(out chan<- result) {
		defer wg.Done()
		totalCountries, err := es.geoTotalCountries(ctx, from, to, filters, indices...)
		if err != nil {
			out <- result{
				field: "total_countries",
				err:   err,
			}
		} else {
			out <- result{
				field: "total_countries",
				value: totalCountries,
			}
		}
	}(out)

	go func() {
		wg.Wait()
		close(out)
	}()

	var geoDistributionResponse = make(map[string]interface{})

	for result := range out {
		if result.err != nil {
			log.Errorln(logTag, ":", result.err)
			return nil, fmt.Errorf(`cannot fetch value for "%s"`, result.field)
		}
		switch result.field {
		case "geo_distribution":
			var geoDistribution map[string]interface{}
			responseInBytes, ok := result.value.([]byte)
			if !ok {
				return nil, fmt.Errorf(`cannot parse value for "%s"`, result.field)
			}
			err := json.Unmarshal(responseInBytes, &geoDistribution)
			if err != nil {
				log.Errorln(logTag, ":", err)
				return nil, fmt.Errorf(`cannot un-marshall value for "%s"`, result.field)
			}
			for k, v := range geoDistribution {
				geoDistributionResponse[k] = v
			}
		case "total_searches", "total_countries":
			geoDistributionResponse[result.field] = result.value
		default:
			return nil, fmt.Errorf(`illegal field "%s" encountered`, result.field)
		}
	}
	return json.Marshal(geoDistributionResponse)
}

func (es *elasticsearch) geoRequestsDistribution(ctx context.Context, from, to string, size int, filters map[string]interface{}, indices ...string) ([]byte, error) {
	return es.geoRequestsDistributionEs7(ctx, from, to, size, filters, indices...)
}

func (es *elasticsearch) latenciesWithSummary(ctx context.Context, from, to string, size int, filters map[string]interface{}, indices ...string) ([]byte, error) {
	type result struct {
		field string
		value interface{}
		err   error
	}
	var wg sync.WaitGroup
	out := make(chan result)

	wg.Add(1)
	go func(out chan<- result) {
		defer wg.Done()
		latencies, err := es.latencies(ctx, from, to, size, filters, indices...)
		if err != nil {
			out <- result{
				field: "latencies",
				err:   err,
			}
		} else {
			out <- result{
				field: "latencies",
				value: latencies,
			}
		}
	}(out)

	wg.Add(1)
	go func(out chan<- result) {
		defer wg.Done()
		totalRequests, err := es.totalSearches(ctx, from, to, filters, indices...)
		if err != nil {
			out <- result{
				field: "total_searches",
				err:   err,
			}
		} else {
			out <- result{
				field: "total_searches",
				value: totalRequests,
			}
		}
	}(out)

	wg.Add(1)
	go func(out chan<- result) {
		defer wg.Done()
		totalRequests, err := es.getAvgSearchLatency(ctx, from, to, filters, indices...)
		if err != nil {
			out <- result{
				field: "avg_search_latency",
				err:   err,
			}
		} else {
			out <- result{
				field: "avg_search_latency",
				value: totalRequests,
			}
		}
	}(out)

	go func() {
		wg.Wait()
		close(out)
	}()

	var latenciesResponse = make(map[string]interface{})

	for result := range out {
		if result.err != nil {
			log.Errorln(logTag, ":", result.err)
			return nil, fmt.Errorf(`cannot fetch value for "%s"`, result.field)
		}
		switch result.field {
		case "latencies":
			var latencies map[string]interface{}
			responseInBytes, ok := result.value.([]byte)
			if !ok {
				return nil, fmt.Errorf(`cannot parse value for "%s"`, result.field)
			}
			err := json.Unmarshal(responseInBytes, &latencies)
			if err != nil {
				log.Errorln(logTag, ":", err)
				return nil, fmt.Errorf(`cannot un-marshall value for "%s"`, result.field)
			}
			for k, v := range latencies {
				latenciesResponse[k] = v
			}
		case "total_searches", "avg_search_latency":
			latenciesResponse[result.field] = result.value
		default:
			return nil, fmt.Errorf(`illegal field "%s" encountered`, result.field)
		}
	}
	return json.Marshal(latenciesResponse)
}

func (es *elasticsearch) latencies(ctx context.Context, from, to string, size int, filters map[string]interface{}, indices ...string) ([]byte, error) {
	return es.latenciesEs7(ctx, from, to, size, filters, indices...)
}

func (es *elasticsearch) getAvgSearchLatency(ctx context.Context, from, to string, filters map[string]interface{}, indices ...string) (float64, error) {
	return es.getAvgSearchLatencyEs7(ctx, from, to, filters, indices...)
}

func (es *elasticsearch) getClickAnalyticsSummary(ctx context.Context, from, to string, filters map[string]interface{}, indices ...string) (map[string]interface{}, error) {
	var clickAnalyticsSummary = make(map[string]interface{})

	type result struct {
		field string
		value float64
		err   error
	}

	var wg sync.WaitGroup
	out := make(chan result)

	wg.Add(1)
	go func(out chan<- result) {
		defer wg.Done()
		totalSearches, err := es.totalSearches(ctx, from, to, filters, indices...)
		if err != nil {
			out <- result{
				field: "total_searches",
				err:   err,
			}
		} else {
			out <- result{
				field: "total_searches",
				value: totalSearches,
			}
		}
	}(out)

	wg.Add(1)
	go func(out chan<- result) {
		defer wg.Done()
		totalClicks, err := es.totalClicks(ctx, from, to, filters, nil, indices...)
		if err != nil {
			out <- result{
				field: "total_clicks",
				err:   err,
			}
		} else {
			out <- result{
				field: "total_clicks",
				value: totalClicks,
			}
		}
	}(out)

	wg.Add(1)
	go func(out chan<- result) {
		defer wg.Done()
		avgResultClickPosition, err := es.avgResultClickPosition(ctx, from, to, filters, nil, indices...)
		if err != nil {
			out <- result{
				field: "avg_result_click_position",
				err:   err,
			}
		} else {
			out <- result{
				field: "avg_result_click_position",
				value: avgResultClickPosition,
			}
		}
	}(out)

	wg.Add(1)
	go func(out chan<- result) {
		defer wg.Done()
		avgSuggestionsClickPosition, err := es.avgSuggestionsClickPosition(ctx, from, to, filters, nil, indices...)
		if err != nil {
			out <- result{
				field: "avg_suggestions_click_position",
				err:   err,
			}
		} else {
			out <- result{
				field: "avg_suggestions_click_position",
				value: avgSuggestionsClickPosition,
			}
		}
	}(out)

	wg.Add(1)
	go func(out chan<- result) {
		defer wg.Done()

		totalSuggestionsClicks, err := es.totalSuggestionsClicks(ctx, from, to, filters, nil, indices...)
		if err != nil {
			out <- result{
				field: "total_suggestions_clicks",
				err:   err,
			}
		} else {
			out <- result{
				field: "total_suggestions_clicks",
				value: totalSuggestionsClicks,
			}
		}
	}(out)

	wg.Add(1)
	go func(out chan<- result) {
		defer wg.Done()
		totalConversions, err := es.totalConversions(ctx, from, to, filters, nil, indices...)
		if err != nil {
			out <- result{
				field: "total_conversions",
				err:   err,
			}
		} else {
			out <- result{
				field: "total_conversions",
				value: totalConversions,
			}
		}
	}(out)

	go func() {
		wg.Wait()
		close(out)
	}()

	var totalSearches, totalResultClicks, totalConversions, totalSuggestionsClicks, avgResultClickPosition, avgSuggestionsClickPosition float64
	for result := range out {
		if result.err != nil {
			log.Errorln(logTag, ":", result.err)
			return nil, fmt.Errorf(`cannot fetch value for "%s"`, result.field)
		}
		switch result.field {
		case "total_searches":
			totalSearches = result.value
		case "total_clicks":
			totalResultClicks = result.value
		case "total_suggestions_clicks":
			totalSuggestionsClicks = result.value
		case "total_conversions":
			totalConversions = result.value
		case "avg_result_click_position":
			avgResultClickPosition = result.value
		case "avg_suggestions_click_position":
			avgSuggestionsClickPosition = result.value
		default:
			return nil, fmt.Errorf(`illegal field "%s" encountered`, result.field)
		}
	}
	totalClicks := totalResultClicks + totalSuggestionsClicks

	var avgClickRate, avgConversionRate, avgClickPosition float64
	if totalSearches != 0 {
		avgClickRate = totalClicks / totalSearches * 100
		avgConversionRate = totalConversions / totalSearches * 100
	}

	if totalClicks != 0 {
		avgClickPosition = ((totalResultClicks * avgResultClickPosition) + (totalSuggestionsClicks * avgSuggestionsClickPosition)) / totalClicks
	}

	// Total Clicks
	clickAnalyticsSummary["total_clicks"] = util.WithPrecision(totalClicks, 2)

	// Avg. Click Position
	clickAnalyticsSummary["avg_click_position"] = util.WithPrecision(avgClickPosition, 2)

	// Avg. Click Rate
	clickAnalyticsSummary["avg_click_rate"] = util.WithPrecision(avgClickRate, 2)

	// Avg. Conversion Rate
	clickAnalyticsSummary["avg_conversion_rate"] = util.WithPrecision(avgConversionRate, 2)
	return clickAnalyticsSummary, nil
}

func (es *elasticsearch) getClickAnalyticsSummaryForResults(ctx context.Context, from, to string, filters map[string]interface{}, indices ...string) (ClickSummary, error) {
	type result struct {
		field string
		value float64
		err   error
	}

	var wg sync.WaitGroup
	out := make(chan result)

	wg.Add(1)
	go func(out chan<- result) {
		defer wg.Done()
		totalClicks, err := es.totalClicks(ctx, from, to, filters, nil, indices...)
		if err != nil {
			out <- result{
				field: "total_clicks",
				err:   err,
			}
		} else {
			out <- result{
				field: "total_clicks",
				value: totalClicks,
			}
		}
	}(out)

	wg.Add(1)
	go func(out chan<- result) {
		defer wg.Done()
		avgResultClickPosition, err := es.avgResultClickPosition(ctx, from, to, filters, nil, indices...)
		if err != nil {
			out <- result{
				field: "avg_result_click_position",
				err:   err,
			}
		} else {
			out <- result{
				field: "avg_result_click_position",
				value: avgResultClickPosition,
			}
		}
	}(out)

	wg.Add(1)
	go func(out chan<- result) {
		defer wg.Done()
		totalConversions, err := es.totalConversions(ctx, from, to, filters, nil, indices...)
		if err != nil {
			out <- result{
				field: "total_conversions",
				err:   err,
			}
		} else {
			out <- result{
				field: "total_conversions",
				value: totalConversions,
			}
		}
	}(out)

	go func() {
		wg.Wait()
		close(out)
	}()

	var totalResultClicks, totalConversions, avgResultClickPosition float64
	for result := range out {
		if result.err != nil {
			log.Errorln(logTag, ":", result.err)
			return ClickSummary{}, fmt.Errorf(`cannot fetch value for "%s"`, result.field)
		}
		switch result.field {
		case "total_clicks":
			totalResultClicks = result.value
		case "total_conversions":
			totalConversions = result.value
		case "avg_result_click_position":
			avgResultClickPosition = result.value
		default:
			return ClickSummary{}, fmt.Errorf(`illegal field "%s" encountered`, result.field)
		}
	}

	return ClickSummary{
		AvgClickPosition: util.WithPrecision(avgResultClickPosition, 2),
		TotalClicks:      util.WithPrecision(totalResultClicks, 2),
		TotalConversions: util.WithPrecision(totalConversions, 2),
	}, nil
}

func (es *elasticsearch) getClickAnalyticsSummaryForFilters(ctx context.Context, from, to string, filters map[string]interface{}, indices ...string) (ClickSummary, error) {
	var clickAnalyticsSummary = make(map[string]interface{})

	type result struct {
		field string
		value float64
		err   error
	}

	var wg sync.WaitGroup
	out := make(chan result)
	queryFilters := []QueryFilter{
		{
			Type:       "exists",
			Field:      "search_filters.key",
			NestedPath: "search_filters",
		},
	}

	wg.Add(1)
	go func(out chan<- result) {
		defer wg.Done()
		totalClicks, err := es.totalClicks(ctx, from, to, filters, &queryFilters, indices...)
		if err != nil {
			out <- result{
				field: "total_clicks",
				err:   err,
			}
		} else {
			out <- result{
				field: "total_clicks",
				value: totalClicks,
			}
		}
	}(out)

	wg.Add(1)
	go func(out chan<- result) {
		defer wg.Done()
		avgResultClickPosition, err := es.avgResultClickPosition(ctx, from, to, filters, &queryFilters, indices...)
		if err != nil {
			out <- result{
				field: "avg_result_click_position",
				err:   err,
			}
		} else {
			out <- result{
				field: "avg_result_click_position",
				value: avgResultClickPosition,
			}
		}
	}(out)

	wg.Add(1)
	go func(out chan<- result) {
		defer wg.Done()
		avgSuggestionsClickPosition, err := es.avgSuggestionsClickPosition(ctx, from, to, filters, &queryFilters, indices...)
		if err != nil {
			out <- result{
				field: "avg_suggestions_click_position",
				err:   err,
			}
		} else {
			out <- result{
				field: "avg_suggestions_click_position",
				value: avgSuggestionsClickPosition,
			}
		}
	}(out)

	wg.Add(1)
	go func(out chan<- result) {
		defer wg.Done()

		totalSuggestionsClicks, err := es.totalSuggestionsClicks(ctx, from, to, filters, &queryFilters, indices...)
		if err != nil {
			out <- result{
				field: "total_suggestions_clicks",
				err:   err,
			}
		} else {
			out <- result{
				field: "total_suggestions_clicks",
				value: totalSuggestionsClicks,
			}
		}
	}(out)

	wg.Add(1)
	go func(out chan<- result) {
		defer wg.Done()
		totalConversions, err := es.totalConversions(ctx, from, to, filters, &queryFilters, indices...)
		if err != nil {
			out <- result{
				field: "total_conversions",
				err:   err,
			}
		} else {
			out <- result{
				field: "total_conversions",
				value: totalConversions,
			}
		}
	}(out)

	go func() {
		wg.Wait()
		close(out)
	}()

	var totalResultClicks, totalConversions, totalSuggestionsClicks, avgResultClickPosition, avgSuggestionsClickPosition float64
	for result := range out {
		if result.err != nil {
			log.Errorln(logTag, ":", result.err)
			return ClickSummary{}, fmt.Errorf(`cannot fetch value for "%s"`, result.field)
		}
		switch result.field {
		case "total_clicks":
			totalResultClicks = result.value
		case "total_suggestions_clicks":
			totalSuggestionsClicks = result.value
		case "total_conversions":
			totalConversions = result.value
		case "avg_result_click_position":
			avgResultClickPosition = result.value
		case "avg_suggestions_click_position":
			avgSuggestionsClickPosition = result.value
		default:
			return ClickSummary{}, fmt.Errorf(`illegal field "%s" encountered`, result.field)
		}
	}
	totalClicks := totalResultClicks + totalSuggestionsClicks

	var avgClickPosition float64

	if totalClicks != 0 {
		avgClickPosition = ((totalResultClicks * avgResultClickPosition) + (totalSuggestionsClicks * avgSuggestionsClickPosition)) / totalClicks
	}

	// Total Clicks
	clickAnalyticsSummary["total_clicks"] = util.WithPrecision(totalClicks, 2)

	// Avg. Click Position
	clickAnalyticsSummary["avg_click_position"] = util.WithPrecision(avgClickPosition, 2)

	return ClickSummary{
		AvgClickPosition: avgClickPosition,
		TotalClicks:      totalClicks,
		TotalConversions: totalConversions,
	}, nil
}

func (es *elasticsearch) popularSearchesWithSummary(ctx context.Context, from, to string, size int, clickAnalytics bool, filters map[string]interface{}, indices ...string) ([]byte, error) {
	type result struct {
		field string
		value interface{}
		err   error
	}
	var wg sync.WaitGroup
	out := make(chan result)

	wg.Add(1)
	go func(out chan<- result) {
		defer wg.Done()
		popularSearches, err := es.popularSearchesRaw(ctx, from, to, size, clickAnalytics, filters, indices...)
		if err != nil {
			out <- result{
				field: "popular_searches",
				err:   err,
			}
		} else {
			out <- result{
				field: "popular_searches",
				value: popularSearches,
			}
		}
	}(out)

	wg.Add(1)
	go func(out chan<- result) {
		defer wg.Done()
		totalUniqueSearches, _, err := es.getTotalUniqueSearches(ctx, from, to, 0, filters, indices...)
		if err != nil {
			out <- result{
				field: "total_search_terms",
				err:   err,
			}
		} else {
			out <- result{
				field: "total_search_terms",
				value: totalUniqueSearches,
			}
		}
	}(out)

	if clickAnalytics {
		wg.Add(1)
		go func(out chan<- result) {
			defer wg.Done()
			clickAnalyticsSummary, err := es.getClickAnalyticsSummary(ctx, from, to, filters, indices...)
			if err != nil {
				out <- result{
					field: "click_analytics_summary",
					err:   err,
				}
			} else {
				out <- result{
					field: "click_analytics_summary",
					value: clickAnalyticsSummary,
				}
			}
		}(out)
	}

	go func() {
		wg.Wait()
		close(out)
	}()

	var popularSearchesResponse = make(map[string]interface{})

	for result := range out {
		if result.err != nil {
			log.Errorln(logTag, ":", result.err)
			return nil, fmt.Errorf(`cannot fetch value for "%s"`, result.field)
		}
		switch result.field {
		case "popular_searches":
			var popularSearches map[string]interface{}
			responseInBytes, ok := result.value.([]byte)
			if !ok {
				return nil, fmt.Errorf(`cannot parse value for "%s"`, result.field)
			}
			err := json.Unmarshal(responseInBytes, &popularSearches)
			if err != nil {
				log.Errorln(logTag, ":", err)
				return nil, fmt.Errorf(`cannot un-marshall value for "%s"`, result.field)
			}
			for k, v := range popularSearches {
				popularSearchesResponse[k] = v
			}
		case "total_search_terms":
			popularSearchesResponse["total_search_terms"] = result.value
		case "click_analytics_summary":
			clickSummary, ok := result.value.(map[string]interface{})
			if !ok {
				return nil, fmt.Errorf(`cannot parse value for "%s"`, result.field)
			}
			for k, v := range clickSummary {
				popularSearchesResponse[k] = v
			}
		default:
			return nil, fmt.Errorf(`illegal field "%s" encountered`, result.field)
		}
	}
	return json.Marshal(popularSearchesResponse)
}

func (es *elasticsearch) recentSearches(ctx context.Context, from, to string, size int, minChar *int, filters map[string]interface{}, indices ...string) ([]byte, error) {
	return es.recentSearchesEs7(ctx, from, to, size, minChar, filters, indices...)
}

func (es *elasticsearch) topResultsRaw(ctx context.Context, from, to, queryTerm string, size int, filters map[string]interface{}, indices ...string) ([]map[string]interface{}, error) {
	return es.topResultsRawEs7(ctx, from, to, queryTerm, size, filters, indices...)
}

func (es *elasticsearch) topResultsClicksRaw(ctx context.Context, from, to, queryTerm string, size int, filters map[string]interface{}, indices ...string) ([]map[string]interface{}, error) {
	return es.topResultsClicksRawEs7(ctx, from, to, queryTerm, size, filters, indices...)
}

func (es *elasticsearch) topSuggestionsClicksRaw(ctx context.Context, from, to, queryTerm string, size int, filters map[string]interface{}, indices ...string) ([]map[string]interface{}, error) {
	return es.topSuggestionsClicksRawEs7(ctx, from, to, queryTerm, size, filters, indices...)
}

func (es *elasticsearch) summary(ctx context.Context, from, to string, filters map[string]interface{}, indices ...string) ([]byte, error) {
	type result struct {
		field string
		value *SummaryRecord
		err   error
	}

	var wg sync.WaitGroup
	out := make(chan result)

	wg.Add(1)
	go func(out chan<- result) {
		defer wg.Done()
		summary, err := es.getSummary(ctx, from, to, filters, indices...)
		if err != nil {
			out <- result{
				field: "summary",
				err:   err,
			}
		} else {
			out <- result{
				field: "summary",
				value: summary,
			}
		}
	}(out)

	// Only calculate comparison for production and enterprise users
	if util.ValidatePlans(validPlans, util.GetFeatureCustomEvents()) {
		wg.Add(1)
		go func(out chan<- result) {
			defer wg.Done()

			fromTime, err := time.Parse(time.RFC3339, from)
			if err != nil {
				out <- result{
					field: "compare_timeframe",
					err:   err,
				}
			}
			toTime, err := time.Parse(time.RFC3339, to)
			if err != nil {
				out <- result{
					field: "compare_timeframe",
					err:   err,
				}
			}
			duration := toTime.Sub(fromTime)
			compareFromTime := fromTime.Add(time.Duration(-duration.Hours()) * time.Hour)
			compareFrom := compareFromTime.Format(time.RFC3339)
			compareTo := from
			// extract summary for last timeframe
			summary, err := es.getSummary(ctx, compareFrom, compareTo, filters, indices...)
			if err != nil {
				out <- result{
					field: "compare_timeframe",
					err:   err,
				}
			} else {
				out <- result{
					field: "compare_timeframe",
					value: summary,
				}
			}
		}(out)
	}

	go func() {
		wg.Wait()
		close(out)
	}()

	var summary, compareTimeframe *SummaryRecord
	for result := range out {
		if result.err != nil {
			log.Errorln(logTag, ":", result.err)
			return nil, fmt.Errorf(`cannot fetch value for "%s"`, result.field)
		}
		switch result.field {
		case "summary":
			summary = result.value
		case "compare_timeframe":
			compareTimeframe = result.value
		default:
			return nil, fmt.Errorf(`illegal field "%s" encountered`, result.field)
		}
	}

	summaryResponse := SummaryResponse{
		Summary:          *summary,
		CompateTimeframe: compareTimeframe,
	}

	return json.Marshal(summaryResponse)
}

func (es *elasticsearch) getDetailedSummary(ctx context.Context, from, to string, indices ...string) (*SummaryRecord, *DetailedSummary, error) {
	type result struct {
		field      string
		value      float64
		extraValue float64
		err        error
	}

	var wg sync.WaitGroup
	out := make(chan result)

	wg.Add(1)
	go func(out chan<- result) {
		defer wg.Done()
		filters := make(map[string]interface{})
		totalUniqueSearches, _, err := es.getTotalUniqueSearches(ctx, from, to, 0, filters, indices...)
		if err != nil {
			out <- result{
				field: "total_unique_searches",
				err:   err,
			}
		} else {
			out <- result{
				field: "total_unique_searches",
				value: totalUniqueSearches,
			}
		}
	}(out)

	wg.Add(1)
	go func(out chan<- result) {
		defer wg.Done()
		filters := make(map[string]interface{})
		totalUniqueSearchesWithMinCount, avgClickRateWithMinCount, err := es.getTotalUniqueSearches(ctx, from, to, 100, filters, indices...)
		if err != nil {
			out <- result{
				field: "total_unique_searches_with_min_count",
				err:   err,
			}
		} else {
			out <- result{
				field:      "total_unique_searches_with_min_count",
				value:      totalUniqueSearchesWithMinCount,
				extraValue: avgClickRateWithMinCount,
			}
		}
	}(out)

	wg.Add(1)
	go func(out chan<- result) {
		defer wg.Done()
		avgQueryLength, err := es.avgQueryLength(ctx, from, to, indices...)
		if err != nil {
			out <- result{
				field: "avg_query_length",
				err:   err,
			}
		} else {
			out <- result{
				field: "avg_query_length",
				value: avgQueryLength,
			}
		}
	}(out)

	wg.Add(1)
	go func(out chan<- result) {
		defer wg.Done()
		minStatus := 400
		maxStatus := 500
		badRequestErrors, err := es.totalErrorsByStatus(ctx, from, to, &minStatus, &maxStatus, indices...)
		if err != nil {
			out <- result{
				field: "bad_request_errors",
				err:   err,
			}
		} else {
			out <- result{
				field: "bad_request_errors",
				value: badRequestErrors,
			}
		}
	}(out)

	wg.Add(1)
	go func(out chan<- result) {
		defer wg.Done()
		minStatus := 500
		internalServerErrors, err := es.totalErrorsByStatus(ctx, from, to, &minStatus, nil, indices...)
		if err != nil {
			out <- result{
				field: "internal_server_errors",
				err:   err,
			}
		} else {
			out <- result{
				field: "internal_server_errors",
				value: internalServerErrors,
			}
		}
	}(out)

	wg.Add(1)
	go func(out chan<- result) {
		defer wg.Done()
		internalServerErrors, err := es.getTotalSearchesFromLogs(ctx, from, to, nil, indices...)
		if err != nil {
			out <- result{
				field: "total_searches_logs",
				err:   err,
			}
		} else {
			out <- result{
				field: "total_searches_logs",
				value: internalServerErrors,
			}
		}
	}(out)

	wg.Add(1)
	go func(out chan<- result) {
		defer wg.Done()
		minResponseTime := 1000 // 1 second in ms
		internalServerErrors, err := es.getTotalSearchesFromLogs(ctx, from, to, &minResponseTime, indices...)
		if err != nil {
			out <- result{
				field: "total_searches_logs_with_min_response_time",
				err:   err,
			}
		} else {
			out <- result{
				field: "total_searches_logs_with_min_response_time",
				value: internalServerErrors,
			}
		}
	}(out)

	go func() {
		wg.Wait()
		close(out)
	}()

	var totalUniqueSearches, totalUniqueSearchesWithMinCount, popularSearches, totalSearchesLogs, totalSearchesLogsWithMinResponseTime, highResponseTimeSearches, avgClickRatePopularSearches, avgQueryLength, badRequestErrors, internalServerErrors float64
	for result := range out {
		if result.err != nil {
			log.Errorln(logTag, ":", result.err)
			return nil, nil, fmt.Errorf(`cannot fetch value for "%s"`, result.field)
		}
		switch result.field {
		case "total_unique_searches":
			totalUniqueSearches = result.value
		case "total_unique_searches_with_min_count":
			totalUniqueSearchesWithMinCount = result.value
			avgClickRatePopularSearches = result.extraValue
		case "avg_query_length":
			avgQueryLength = result.value
		case "bad_request_errors":
			badRequestErrors = result.value
		case "internal_server_errors":
			internalServerErrors = result.value
		case "total_searches_logs":
			totalSearchesLogs = result.value
		case "total_searches_logs_with_min_response_time":
			totalSearchesLogsWithMinResponseTime = result.value
		default:
			return nil, nil, fmt.Errorf(`illegal field "%s" encountered`, result.field)
		}
	}
	// calculate popular searches
	if totalUniqueSearchesWithMinCount > 0 && totalUniqueSearches > 0 {
		percentage := (totalUniqueSearchesWithMinCount / totalUniqueSearches) * 100
		// If total searches with 100 search count is at least 1% of total searches then set popular searches
		if percentage >= 1 {
			popularSearches = totalUniqueSearchesWithMinCount
		}
	}
	// Calculate high response time searches
	if totalSearchesLogsWithMinResponseTime > 0 && totalSearchesLogs > 0 {
		percentage := (totalSearchesLogsWithMinResponseTime / totalSearchesLogs) * 100
		// If total searches with response time > 1s is at least 10% of total searches then set high response time searches
		if percentage >= 10 {
			highResponseTimeSearches = totalSearchesLogsWithMinResponseTime
		}
	}
	// filters don't have any significance, defined here to just use the existing summary method
	filters := make(map[string]interface{})
	summaryRecord, err := es.getSummary(ctx, from, to, filters, indices...)
	if err != nil {
		return nil, nil, err
	}
	return summaryRecord, &DetailedSummary{
		AvgBounceRate:               summaryRecord.AvgBounceRate,
		AvgUserSessionDuration:      summaryRecord.AvgUserSessionDuration,
		AvgClickRate:                summaryRecord.AvgClickRate,
		AvgClickPosition:            summaryRecord.AvgClickPosition,
		AvgResultClickPosition:      summaryRecord.AvgResultClickPosition,
		AvgSuggestionsClickPosition: summaryRecord.AvgSuggestionsClickPosition,
		AvgSuggestionsClickRate:     summaryRecord.AvgSuggestionsClickRate,
		AvgConversionRate:           summaryRecord.AvgConversionRate,
		TotalResultsCount:           summaryRecord.TotalResultsCount,
		TotalSearches:               summaryRecord.TotalSearches,
		TotalUsers:                  summaryRecord.TotalUsers,
		TotalUserSessions:           summaryRecord.TotalUserSessions,
		TotalBounceUsers:            summaryRecord.TotalBounceUsers,
		TotalClicks:                 summaryRecord.TotalClicks,
		TotalSuggestionsClicks:      summaryRecord.TotalSuggestionsClicks,
		TotalResultsClicks:          summaryRecord.TotalResultsClicks,
		TotalConversions:            summaryRecord.TotalConversions,
		TotalNoResultsSearches:      summaryRecord.TotalNoResultsSearches,
		NoResultsRate:               summaryRecord.NoResultsRate,
		PopularSearches:             popularSearches,
		AvgClickRatePopularSearches: avgClickRatePopularSearches,
		AvgQueryLength:              avgQueryLength,
		InternalServerErrors:        internalServerErrors,
		BadRequestErrors:            badRequestErrors,
		HighResponseTimeSearches:    highResponseTimeSearches,
	}, nil
}

func (es *elasticsearch) getSummary(ctx context.Context, from, to string, filters map[string]interface{}, indices ...string) (*SummaryRecord, error) {
	type result struct {
		field      string
		value      float64
		extraValue float64
		err        error
	}

	var wg sync.WaitGroup
	out := make(chan result)

	wg.Add(1)
	go func(out chan<- result) {
		defer wg.Done()
		totalSearches, err := es.totalSearches(ctx, from, to, filters, indices...)
		if err != nil {
			out <- result{
				field: "total_searches",
				err:   err,
			}
		} else {
			out <- result{
				field: "total_searches",
				value: totalSearches,
			}
		}
	}(out)

	wg.Add(1)
	go func(out chan<- result) {
		defer wg.Done()

		totalResultsCount, err := es.totalResultsCount(ctx, from, to, filters, indices...)
		if err != nil {
			out <- result{
				field: "total_results_count",
				err:   err,
			}
		} else {
			out <- result{
				field: "total_results_count",
				value: totalResultsCount,
			}
		}
	}(out)

	wg.Add(1)
	go func(out chan<- result) {
		defer wg.Done()
		noResultsSearches, err := es.noResultsSearches(ctx, from, to, filters, indices...)
		if err != nil {
			out <- result{
				field: "no_results_searches",
				err:   err,
			}
		} else {
			out <- result{
				field: "no_results_searches",
				value: noResultsSearches,
			}
		}
	}(out)

	wg.Add(1)
	go func(out chan<- result) {
		defer wg.Done()
		totalUsers, err := es.totalUsers(ctx, from, to, filters, indices...)
		if err != nil {
			out <- result{
				field: "total_users",
				err:   err,
			}
		} else {
			out <- result{
				field: "total_users",
				value: totalUsers,
			}
		}
	}(out)

	wg.Add(1)
	go func(out chan<- result) {
		defer wg.Done()
		totalUserSessions, avgDuration, err := es.totalUserSessions(ctx, from, to, filters, indices...)
		if err != nil {
			out <- result{
				field: "total_user_sessions",
				err:   err,
			}
		} else {
			out <- result{
				field:      "total_user_sessions",
				value:      totalUserSessions,
				extraValue: avgDuration,
			}
		}
	}(out)

	wg.Add(1)
	go func(out chan<- result) {
		defer wg.Done()
		totalBounceUsers, err := es.totalBounceUsers(ctx, from, to, filters, indices...)
		if err != nil {
			out <- result{
				field: "total_bounce_users",
				err:   err,
			}
		} else {
			out <- result{
				field: "total_bounce_users",
				value: totalBounceUsers,
			}
		}
	}(out)

	wg.Add(1)
	go func(out chan<- result) {
		defer wg.Done()
		totalClicks, err := es.totalClicks(ctx, from, to, filters, nil, indices...)
		if err != nil {
			out <- result{
				field: "total_clicks",
				err:   err,
			}
		} else {
			out <- result{
				field: "total_clicks",
				value: totalClicks,
			}
		}
	}(out)

	wg.Add(1)
	go func(out chan<- result) {
		defer wg.Done()
		avgResultClickPosition, err := es.avgResultClickPosition(ctx, from, to, filters, nil, indices...)
		if err != nil {
			out <- result{
				field: "avg_result_click_position",
				err:   err,
			}
		} else {
			out <- result{
				field: "avg_result_click_position",
				value: avgResultClickPosition,
			}
		}
	}(out)

	wg.Add(1)
	go func(out chan<- result) {
		defer wg.Done()
		avgSuggestionsClickPosition, err := es.avgSuggestionsClickPosition(ctx, from, to, filters, nil, indices...)
		if err != nil {
			out <- result{
				field: "avg_suggestions_click_position",
				err:   err,
			}
		} else {
			out <- result{
				field: "avg_suggestions_click_position",
				value: avgSuggestionsClickPosition,
			}
		}
	}(out)

	wg.Add(1)
	go func(out chan<- result) {
		defer wg.Done()

		totalSuggestionsClicks, err := es.totalSuggestionsClicks(ctx, from, to, filters, nil, indices...)
		if err != nil {
			out <- result{
				field: "total_suggestions_clicks",
				err:   err,
			}
		} else {
			out <- result{
				field: "total_suggestions_clicks",
				value: totalSuggestionsClicks,
			}
		}
	}(out)

	wg.Add(1)
	go func(out chan<- result) {
		defer wg.Done()
		totalConversions, err := es.totalConversions(ctx, from, to, filters, nil, indices...)
		if err != nil {
			out <- result{
				field: "total_conversions",
				err:   err,
			}
		} else {
			out <- result{
				field: "total_conversions",
				value: totalConversions,
			}
		}
	}(out)

	go func() {
		wg.Wait()
		close(out)
	}()

	var totalSearches, totalResultsCount, totalResultClicks, totalConversions, totalSuggestionsClicks, totalUsers, totalUserSessions, avgDuration, totalBounceUsers, noResultsSearches, avgResultClickPosition, avgSuggestionsClickPosition float64
	for result := range out {
		if result.err != nil {
			log.Errorln(logTag, ":", result.err)
			return nil, fmt.Errorf(`cannot fetch value for "%s"`, result.field)
		}
		switch result.field {
		case "total_results_count":
			totalResultsCount = result.value
		case "total_users":
			totalUsers = result.value
		case "total_bounce_users":
			totalBounceUsers = result.value
		case "total_user_sessions":
			totalUserSessions = result.value
			avgDuration = result.extraValue
		case "total_searches":
			totalSearches = result.value
		case "total_clicks":
			totalResultClicks = result.value
		case "total_suggestions_clicks":
			totalSuggestionsClicks = result.value
		case "total_conversions":
			totalConversions = result.value
		case "no_results_searches":
			noResultsSearches = result.value
		case "avg_result_click_position":
			avgResultClickPosition = result.value
		case "avg_suggestions_click_position":
			avgSuggestionsClickPosition = result.value
		default:
			return nil, fmt.Errorf(`illegal field "%s" encountered`, result.field)
		}
	}
	totalClicks := totalResultClicks + totalSuggestionsClicks

	var avgClickRate, avgConversionRate, avgNoResultsRate, avgSuggestionsClickRate, avgBounceRate, avgClickPosition float64
	if totalSearches != 0 {
		avgSuggestionsClickRate = totalSuggestionsClicks / totalSearches * 100
		avgClickRate = totalClicks / totalSearches * 100
		avgConversionRate = totalConversions / totalSearches * 100
		avgNoResultsRate = noResultsSearches / totalSearches * 100
	}
	if totalUserSessions != 0 {
		avgBounceRate = totalBounceUsers / totalUserSessions * 100
	}

	if totalClicks != 0 {
		avgClickPosition = ((totalResultClicks * avgResultClickPosition) + (totalSuggestionsClicks * avgSuggestionsClickPosition)) / totalClicks
	}

	summary := SummaryRecord{
		TotalResultsCount:           util.WithPrecision(totalResultsCount, 2),
		TotalSearches:               util.WithPrecision(totalSearches, 2),
		TotalUsers:                  util.WithPrecision(totalUsers, 2),
		TotalUserSessions:           util.WithPrecision(totalUserSessions, 2),
		TotalBounceUsers:            util.WithPrecision(totalBounceUsers, 2),
		AvgBounceRate:               util.WithPrecision(avgBounceRate, 2),
		AvgUserSessionDuration:      util.WithPrecision(avgDuration, 2),
		AvgClickPosition:            util.WithPrecision(avgClickPosition, 2),
		AvgResultClickPosition:      util.WithPrecision(avgResultClickPosition, 2),
		AvgSuggestionsClickPosition: util.WithPrecision(avgSuggestionsClickPosition, 2),
		TotalClicks:                 util.WithPrecision(totalClicks, 2),
		TotalSuggestionsClicks:      util.WithPrecision(totalSuggestionsClicks, 2),
		TotalResultsClicks:          util.WithPrecision(totalResultClicks, 2),
		AvgClickRate:                util.WithPrecision(avgClickRate, 2),
		AvgSuggestionsClickRate:     util.WithPrecision(avgSuggestionsClickRate, 2),
		TotalConversions:            util.WithPrecision(totalConversions, 2),
		AvgConversionRate:           util.WithPrecision(avgConversionRate, 2),
		TotalNoResultsSearches:      util.WithPrecision(noResultsSearches, 2),
		NoResultsRate:               util.WithPrecision(avgNoResultsRate, 2),
	}

	return &summary, nil
}

func (es *elasticsearch) totalSearches(ctx context.Context, from, to string, filters map[string]interface{}, indices ...string) (float64, error) {
	return es.totalSearchesEs7(ctx, from, to, filters, indices...)
}

func (es *elasticsearch) totalSearchesWithMinHundredCount(ctx context.Context, from, to string, filters map[string]interface{}, indices ...string) (float64, error) {
	return es.totalSearchesEs7(ctx, from, to, filters, indices...)
}

func (es *elasticsearch) totalResultsCount(ctx context.Context, from, to string, filters map[string]interface{}, indices ...string) (float64, error) {
	return es.totalResultsCountEs7(ctx, from, to, filters, indices...)
}

func (es *elasticsearch) totalUniqueResultsCount(ctx context.Context, from, to string, filters map[string]interface{}, indices ...string) (float64, error) {
	return es.totalUniqueResultsCountEs7(ctx, from, to, filters, indices...)
}

func (es *elasticsearch) noResultsSearches(ctx context.Context, from, to string, filters map[string]interface{}, indices ...string) (float64, error) {
	return es.noResultsSearchesEs7(ctx, from, to, filters, indices...)
}

func (es *elasticsearch) totalUsers(ctx context.Context, from, to string, filters map[string]interface{}, indices ...string) (float64, error) {
	return es.totalUsersEs7(ctx, from, to, filters, indices...)
}

func (es *elasticsearch) totalUserSessions(ctx context.Context, from, to string, filters map[string]interface{}, indices ...string) (float64, float64, error) {
	return es.totalUserSessionsEs7(ctx, from, to, filters, indices...)
}

func (es *elasticsearch) getActiveUserSessions(ctx context.Context) ([]ActiveUserSessionES, error) {
	return es.getActiveUserSessionsEs7(ctx)
}

func (es *elasticsearch) totalBounceUsers(ctx context.Context, from, to string, filters map[string]interface{}, indices ...string) (float64, error) {
	return es.totalBounceUsersEs7(ctx, from, to, filters, indices...)
}

func (es *elasticsearch) totalConversions(ctx context.Context, from, to string, filters map[string]interface{}, queryFilters *[]QueryFilter, indices ...string) (float64, error) {
	return es.totalConversionsEs7(ctx, from, to, filters, queryFilters, indices...)
}

func (es *elasticsearch) totalClicks(ctx context.Context, from, to string, filters map[string]interface{}, queryFilters *[]QueryFilter, indices ...string) (float64, error) {
	return es.totalClicksEs7(ctx, from, to, filters, queryFilters, indices...)
}

func (es *elasticsearch) avgQueryLength(ctx context.Context, from, to string, indices ...string) (float64, error) {
	return es.avgQueryLengthEs7(ctx, from, to, indices...)
}

func (es *elasticsearch) avgResultClickPosition(ctx context.Context, from, to string, filters map[string]interface{}, queryFilters *[]QueryFilter, indices ...string) (float64, error) {
	return es.avgResultClickPositionEs7(ctx, from, to, filters, queryFilters, indices...)
}

func (es *elasticsearch) avgSuggestionsClickPosition(ctx context.Context, from, to string, filters map[string]interface{}, queryFilters *[]QueryFilter, indices ...string) (float64, error) {
	return es.avgSuggestionsClickPositionEs7(ctx, from, to, filters, queryFilters, indices...)
}
func (es *elasticsearch) totalSuggestionsClicks(ctx context.Context, from, to string, filters map[string]interface{}, queryFilters *[]QueryFilter, indices ...string) (float64, error) {
	return es.totalSuggestionsClicksEs7(ctx, from, to, filters, queryFilters, indices...)
}

func (es *elasticsearch) totalErrorsByStatus(ctx context.Context, from, to string, minStatusCode *int, maxStatusCode *int, indices ...string) (float64, error) {
	return es.totalErrorsByStatusEs7(ctx, from, to, minStatusCode, maxStatusCode, indices...)
}

func (es *elasticsearch) getAdminUsers(ctx context.Context) (*[]User, error) {
	return es.getAdminUsersEs7(ctx)
}

func (es *elasticsearch) searchHistogram(ctx context.Context, queryParams QueryParams, filters map[string]interface{}, indices ...string) (interface{}, error) {
	raw, err := es.searchHistogramRaw(ctx, queryParams, filters, indices...)
	if err != nil {
		return []interface{}{}, err
	}

	var response struct {
		SearchHistogram []map[string]interface{} `json:"search_histogram"`
	}
	err = json.Unmarshal(raw, &response)
	if err != nil {
		return []interface{}{}, err
	}

	return response, nil
}

func (es *elasticsearch) searchHistogramRaw(ctx context.Context, queryParams QueryParams, filters map[string]interface{}, indices ...string) ([]byte, error) {
	return es.searchHistogramRawEs7(ctx, queryParams, filters, indices...)
}

func (es *elasticsearch) queryHistogramRaw(ctx context.Context, queryParams QueryParams, query string, filters map[string]interface{}, indices ...string) ([]map[string]interface{}, error) {
	return es.queryHistogramRawEs7(ctx, queryParams, query, filters, indices...)
}
func (es *elasticsearch) getRequestDistributionWithSummary(ctx context.Context, queryParams QueryParams, interval string, size int, filters map[string]interface{}, indices ...string) ([]byte, error) {
	type result struct {
		field string
		value interface{}
		err   error
	}
	var wg sync.WaitGroup
	out := make(chan result)

	wg.Add(1)
	go func(out chan<- result) {
		defer wg.Done()
		geoRequestsDistribution, err := es.getRequestDistribution(ctx, queryParams, interval, size, filters, indices...)
		if err != nil {
			out <- result{
				field: "request_distribution",
				err:   err,
			}
		} else {
			out <- result{
				field: "request_distribution",
				value: geoRequestsDistribution,
			}
		}
	}(out)

	wg.Add(1)
	go func(out chan<- result) {
		defer wg.Done()
		totalRequests, err := es.getTotalRequests(ctx, queryParams.From, queryParams.To, 0, indices...)
		if err != nil {
			out <- result{
				field: "total_requests",
				err:   err,
			}
		} else {
			out <- result{
				field: "total_requests",
				value: totalRequests,
			}
		}
	}(out)

	wg.Add(1)
	go func(out chan<- result) {
		defer wg.Done()
		totalRequests, err := es.getTotalRequests(ctx, queryParams.From, queryParams.To, 200, indices...)
		if err != nil {
			out <- result{
				field: "total_200",
				err:   err,
			}
		} else {
			out <- result{
				field: "total_200",
				value: totalRequests,
			}
		}
	}(out)

	wg.Add(1)
	go func(out chan<- result) {
		defer wg.Done()
		totalRequests, err := es.getTotalRequests(ctx, queryParams.From, queryParams.To, 201, indices...)
		if err != nil {
			out <- result{
				field: "total_201",
				err:   err,
			}
		} else {
			out <- result{
				field: "total_201",
				value: totalRequests,
			}
		}
	}(out)

	wg.Add(1)
	go func(out chan<- result) {
		defer wg.Done()
		totalRequests, err := es.getTotalRequests(ctx, queryParams.From, queryParams.To, 400, indices...)
		if err != nil {
			out <- result{
				field: "total_400",
				err:   err,
			}
		} else {
			out <- result{
				field: "total_400",
				value: totalRequests,
			}
		}
	}(out)

	wg.Add(1)
	go func(out chan<- result) {
		defer wg.Done()
		totalRequests, err := es.getTotalRequests(ctx, queryParams.From, queryParams.To, 401, indices...)
		if err != nil {
			out <- result{
				field: "total_401",
				err:   err,
			}
		} else {
			out <- result{
				field: "total_401",
				value: totalRequests,
			}
		}
	}(out)

	go func() {
		wg.Wait()
		close(out)
	}()

	var requestDistributionResponse = make(map[string]interface{})

	for result := range out {
		if result.err != nil {
			log.Errorln(logTag, ":", result.err)
			return nil, fmt.Errorf(`cannot fetch value for "%s"`, result.field)
		}
		switch result.field {
		case "request_distribution":
			var requestDistribution map[string]interface{}
			responseInBytes, ok := result.value.([]byte)
			if !ok {
				return nil, fmt.Errorf(`cannot parse value for "%s"`, result.field)
			}
			err := json.Unmarshal(responseInBytes, &requestDistribution)
			if err != nil {
				log.Errorln(logTag, ":", err)
				return nil, fmt.Errorf(`cannot un-marshall value for "%s"`, result.field)
			}
			for k, v := range requestDistribution {
				requestDistributionResponse[k] = v
			}
		case "total_requests", "total_200", "total_201", "total_400", "total_401":
			requestDistributionResponse[result.field] = result.value
		default:
			return nil, fmt.Errorf(`illegal field "%s" encountered`, result.field)
		}
	}
	return json.Marshal(requestDistributionResponse)
}

func (es *elasticsearch) getRequestDistribution(ctx context.Context, queryParams QueryParams, interval string, size int, filters map[string]interface{}, indices ...string) ([]byte, error) {
	return es.getRequestDistributionEs7(ctx, queryParams, interval, size, filters, indices...)
}

func (es *elasticsearch) getTotalRequests(ctx context.Context, from, to string, code int, indices ...string) (int64, error) {
	return es.getTotalRequestsEs7(ctx, from, to, code, indices...)
}

func (es *elasticsearch) geoTotalCountries(ctx context.Context, from, to string, filters map[string]interface{}, indices ...string) (float64, error) {
	return es.geoTotalCountriesEs7(ctx, from, to, filters, indices...)
}

func (es *elasticsearch) getFilterLabels(ctx context.Context) ([]byte, error) {
	indices := es.getSortedIndices(es.analyticsIndex)
	var indexName = es.analyticsIndex
	if len(indices) > 0 {
		indexName = indices[len(indices)-1]
	}
	response, err := util.GetClient7().GetMapping().Index(indexName).
		Do(ctx)
	if err != nil {
		return nil, err
	}
	var fieldMap map[string]interface{}
	if indexName != "" {
		if response[indexName] != nil && response[indexName].(map[string]interface{})["mappings"] != nil {
			switch util.GetVersion() {
			case 6:
				if response[indexName].(map[string]interface{})["mappings"].(map[string]interface{})["_doc"] != nil && response[indexName].(map[string]interface{})["mappings"].(map[string]interface{})["_doc"].(map[string]interface{})["properties"] != nil {
					fieldMap = response[indexName].(map[string]interface{})["mappings"].(map[string]interface{})["_doc"].(map[string]interface{})["properties"].(map[string]interface{})
				}
			default:
				if response[indexName].(map[string]interface{})["mappings"].(map[string]interface{})["properties"] != nil {
					fieldMap = response[indexName].(map[string]interface{})["mappings"].(map[string]interface{})["properties"].(map[string]interface{})
				}
			}
		}
	}
	keys := make([]string, 0, len(fieldMap))
	for key := range fieldMap {
		if strings.HasPrefix(key, CustomEventsPrefix) {
			keys = append(keys, trimEventPrefix(key))
		}
	}
	filterLabels := make(map[string]interface{})
	filterLabels["filter_labels"] = keys

	return json.Marshal(filterLabels)
}

func (es *elasticsearch) getFilterValues(ctx context.Context, label, prefix string, indices ...string) ([]byte, error) {
	return es.getFilterValuesEs7(ctx, label, prefix, indices...)
}

func (es *elasticsearch) getSortedIndices(alias string) []string {
	ctx := context.Background()
	indices, err := util.GetClient7().Aliases().Index(es.analyticsIndex).
		Do(ctx)
	if err != nil {
		log.Errorln(logTag, ": rollover cronjob error getting indices", err)
	}

	rolloverIndices := []string{}
	for index := range indices.Indices {
		rolloverIndices = append(rolloverIndices, index)
	}

	sort.Strings(rolloverIndices)

	return rolloverIndices
}

func (es *elasticsearch) rolloverIndexJob(alias string) {
	ctx := context.Background()

	var mappings map[string]interface{}
	err2 := json.Unmarshal([]byte(getAnalyticsMappings()), &mappings)
	if err2 != nil {
		log.Errorln(logTag, "error while un-marshalling mappings", err2)
		return
	}
	rolloverConditions := make(map[string]interface{})
	rolloverConfiguration := fmt.Sprintf(rolloverConfig, "30d", 100000, "5gb")
	if util.IsProductionPlan() {
		rolloverConfiguration = fmt.Sprintf(rolloverConfig, "30d", 10000000, "10gb")
	}

	json.Unmarshal([]byte(rolloverConfiguration), &rolloverConditions)

	shouldRollover := true
	if util.IsServerless() {
		var conditionErr error
		shouldRollover, conditionErr = util.WriteIndexMeetsRolloverConditions(ctx, alias, rolloverConditions)
		if conditionErr != nil {
			log.Errorln(logTag, ": serverless rollover condition check error, skipping rollover", conditionErr)
			shouldRollover = false
		} else if !shouldRollover {
			log.Println(logTag, ": serverless rollover skipped, conditions not met for alias", alias)
		}
	}

	if shouldRollover {
		rolloverSvc := util.NewIndicesRolloverService(alias, rolloverConditions).
			Mappings(mappings)
		if settings := util.RolloverIndexSettings(2); len(settings) > 0 {
			rolloverSvc = rolloverSvc.Settings(settings)
		}
		rolloverService, err := rolloverSvc.Do(ctx)
		if err != nil {
			log.Printf("%s: error while creating a rollover service %s %v", logTag, alias, err)
			return
		}
		log.Println(logTag, ": rollover res oldIndex", rolloverService.OldIndex)
		log.Println(logTag, ": rollover res newIndex", rolloverService.NewIndex)
		log.Println(logTag, ": rollover res isRolledover", rolloverService.RolledOver)

		if rolloverService.RolledOver {
			classify.SetIndexAlias(rolloverService.NewIndex, alias)
			classify.SetAliasIndex(alias, rolloverService.NewIndex)
		}
	}

	// We cannot rely on rollover service response here,
	// Because it returns rollover as false when we restart reactivesearch.
	// To preserve the last 2 index and delete others:
	// -> cat all the indices with .analytics-*
	// -> if count is > 2
	//   -> sort them based on -[Number]
	//   -> preserve last 2 and delete all
	// -> else do not delete any index

	// cat all the indices starting with `${alias}-Number` pattern
	rolloverIndices := es.getSortedIndices(alias)

	if len(rolloverIndices) > 2 {
		// ignore last 2 indices
		rolloverIndices = rolloverIndices[:len(rolloverIndices)-2]

		log.Println(logTag, ": rollover cronjob, indices to delete", rolloverIndices)
		_, deleteErr := util.GetClient7().DeleteIndex(strings.Join(rolloverIndices, ",")).Do(ctx)
		if deleteErr != nil {
			log.Errorln(logTag, ": rollover cronjob, error while deleting indices", deleteErr)
		}
	}
}

func (es *elasticsearch) getInsights(ctx context.Context, indexName string, disableFilterByStatus bool) (GetInsight, error) {
	// Use the last month for the insights
	firstOfMonth, lastOfMonth := getMonthRange(-1)
	// Fix the `from` and `to` to first and last date of the month
	from := firstOfMonth.Format(time.RFC3339)
	to := lastOfMonth.Format(time.RFC3339)

	indices, err := index.FromContext(ctx)
	// ignore error because indices won't present while reporting analytics
	if err != nil {
		log.Errorln(logTag, ":", err)
	}
	docID := getInsightStatusID(indexName)
	// ignore error because there may be a case that document not present for docID
	insightStatus, _ := es.getInsightStatus(ctx, docID)
	summary, detailedSummary, err := es.getDetailedSummary(ctx, from, to, indices...)
	if err != nil {
		msg := "error occurred while fetching insights"
		log.Errorln(logTag, ":", err)
		return GetInsight{}, fmt.Errorf(msg)
	}
	var insights = make([]InsightResponseType, 0)
	var savedInsights = make([]InsightResponseType, 0)
	var readInsights = make([]InsightResponseType, 0)
	for _, insightConfig := range InsightConfig {
		if insightConfig.Insight.Condition == "" {
			continue
		}
		program, err := expr.Compile(insightConfig.Insight.Condition, expr.Env(*detailedSummary), expr.AsBool())
		if err != nil {
			log.Errorln(logTag, "error encountered while validating condition expression for "+insightConfig.ID.String()+" type", err)
			continue
		}
		output, err := expr.Run(program, *detailedSummary)
		if err != nil {
			log.Errorln(logTag, "error encountered while running condition expression for "+insightConfig.ID.String()+" type", err)
			continue
		}
		outputAsBool, ok := output.(bool)
		if !ok {
			log.Errorln(logTag, "error encountered while parsing the condition expression output for "+insightConfig.ID.String()+" type", err)
			continue
		}
		if outputAsBool {
			// Evaluate title expression
			titleEnvironments := getInsightEnvironmentsFromSummary(*detailedSummary)

			title, err := evaluateStringExpression(insightConfig.Insight.Title, "title", insightConfig.ID.String(), titleEnvironments)
			if err != nil {
				log.Errorln(logTag, ":", err)
			}

			description, err2 := evaluateStringExpression(insightConfig.Insight.Description, "description", insightConfig.ID.String(), titleEnvironments)
			if err2 != nil {
				log.Errorln(logTag, ":", err2)
			}

			insight := InsightResponseType{
				ID: insightConfig.ID,
				Insight: InsightResponse{
					Title:           title,
					Description:     description,
					Recommendations: insightConfig.Insight.Recommendations,
					ShortLink:       insightConfig.Insight.ShortLink,
				},
			}
			if disableFilterByStatus {
				insights = append(insights, insight)
			} else {
				// Filter deleted insights
				if !isInsightExist(insightStatus.Deleted, insightConfig.ID) {
					if isInsightExist(insightStatus.Saved, insightConfig.ID) {
						// Add saved insights
						savedInsights = append(savedInsights, insight)
					} else if isInsightExist(insightStatus.Read, insightConfig.ID) {
						// Add read insights
						readInsights = append(readInsights, insight)
					} else {
						// Add fresh insights
						insights = append(insights, insight)
					}
				}
			}
		}
	}
	return GetInsight{
		Insights:      insights,
		SavedInsights: savedInsights,
		ReadInsights:  readInsights,
		Summary:       *summary,
	}, nil
}

func (es *elasticsearch) reportAnalyticsToUsers() {
	reportAnalyticsRequest := ReportAnalyticsRequest{}
	ctx := context.Background()
	adminUsers, err := es.getAdminUsers(ctx)
	if err != nil {
		log.Errorln(logTag, ":", err)
		return
	}
	var adminUserEmails []string
	for _, user := range *adminUsers {
		if user.Email != "" {
			adminUserEmails = append(adminUserEmails, user.Email)
		}
	}
	// filters don't have any significance, defined here to just use the existing summary method
	filters := make(map[string]interface{})
	// report insights and comparison for valid plan users
	if util.ValidatePlans(validPlans, util.GetFeatureCustomEvents()) {
		insights, err := es.getInsights(ctx, "", true)
		if err != nil {
			log.Errorln(logTag, ":", err)
			return
		}
		// Use the second last month for the summary comparison
		firstOfSecondLastMonth, lastOfSecondLastMonth := getMonthRange(-2)
		// Fix the `from` and `to` to first and last date of the month
		compareFrom := firstOfSecondLastMonth.Format(time.RFC3339)
		compareTo := lastOfSecondLastMonth.Format(time.RFC3339)
		// extract summary for last timeframe
		summaryForComparison, err := es.getSummary(ctx, compareFrom, compareTo, filters)
		if err != nil {
			log.Errorln(logTag, ":", err)
			return
		}

		var parsedInsights []InsightReportAnalytics
		for _, insight := range insights.Insights {
			parsedInsights = append(parsedInsights, InsightReportAnalytics{
				Title:       insight.Insight.Title,
				Description: insight.Insight.Description,
				Link:        getInsightsEmailLink(insight.ID.String()),
			})
		}
		summary := insights.Summary

		summaryComparison := SummaryComparison{
			AvgBounceRateComparison:               calculateSummaryComparison(summaryForComparison.AvgBounceRate, summary.AvgBounceRate),
			AvgBounceRatePrevious:                 summaryForComparison.AvgBounceRate,
			AvgUserSessionDurationComparison:      calculateSummaryComparison(summaryForComparison.AvgUserSessionDuration, summary.AvgUserSessionDuration),
			AvgUserSessionDurationPrevious:        summaryForComparison.AvgUserSessionDuration,
			AvgClickRateComparison:                calculateSummaryComparison(summaryForComparison.AvgClickRate, summary.AvgClickRate),
			AvgClickRatePrevious:                  summaryForComparison.AvgClickRate,
			AvgClickPositionComparison:            calculateSummaryComparison(summaryForComparison.AvgClickPosition, summary.AvgClickPosition),
			AvgClickPositionPrevious:              summaryForComparison.AvgClickPosition,
			AvgResultClickPositionComparison:      calculateSummaryComparison(summaryForComparison.AvgResultClickPosition, summary.AvgResultClickPosition),
			AvgResultClickPositionPrevious:        summaryForComparison.AvgResultClickPosition,
			AvgSuggestionsClickPositionComparison: calculateSummaryComparison(summaryForComparison.AvgSuggestionsClickPosition, summary.AvgSuggestionsClickPosition),
			AvgSuggestionsClickPositionPrevious:   summaryForComparison.AvgSuggestionsClickPosition,
			AvgSuggestionsClickRateComparison:     calculateSummaryComparison(summaryForComparison.AvgSuggestionsClickRate, summary.AvgSuggestionsClickRate),
			AvgSuggestionsClickRatePrevious:       summaryForComparison.AvgSuggestionsClickRate,
			AvgConversionRateComparison:           calculateSummaryComparison(summaryForComparison.AvgConversionRate, summary.AvgConversionRate),
			AvgConversionRatePrevious:             summaryForComparison.AvgConversionRate,
			TotalResultsCountComparison:           calculateSummaryComparison(summaryForComparison.TotalResultsCount, summary.TotalResultsCount),
			TotalResultsCountPrevious:             summaryForComparison.TotalResultsCount,
			TotalSearchesComparison:               calculateSummaryComparison(summaryForComparison.TotalSearches, summary.TotalSearches),
			TotalSearchesPrevious:                 summaryForComparison.TotalSearches,
			TotalUsersComparison:                  calculateSummaryComparison(summaryForComparison.TotalUsers, summary.TotalUsers),
			TotalUsersPrevious:                    summaryForComparison.TotalUsers,
			TotalUserSessionsComparison:           calculateSummaryComparison(summaryForComparison.TotalUserSessions, summary.TotalUserSessions),
			TotalUserSessionsPrevious:             summaryForComparison.TotalUserSessions,
			TotalBounceUsersComparison:            calculateSummaryComparison(summaryForComparison.TotalBounceUsers, summary.TotalBounceUsers),
			TotalBounceUsersPrevious:              summaryForComparison.TotalBounceUsers,
			TotalClicksComparison:                 calculateSummaryComparison(summaryForComparison.TotalClicks, summary.TotalClicks),
			TotalClicksPrevious:                   summaryForComparison.TotalClicks,
			TotalSuggestionsClicksComparison:      calculateSummaryComparison(summaryForComparison.TotalSuggestionsClicks, summary.TotalSuggestionsClicks),
			TotalSuggestionsClicksPrevious:        summaryForComparison.TotalSuggestionsClicks,
			TotalResultsClicksComparison:          calculateSummaryComparison(summaryForComparison.TotalResultsClicks, summary.TotalResultsClicks),
			TotalResultsClicksPrevious:            summaryForComparison.TotalResultsClicks,
			TotalConversionsComparison:            calculateSummaryComparison(summaryForComparison.TotalConversions, summary.TotalConversions),
			TotalConversionsPrevious:              summaryForComparison.TotalConversions,
			TotalNoResultsSearchesComparison:      calculateSummaryComparison(summaryForComparison.TotalNoResultsSearches, summary.TotalNoResultsSearches),
			TotalNoResultsSearchesPrevious:        summaryForComparison.TotalNoResultsSearches,
			NoResultsRateComparison:               calculateSummaryComparison(summaryForComparison.NoResultsRate, summary.NoResultsRate),
			NoResultsRatePrevious:                 summaryForComparison.NoResultsRate,
		}

		// Use the last month for the insights date range
		firstOfLastMonth, lastOfLastMonth := getMonthRange(-1)

		insightsReportAnalytics := InsightsEnvironments{
			DateRange:        firstOfLastMonth.Format("Jan 2") + " - " + lastOfLastMonth.Format("Jan 2"),
			Insights:         parsedInsights,
			NumberOfInsights: len(parsedInsights),
		}
		reportAnalyticsRequest = ReportAnalyticsRequest{
			Summary:             summary,
			Insights:            insightsReportAnalytics,
			SummaryComparison:   summaryComparison,
			Users:               adminUserEmails,
			FeatureCustomEvents: true,
		}
	} else {
		// only send summary for normal users
		// Get summary for previous month
		from, to := getMonthRange(-1)
		summary, err := es.getSummary(ctx, from.Format(time.RFC3339), to.Format(time.RFC3339), filters)
		if err != nil {
			log.Errorln(logTag, ":", err)
			return
		}
		if summary != nil {
			reportAnalyticsRequest = ReportAnalyticsRequest{
				Summary:             *summary,
				Users:               adminUserEmails,
				FeatureCustomEvents: false,
			}
		} else {
			log.Errorln(logTag, ":", "Summary not found")
			return
		}
	}
	// Send request to accapi
	arcID, err := util.GetArcID()
	if err != nil {
		log.Errorln(logTag, ":", err)
		return
	}
	marshalledRequest, err := json.Marshal(reportAnalyticsRequest)
	if err != nil {
		log.Errorln("error while marshalling req body:", err)
		return
	}
	req, err := http.NewRequest(http.MethodPost, util.ACCAPI+"arc/"+arcID+"/report_analytics", bytes.NewBuffer(marshalledRequest))
	if err != nil {
		log.Errorln(logTag, ":", err)
		return
	}
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("cache-control", "no-cache")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Errorln(logTag, ":", err)
		return
	}
	if res.StatusCode != http.StatusOK {
		log.Errorln(logTag, ":", "error encountered while reporting analytics")
		return
	}
	log.Println(logTag, "Successfully sent analytics insights email")
}

func (es *elasticsearch) deleteOldMetricBeatIndices() {
	log.Infoln(logTag, "Running cron job to delete metricbeat indices")
	ctx := context.Background()
	indices, err := util.GetClient7().CatIndices().Index("metricbeat-*").Do(ctx)
	if err != nil {
		log.Errorln(logTag, ": error getting metricbeat indices", err)
		return
	}

	currentTime := time.Now()
	dateFormat := "2006.01.02"
	datesToPreserve := []string{}

	// check if index with current date is present.
	// this is required it might not be true that when cron job runs
	// new metricbeat for that date is create so me we start deleting from today-(2/8) days
	hasTodayIndice := false
	for _, catResRow := range indices {
		if strings.Contains(catResRow.Index, currentTime.Format(dateFormat)) {
			hasTodayIndice = true
			break
		}
	}

	startingIndex := 0
	endingIndex := 2

	if !util.IsProductionPlan() {
		if !hasTodayIndice {
			startingIndex = 1
			endingIndex = 3
		}
		for i := startingIndex; i < endingIndex; i++ {
			datesToPreserve = append(datesToPreserve, currentTime.AddDate(0, 0, -i).Format(dateFormat))
		}
	} else {
		endingIndex = 8
		if !hasTodayIndice {
			startingIndex = 1
			endingIndex = 9
		}
		for i := startingIndex; i < endingIndex; i++ {
			datesToPreserve = append(datesToPreserve, currentTime.AddDate(0, 0, -i).Format(dateFormat))
		}
	}

	indicesToDelete := []string{}

	for _, catResRow := range indices {
		splitIndex := strings.Split(catResRow.Index, "-")
		datePart := splitIndex[len(splitIndex)-1]
		if !contains(datesToPreserve, datePart) {
			indicesToDelete = append(indicesToDelete, catResRow.Index)
		}
	}
	log.Infoln("indices to delete", indicesToDelete)

	if len(indicesToDelete) > 0 {
		_, err = util.GetClient7().DeleteIndex(strings.Join(indicesToDelete, ",")).Do(ctx)
		if err != nil {
			log.Errorln(logTag, ":error while deleting metricbeat indices", err)
		}
	}
}

func (es *elasticsearch) getAnalyticsDocument(ctx context.Context, docId string) (*analyticsrequest.Record, error) {
	var record *analyticsrequest.Record
	analyticIndex := es.analyticsIndex
	sortedIndices := es.getSortedIndices(es.analyticsIndex)
	if len(sortedIndices) > 0 {
		analyticIndex = sortedIndices[len(sortedIndices)-1]
	}
	result, err := util.GetClient7().Get().Index(analyticIndex).Id(docId).Do(ctx)
	if err != nil {
		log.Errorln(logTag, ":", err)
		return nil, err
	}
	err2 := json.Unmarshal(result.Source, &record)
	if err2 != nil {
		log.Errorln(logTag, ":", err2)
		return nil, err2
	}
	var resultMap map[string]interface{}
	err3 := json.Unmarshal(result.Source, &resultMap)
	if err3 != nil {
		log.Errorln(logTag, ":", err3)
		return nil, err3
	}
	// apply custom events
	var customEvents = make([]analyticsrequest.CustomEvent, 0)
	for k, v := range resultMap {
		if strings.HasPrefix(k, CustomEventsPrefix) {
			customEvents = append(customEvents, analyticsrequest.CustomEvent{
				Key:   trimEventPrefix(k),
				Value: v,
			})
		}
	}
	record.CustomEvents = customEvents
	return record, nil
}

func (es *elasticsearch) getSavedSearch(ctx context.Context, docId string) (*SavedSearchES, error) {
	result, err := util.GetClient7().Get().Index(es.savedSearchesIndex).Id(docId).Do(ctx)
	if err != nil {
		return nil, err
	}
	var record *SavedSearchES
	err2 := json.Unmarshal(result.Source, &record)
	if err2 != nil {
		return nil, err2
	}
	return record, nil
}

func (es *elasticsearch) getFavorite(ctx context.Context, docId string) (*FavoriteES, error) {
	result, err := util.GetClient7().Get().Index(es.favoritesIndex).Id(docId).Do(ctx)
	if err != nil {
		return nil, err
	}
	var record *FavoriteES
	err2 := json.Unmarshal(result.Source, &record)
	if err2 != nil {
		return nil, err2
	}
	return record, nil
}

func (es *elasticsearch) updateSavedSearch(ctx context.Context, record SavedSearchRequest) *Error {
	analyticsRecord, err := es.getAnalyticsDocument(ctx, *record.QueryId)
	if err != nil || analyticsRecord == nil {
		return &Error{
			message: err,
			code:    http.StatusNotFound,
		}
	}
	var createdAt, updatedAt *int64
	t := time.Now()
	currentTime := t.Unix()
	timeStamp := t.Format(time.RFC3339)
	// check saved search existence
	savedSearch, _ := es.getSavedSearch(ctx, *record.SavedSearchId)
	if savedSearch != nil {
		createdAt = savedSearch.CreatedAt
		updatedAt = &currentTime
	} else {
		createdAt = &currentTime
	}
	var searchState interface{}
	searchState = analyticsRecord.SearchState
	savedSearchESRecord := SavedSearchES{
		QueryId:         record.QueryId,
		SavedSearchId:   record.SavedSearchId,
		SavedSearchName: record.SavedSearchName,
		SavedSearchMeta: record.SavedSearchMeta,
		UserId:          record.UserId,
		CustomEvents:    record.CustomEvents,
		CreatedAt:       createdAt,
		UpdatedAt:       updatedAt,
		SearchState: querytranslate.Endpoint{
			URL:    analyticsRecord.URL,
			Method: analyticsRecord.Method,
			Body:   &searchState,
		},
		TimeStamp:              &timeStamp,
		SearchQuery:            analyticsRecord.SearchQuery,
		SearchQueryCharsLength: analyticsRecord.QueryLength,
	}
	// apply analytics
	if savedSearchESRecord.UserId == nil {
		savedSearchESRecord.UserId = &analyticsRecord.UserID
	}
	// apply custom events
	customEvents := make(map[string]interface{})
	for _, v := range analyticsRecord.CustomEvents {
		customEvents[CustomEventsPrefix+v.Key] = v.Value
	}
	if savedSearchESRecord.CustomEvents != nil {
		for k, v := range *savedSearchESRecord.CustomEvents {
			customEvents[CustomEventsPrefix+k] = v
		}
	}
	var savedSearchMap map[string]interface{}
	marshalled, _ := json.Marshal(savedSearchESRecord)
	json.Unmarshal(marshalled, &savedSearchMap)
	delete(savedSearchMap, "customEvents")
	// Add custom events as field
	for k, v := range customEvents {
		savedSearchMap[k] = v
	}
	_, err2 := util.GetClient7().
		Index().
		Index(es.savedSearchesIndex).
		BodyJson(savedSearchMap).
		Refresh("wait_for").
		Id(*savedSearchESRecord.SavedSearchId).
		Do(ctx)
	if err2 != nil {
		log.Errorln(logTag, ":", err2)
		return &Error{
			message: err2,
		}
	}
	return nil
}

type SavedSearchesFilters struct {
	FromTimeStamp string
	ToTimeStamp   string
	TimeZone      string
	Size          int
	MinChars      *int
	CustomEvents  map[string]interface{}
}

func (es *elasticsearch) getSavedSearches(ctx context.Context, filters SavedSearchesFilters) ([]SavedSearchES, error) {
	var savedSearches = make([]SavedSearchES, 0)
	duration := escompat.NewRangeQuery("timestamp").
		Gte(filters.FromTimeStamp).
		Lte(filters.ToTimeStamp).
		TimeZone(filters.TimeZone)

	query := es7.NewBoolQuery().Filter(duration)

	if filters.MinChars != nil {
		minCharQuery := escompat.NewRangeQuery("search_characters_length").Gte(*filters.MinChars)
		query.Filter(minCharQuery)
	}

	applyCustomEventsEs7(query, filters.CustomEvents)

	results, err := util.
		GetClient7().
		Search(es.savedSearchesIndex).
		Query(query).
		Size(filters.Size).
		Do(ctx)

	if err != nil {
		log.Errorln(logTag, ":", err.Error())
		return savedSearches, err
	}
	for _, v := range results.Hits.Hits {
		var savedSearch SavedSearchES
		var savedSearchMap map[string]interface{}
		err2 := json.Unmarshal(v.Source, &savedSearch)
		if err2 != nil {
			log.Errorln(logTag, ":", err2.Error())
			return savedSearches, err2
		}
		err3 := json.Unmarshal(v.Source, &savedSearchMap)
		if err3 != nil {
			log.Errorln(logTag, ":", err3.Error())
			return savedSearches, err3
		}
		customEvents := make(map[string]interface{})
		// parse custom events
		for k, v := range savedSearchMap {
			if strings.HasPrefix(k, CustomEventsPrefix) {
				customEvents[trimEventPrefix(k)] = v
			}
		}
		savedSearch.CustomEvents = &customEvents
		savedSearches = append(savedSearches, savedSearch)
	}
	return savedSearches, nil
}

func (es *elasticsearch) updateFavorite(ctx context.Context, record FavoriteRequest) *Error {
	analyticsRecord, err := es.getAnalyticsDocument(ctx, *record.QueryId)
	if err != nil || analyticsRecord == nil {
		return &Error{
			message: err,
			code:    http.StatusNotFound,
		}
	}
	var createdAt, updatedAt *int64
	t := time.Now()
	currentTime := t.Unix()
	timeStamp := t.Format(time.RFC3339)
	// check saved search existence
	favorite, _ := es.getFavorite(ctx, *record.Id)
	if favorite != nil {
		createdAt = favorite.CreatedAt
		updatedAt = &currentTime
	} else {
		createdAt = &currentTime
	}
	var source *string
	if record.Source != nil {
		marshalledSource, err := json.Marshal(*record.Source)
		if err != nil {
			return &Error{
				message: err,
			}
		}
		stringSource := string(marshalledSource)
		source = &stringSource
	}

	favoriteESRecord := FavoriteES{
		QueryId:                record.QueryId,
		FavoriteOn:             record.FavoriteOn,
		Source:                 source,
		Id:                     record.Id,
		UserId:                 record.UserId,
		CustomEvents:           record.CustomEvents,
		CreatedAt:              createdAt,
		UpdatedAt:              updatedAt,
		TimeStamp:              &timeStamp,
		SearchQuery:            analyticsRecord.SearchQuery,
		SearchQueryCharsLength: analyticsRecord.QueryLength,
		Meta:                   record.Meta,
	}
	// apply analytics
	if favoriteESRecord.UserId == nil {
		favoriteESRecord.UserId = &analyticsRecord.UserID
	}
	// apply custom events
	customEvents := make(map[string]interface{})
	for _, v := range analyticsRecord.CustomEvents {
		customEvents[CustomEventsPrefix+v.Key] = v.Value
	}
	if favoriteESRecord.CustomEvents != nil {
		for k, v := range *favoriteESRecord.CustomEvents {
			customEvents[CustomEventsPrefix+k] = v
		}
	}
	var favoriteMap map[string]interface{}
	marshalled, _ := json.Marshal(favoriteESRecord)
	json.Unmarshal(marshalled, &favoriteMap)
	delete(favoriteMap, "customEvents")
	// Add custom events as field
	for k, v := range customEvents {
		favoriteMap[k] = v
	}
	_, err2 := util.GetClient7().
		Index().
		Index(es.favoritesIndex).
		BodyJson(favoriteMap).
		Refresh("wait_for").
		Id(*favoriteESRecord.Id).
		Do(ctx)
	if err2 != nil {
		log.Errorln(logTag, ":", err2)
		return &Error{
			message: err2,
		}
	}
	return nil
}
func (es *elasticsearch) getFavorites(ctx context.Context, filters SavedSearchesFilters) ([]map[string]interface{}, error) {
	var favorites = make([]map[string]interface{}, 0)
	duration := escompat.NewRangeQuery("timestamp").
		Gte(filters.FromTimeStamp).
		Lte(filters.ToTimeStamp).
		TimeZone(filters.TimeZone)

	query := es7.NewBoolQuery().Filter(duration)

	if filters.MinChars != nil {
		minCharQuery := escompat.NewRangeQuery("search_characters_length").Gte(*filters.MinChars)
		query.Filter(minCharQuery)
	}

	applyCustomEventsEs7(query, filters.CustomEvents)

	results, err := util.
		GetClient7().
		Search(es.favoritesIndex).
		Query(query).
		Size(filters.Size).
		Do(ctx)

	if err != nil {
		log.Errorln(logTag, ":", err.Error())
		return favorites, err
	}
	for _, v := range results.Hits.Hits {
		var favorite map[string]interface{}
		err2 := json.Unmarshal(v.Source, &favorite)
		if err2 != nil {
			log.Errorln(logTag, ":", err2.Error())
			return favorites, err2
		}
		customEvents := make(map[string]interface{})
		// parse custom events
		for k, v := range favorite {
			if strings.HasPrefix(k, CustomEventsPrefix) {
				customEvents[trimEventPrefix(k)] = v
			}
		}
		favorite["custom_events"] = &customEvents
		// Convert source back to object
		if source, ok := favorite["source"]; ok {
			sourceAsString, isString := source.(string)
			if isString {
				var sourceMap map[string]interface{}
				err := json.Unmarshal([]byte(sourceAsString), &sourceMap)
				if err != nil {
					log.Errorln(logTag, ":", err)
					return favorites, err
				}
				// return source as an object
				favorite["source"] = sourceMap
			}
		}
		favorites = append(favorites, favorite)
	}
	return favorites, nil
}

// savePreferences will save the analytics preferences
func (es *elasticsearch) savePreferences(ctx context.Context, preferences AnalyticsPreferences) error {
	return es.savePreferencesEs7(ctx, preferences)
}

// getPreferences will get the analytics preferences
func (es *elasticsearch) getPreferences(ctx context.Context) (AnalyticsPreferences, error) {
	return es.getPreferencesEs7(ctx)
}

type recentDocumentsElasticsearch struct {
	index string
}

// createRecentSearchesIndex will create the recent searches index to
// store map of documents to users
func createRecentSearchesIndex(indexWithSuffix, indexConfig string) (*recentDocumentsElasticsearch, error) {
	// Generate the mappings based on the cluster type
	replicas := util.GetReplicas()

	es := &recentDocumentsElasticsearch{indexWithSuffix}

	// Check if the index already exists
	exists, err := util.GetClient7().IndexExists(indexWithSuffix).Do(context.Background())
	if err != nil {
		return nil, fmt.Errorf("error while checking if index already exists: %v", err)
	}
	if exists {
		log.Println(logTag, ": index named", indexWithSuffix, "already exists, skipping...")
		return es, nil
	}

	mappings := `{"properties": {"users": {"type": "%s"}}}`
	mappingForType := ""
	switch util.GetClusterType().String() {
	case util.ElasticSearch.String():
		mappingForType = "flattened"
	case util.OpenSearch.String():
		mappingForType = "flat_object"
	}

	mappings = fmt.Sprintf(mappings, mappingForType)

	settings := util.AdaptIndexBody(fmt.Sprintf(indexConfig, util.HiddenIndexSettings(), replicas, mappings))

	// index does not exists, create a new one
	_, err = util.GetClient7().CreateIndex(indexWithSuffix).Body(settings).Do(context.Background())
	if err == nil {
		return es, nil
	}

	// Use fallback method to create the index without the mappings
	fallbackSettings := util.AdaptIndexBody(fmt.Sprintf(indexConfig, util.HiddenIndexSettings(), replicas, "{}"))
	_, fallbackErr := util.GetClient7().CreateIndex(indexWithSuffix).Body(fallbackSettings).Do(context.Background())
	if fallbackErr != nil {
		return nil, fmt.Errorf("error while creating index named %s: %v", indexWithSuffix, err)
	}

	return es, nil
}
