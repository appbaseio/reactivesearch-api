package querytranslate

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"time"

	"github.com/appbaseio/reactivesearch-api/middleware/classify"
	"github.com/appbaseio/reactivesearch-api/model/index"
	"github.com/appbaseio/reactivesearch-api/util"
	"github.com/buger/jsonparser"
	"github.com/gorilla/mux"
	es7 "github.com/olivere/elastic/v7"
	log "github.com/sirupsen/logrus"
)

func (r *QueryTranslate) search() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		ctx := req.Context()
		vars := mux.Vars(req)
		reqBody, err := ioutil.ReadAll(req.Body)
		if err != nil {
			log.Errorln(logTag, ":", err)
			util.WriteBackError(w, "Can't read request body", http.StatusBadRequest)
			return
		}
		defer req.Body.Close()
		log.Println("REQUEST BODY", reqBody)
		rsAPIRequest, err := FromContext(req.Context())
		if err != nil {
			msg := "error occurred while retrieving request body from context"
			log.Errorln(logTag, ":", err)
			util.WriteBackError(w, msg, http.StatusInternalServerError)
			return
		}
		var esResponseBody []byte
		responseStatusCode := http.StatusOK
		if len(reqBody) != 0 {
			reqURL := "/" + vars["index"] + "/_msearch"
			start := time.Now()
			httpRes, err := makeESRequest(ctx, reqURL, http.MethodPost, reqBody)
			if err != nil {
				msg := err.Error()
				log.Errorln(logTag, ":", err)
				// Response can be nil sometimes
				if httpRes != nil {
					util.WriteBackError(w, msg, httpRes.StatusCode)
					return
				}
				util.WriteBackError(w, msg, http.StatusInternalServerError)
				return
			}
			log.Println("TIME TAKEN BY ES:", time.Since(start))
			if httpRes.StatusCode > 500 {
				msg := "unable to connect to the upstream Elasticsearch cluster"
				log.Errorln(logTag, ":", msg)
				util.WriteBackError(w, msg, httpRes.StatusCode)
				return
			}
			esResponseBody = httpRes.Body
			responseStatusCode = httpRes.StatusCode
		} else {
			// mock ES response for empty requests
			// for example, suggestion type of requests can disable index suggestions
			// endpoint should retrun mocked response so middlewares can work to apply featured suggestions
			emptyResponses := make([]map[string]interface{}, 0)
			queryIds := GetQueryIds(*rsAPIRequest)
			for range queryIds {
				emptyResponses = append(emptyResponses, map[string]interface{}{
					"took": 0,
					"hits": map[string]interface{}{
						"hits": make([]interface{}, 0),
					},
				})
			}
			// mock es response
			marshalledRes, _ := json.Marshal(map[string]interface{}{
				"responses": emptyResponses,
			})
			esResponseBody = marshalledRes
		}

		rsResponse, err := TransformESResponse(esResponseBody, rsAPIRequest)
		if err != nil {
			util.WriteBackError(w, err.Error(), http.StatusInternalServerError)
			return
		}

		indices, err := index.FromContext(req.Context())
		if err != nil {
			msg := "error getting the index names from context"
			log.Errorln(logTag, ":", err)
			util.WriteBackError(w, msg, http.StatusInternalServerError)
			return
		}
		// Replace indices to alias
		for _, index := range indices {
			alias := classify.GetIndexAlias(index)
			if alias != "" {
				rsResponse = bytes.Replace(rsResponse, []byte(`"`+index+`"`), []byte(`"`+alias+`"`), -1)
				continue
			}
			// if alias is present in url get index name from cache
			indexName := classify.GetAliasIndex(index)
			if indexName != "" {
				rsResponse = bytes.Replace(rsResponse, []byte(`"`+indexName+`"`), []byte(`"`+index+`"`), -1)
			}
		}
		// if status code is not 200 write rsResponse otherwise return raw response from ES
		// avoid copy for performance reasons
		if responseStatusCode == http.StatusOK {
			util.WriteBackRaw(w, rsResponse, responseStatusCode)
		} else {
			util.WriteBackRaw(w, esResponseBody, responseStatusCode)
		}
	}
}

