package pipelines

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"sync"
	"time"

	"github.com/appbaseio-confidential/reactivesearch/model/category"
	"github.com/appbaseio-confidential/reactivesearch/plugins/applycache"
	"github.com/appbaseio-confidential/reactivesearch/plugins/querytranslate"
	"github.com/appbaseio-confidential/reactivesearch/plugins/rules"
	"github.com/appbaseio-confidential/reactivesearch/plugins/suggestions"
	log "github.com/sirupsen/logrus"
)

// constructURL constructs the URL from the request URL and the URL values
// used for creating the URL key for caching and retrieving cache value
func constructURL(reqURL string, urlValues map[string]interface{}) string {
	// Extract keys from the map and sort them
	var keys []string
	for k := range urlValues {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// Construct the query string using the sorted keys
	values := url.Values{}
	for _, k := range keys {
		values.Add(k, fmt.Sprintf("%v", urlValues[k]))
	}
	queryStr := values.Encode()

	return fmt.Sprintf("%s?%s", reqURL, queryStr)
}

func executeUseCacheStage(
	cachedRequestContext *CachedRequestContext,
	scriptContextInBytes []byte,
	scriptEnvs map[string]interface{},
	startTime *time.Time,
	rsAPIRequest *ReactiveSearchQueryContext,
) ([]byte, bool, *Error) {
	{
		var reqURL string
		// Extract path from envs
		path, ok := scriptEnvs["path"].(string)
		if ok {
			reqURL = path
		}
		useCache := true
		// react cache query param
		paramsAsMap, ok := scriptEnvs["urlValues"].(map[string]interface{})
		uri := constructURL(reqURL, paramsAsMap)
		if ok {
			for k, v := range paramsAsMap {
				paramValue, ok := v.(bool)
				if ok && k == "cache" {
					useCache = paramValue
				}
			}
		}
		// apply headers from script context
		var scriptContext rules.ScriptContext
		err := json.Unmarshal(scriptContextInBytes, &scriptContext)
		if err != nil {
			log.Errorln(logTag, ":", err)
			return nil, false, &Error{
				Err: err,
			}
		}
		var rsAPIBody *querytranslate.RSQuery
		reqCategory, ok := scriptEnvs["category"].(string)
		if ok && reqCategory == category.ReactiveSearch.String() {
			// read useCache for RS API
			var rsAPIRequest querytranslate.RSQuery
			err := json.Unmarshal([]byte(scriptContext.Request.Body), &rsAPIRequest)
			if err == nil {
				if rsAPIRequest.Settings != nil && rsAPIRequest.Settings.UseCache != nil {
					useCache = *rsAPIRequest.Settings.UseCache
				}
				rsAPIBody = &rsAPIRequest
			}
		}

		if useCache {
			// Update request body to be cached
			cachedRequestContext.Put([]byte(scriptContext.Request.Body))

			index, indexPresent := scriptEnvs["index"]
			if !indexPresent {
				errMsg := "`index` should be present as an env for zinc stage"
				log.Warnln(logTag, ": ", errMsg)
				return nil, false, &Error{
					Err:  fmt.Errorf(errMsg),
					Code: http.StatusBadRequest,
				}
			}

			indices := make([]string, 0)

			// `index` can be both a string and an array, so we need to
			// support both.
			indexAsArr, asArrOk := index.([]interface{})
			if !asArrOk {
				// Parse as a string else throw error
				indexStr, asStrOk := index.(string)
				if !asStrOk {
					errMsg := "`index` is neither array nor string, should be one of the two"
					log.Warnln(logTag, ": ", errMsg)
					return nil, false, &Error{
						Err:  fmt.Errorf(errMsg),
						Code: http.StatusBadRequest,
					}
				}

				indices = append(indices, indexStr)
			} else {
				// Throw error if array is empty
				if len(indexAsArr) < 1 {
					errMsg := "`index` array should contain at-least 1 element"
					log.Warnln(logTag, ": ", errMsg)
					return nil, false, &Error{
						Err:  fmt.Errorf(errMsg),
						Code: http.StatusBadRequest,
					}
				}

				// Parse the first element of `index` to string and use as
				// the index.
				for _, index := range indexAsArr {
					indexStr, asStrOk := index.(string)
					if !asStrOk {
						errMsg := "element at 0th index for `index` is non-string value"
						log.Warnln(logTag, ": ", errMsg)
						return nil, false, &Error{
							Err:  fmt.Errorf(errMsg),
							Code: http.StatusBadRequest,
						}
					}

					indices = append(indices, indexStr)
				}
			}

			if len(indices) == 0 {
				errMsg := "couldn't parse value of `index`"
				log.Warnln(logTag, ": ", errMsg)
				return nil, false, &Error{
					Err:  fmt.Errorf(errMsg),
					Code: http.StatusBadRequest,
				}
			}

			// apply cache
			cachedResponse, err := applycache.ApplyCache(uri, rsAPIBody, cachedRequestContext.Get(), startTime, "", indices)
			if err != nil {
				return nil, false, &Error{
					Err: err,
				}
			}
			if cachedResponse != nil {
				cachedResponseBody := cachedResponse.Body
				updatedRSBody := rsAPIRequest.Get()
				if updatedRSBody == nil {
					updatedRSBody = rsAPIBody
				}
				if updatedRSBody != nil {
					var recentSuggestionsMap = make(map[string]suggestions.SuggestionOutput)

					var recentSuggestionsWg sync.WaitGroup
					recentSuggestionsOut := make(chan suggestions.SuggestionOutput)
					// Fetch recent suggestions for `suggestion` type of queries
					for _, query := range updatedRSBody.Query {
						if query.Type == querytranslate.Suggestion {
							// fetch recent suggestions
							if query.EnableRecentSuggestions != nil &&
								*query.EnableRecentSuggestions {
								recentSuggestionsWg.Add(1)
								go func(out chan<- suggestions.SuggestionOutput) {
									defer recentSuggestionsWg.Done()
									config := querytranslate.RecentSuggestionsOptions{}
									if query.RecentSuggestionsConfig != nil {
										config = *query.RecentSuggestionsConfig
									}
									indices := getIndicesFromEnvs(scriptEnvs)
									recentSuggestions, err := suggestions.GetRecentSuggestions(query, config, indices)
									if err != nil {
										log.Errorln(logTag, ":", err)
										out <- suggestions.SuggestionOutput{
											QueryID: *query.ID,
											Error:   err,
										}
									} else {
										out <- suggestions.SuggestionOutput{
											QueryID:     *query.ID,
											Suggestions: recentSuggestions,
											Error:       nil,
										}
									}
								}(recentSuggestionsOut)
							} else {
								recentSuggestionsMap[*query.ID] = suggestions.SuggestionOutput{
									QueryID:     *query.ID,
									Suggestions: make([]querytranslate.SuggestionHIT, 0),
									Error:       nil,
								}
							}
						}
					}
					// wait for recent suggestions
					go func() {
						recentSuggestionsWg.Wait()
						close(recentSuggestionsOut)
					}()
					// popular suggestions to query ID map
					for result := range recentSuggestionsOut {
						recentSuggestionsMap[result.QueryID] = result
					}
					// apply suggestions
					responseWithSuggestions, err2 := suggestions.ApplySuggestions(*rsAPIBody, cachedResponseBody, recentSuggestionsMap, nil, nil, nil, nil)
					if err2 != nil {
						log.Errorln(logTag, ":", err2.Error)
						return nil, false, &Error{
							Err:  err2.Error,
							Code: err2.Code,
						}
					}
					cachedResponseBody = responseWithSuggestions
				}
				for k, v := range cachedResponse.Headers {
					scriptContext.Response.Headers[k] = v
				}
				scriptContext.Response.Body = string(cachedResponseBody)
				scriptContext.Response.Code = http.StatusOK
				contextInBytes, err := json.Marshal(scriptContext)
				if err != nil {
					log.Errorln(logTag, ":", err)
					return nil, false, &Error{
						Err: err,
					}
				}
				// stop execution and return response
				return contextInBytes, true, nil
			}
		}
	}
	return scriptContextInBytes, false, nil
}