func (r *QueryTranslate) validate() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		reqBody, err := ioutil.ReadAll(req.Body)
		if err != nil {
			log.Errorln(logTag, ":", err)
			util.WriteBackError(w, "Can't read request body", http.StatusBadRequest)
			return
		}
		w.Header().Add("Content-Type", "application/x-ndjson")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(string(reqBody)))
	}
}

func TransformESResponse(response []byte, rsAPIRequest *RSQuery) ([]byte, error) {
	queryIds := GetQueryIds(*rsAPIRequest)

	rsResponse := []byte(`{}`)

	mockedRSResponse, _ := json.Marshal(map[string]interface{}{
		"took": 0,
		"hits": map[string]interface{}{
			"hits": make([]interface{}, 0),
		},
	})
	for _, query := range rsAPIRequest.Query {
		if query.Type == Suggestion &&
			query.EnableIndexSuggestions != nil &&
			*query.EnableIndexSuggestions {
			// mock empty response for suggestions
			rsResponseMocked, err := jsonparser.Set(rsResponse, []byte(mockedRSResponse), *query.ID)
			response = rsResponseMocked
			if err != nil {
				log.Errorln(logTag, ":", err)
				return nil, errors.New("error updating response :" + err.Error())
			}
		}
	}

	took, valueType1, _, err := jsonparser.Get(response, "took")
	// ignore not exist error
	if err != nil && valueType1 != jsonparser.NotExist {
		log.Errorln(logTag, ":", err)
		return nil, errors.New("can't parse took key from response")
	}
	// Set the `settings` key to response
	rsResponseWithTook, err := jsonparser.Set(rsResponse, []byte(fmt.Sprintf(`{ "took": %s }`, string(took))), "settings")
	if err != nil {
		log.Errorln(logTag, ":", err)
		return nil, errors.New("can't add settings key to response")
	}
	// Assign updated json to actual response
	rsResponse = rsResponseWithTook

	responseError, valueType2, _, err := jsonparser.Get(response, "error")
	// ignore not exist error
	if err != nil && valueType2 != jsonparser.NotExist {
		log.Errorln(logTag, ":", err)
		return nil, errors.New("can't parse error key from response")
	} else if responseError != nil {
		// Set the `error` key to response
		rsResponseWithError, err := jsonparser.Set(rsResponse, responseError, "error")
		if err != nil {
			log.Errorln(logTag, ":", err)
			return nil, errors.New("can't add error key to response")
		}
		// Assign updated json to actual response
		rsResponse = rsResponseWithError
	}

	// Read `responses` value from the response body
	responses, valueType3, _, err4 := jsonparser.Get(response, "responses")
	// ignore not exist error
	if err4 != nil && valueType3 != jsonparser.NotExist {
		log.Errorln(logTag, ":", err4)
		return nil, errors.New("can't parse responses key from response")
	}

	if responses != nil {
		index := 0
		var parsingError error
		// Set `responses` by query ID
		jsonparser.ArrayEach(responses, func(value []byte, dataType jsonparser.ValueType, offset int, err error) {
			if index < len(queryIds) {
				queryID := queryIds[index]
				var isSuggestionRequest bool
				var suggestions = make([]SuggestionHIT, 0)
				// parse suggestions if query is of type `suggestion`
				for _, query := range rsAPIRequest.Query {
					if *query.ID == queryID && query.Type == Suggestion {
						isSuggestionRequest = true
						// Index suggestions are not meant for empty query
						if query.Value != nil {
							valueAsString, ok := (*query.Value).(string)
							if ok {
								var normalizedDataFields = []string{}
								normalizedFields := NormalizedDataFields(query.DataField, query.FieldWeights)
								for _, dataField := range normalizedFields {
									normalizedDataFields = append(normalizedDataFields, dataField.Field)
								}
								suggestionsConfig := SuggestionsConfig{
									// Fields to extract suggestions
									DataFields: normalizedDataFields,
									// Query value
									Value:                       valueAsString,
									ShowDistinctSuggestions:     query.ShowDistinctSuggestions,
									EnablePredictiveSuggestions: query.EnablePredictiveSuggestions,
									MaxPredictedWords:           query.MaxPredictedWords,
									EnableSynonyms:              query.EnableSynonyms,
									ApplyStopwords:              query.ApplyStopwords,
									Stopwords:                   query.Stopwords,
									URLField:                    query.URLField,
									CategoryField:               query.CategoryField,
									HighlightField:              query.HighlightField,
									HighlightConfig:             query.HighlightConfig,
									Language:                    query.SearchLanguage,
								}

								var rawHits []ESDoc
								hits, dataType, _, err1 := jsonparser.Get(value, "hits", "hits")
								if dataType == jsonparser.NotExist {
									// write raw response
									rsResponseWithSearchResponse, err := jsonparser.Set(rsResponse, value, queryID)
									if err != nil {
										log.Errorln(logTag, ":", err)
										parsingError = errors.New("can't add search response to final response")
										return
									}
									rsResponse = rsResponseWithSearchResponse
									continue
								}
								if err1 != nil {
									log.Errorln(logTag, ":", err1)
									parsingError = errors.New("error while retriving hits: " + err1.Error())
									return

								}
								err := json.Unmarshal(hits, &rawHits)
								if err != nil {
									log.Errorln(logTag, ":", err)
									parsingError = errors.New("error while parsing ES response to hits: " + err.Error())
									return
								}
								// extract category suggestions
								if query.CategoryField != nil && *query.CategoryField != "" {
									categories, dataType2, _, err2 := jsonparser.Get(value, "aggregations", *query.CategoryField, "buckets")
									if err2 != nil {
										log.Errorln(logTag, ":", err2)
										parsingError = errors.New("error while retriving aggregations: " + err2.Error())
										return
									}
									if dataType2 != jsonparser.NotExist {
										var buckets []es7.AggregationBucketKeyItem
										err := json.Unmarshal(categories, &buckets)
										if err != nil {
											log.Errorln(logTag, ":", err)
											parsingError = errors.New("error while parsing ES aggregations to suggestions: " + err.Error())
											return
										}
										for _, v := range buckets {
											key, ok := v.Key.(string)
											if ok {
												count := int(v.DocCount)
												suggestions = append(suggestions, SuggestionHIT{
													Label:    valueAsString + " in " + key,
													Value:    valueAsString,
													Count:    &count,
													Category: &key,
												})
											}
										}
									}
								}

								// extract index suggestions
								suggestions = append(suggestions, getIndexSuggestions(suggestionsConfig, rawHits)...)
								if query.Size != nil {
									// fit suggestions to the max requested size
									if len(suggestions) > *query.Size {
										suggestions = suggestions[:*query.Size]
									}
								}
							}
						}
					}
				}
				if isSuggestionRequest {
					responseInByte, err := json.Marshal(suggestions)
					if err != nil {
						log.Errorln(logTag, ":", err)
						parsingError = errors.New("error while parsing suggestions:" + err.Error())
						return
					}
					rsResponseWithSuggestions, err := jsonparser.Set(value, responseInByte, "hits", "hits")
					if err != nil {
						log.Errorln(logTag, ":", err)
						parsingError = errors.New("can't add suggestions to final response" + err.Error())
						return
					}
					rsResponseWithSearchResponse, err := jsonparser.Set(rsResponse, rsResponseWithSuggestions, queryID)
					if err != nil {
						log.Errorln(logTag, ":", err)
						parsingError = errors.New("can't add search response to final response" + err.Error())
						return
					}
					// Modify total suggestions value
					rsResponseWithSearchResponse, err2 := jsonparser.Set(rsResponseWithSearchResponse, []byte(strconv.Itoa(len(suggestions))), queryID, "hits", "total", "value")
					if err2 != nil {
						log.Errorln(logTag, ":", err2)
						parsingError = errors.New("can't apply total value for hits" + err2.Error())
						return
					}
					rsResponse = rsResponseWithSearchResponse
				} else {
					rsResponseWithSearchResponse, err := jsonparser.Set(rsResponse, value, queryID)
					if err != nil {
						log.Errorln(logTag, ":", err)
						parsingError = errors.New("can't add search response to final response" + err.Error())
						return
					}
					rsResponse = rsResponseWithSearchResponse
				}
				index++
			}
		})
		if parsingError != nil {
			return nil, parsingError
		}
	}
	return rsResponse, nil
}
