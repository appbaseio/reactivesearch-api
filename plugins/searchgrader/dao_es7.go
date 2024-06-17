package searchgrader

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/appbaseio-confidential/reactivesearch/plugins/querytranslate"
	log "github.com/sirupsen/logrus"

	"github.com/appbaseio-confidential/reactivesearch/util"
	es7 "github.com/olivere/elastic/v7"
)

func (es *elasticsearch) updateGradeEs7(ctx context.Context, docID string, record ESDoc) error {
	_, err := util.GetClient7().
		Update().
		Index(es.indexName).
		DocAsUpsert(true).
		Doc(record).
		RetryOnConflict(5).
		Id(docID).
		Do(ctx)
	if err != nil {
		log.Errorln(logTag, ": error updating grades for id=", docID, ":", err)
		return err
	}
	return nil
}

func (es *elasticsearch) getMetricsEs7(ctx context.Context, record GradeMetricsRequest) (*GradeMetricsResponse, *int, error) {
	// Get search query terms for indices
	queryToGetTerms := es7.NewBoolQuery().Must(es7.NewTermsQuery("index.keyword", getInterfaceArray(record.Indices)...))
	result, err := util.GetClient7().
		Search().
		Size(0).
		Query(queryToGetTerms).
		Aggregation("query_term_agg", es7.NewTermsAggregation().Field("query.keyword").OrderByCountDesc().Size(10000)).
		Do(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to fetch query terms from es: %v", err)
	}
	aggResult, found := result.Aggregations.Terms("query_term_agg")
	if !found {
		return nil, nil, fmt.Errorf("unable to fetch aggregation value from 'query_term_agg'")
	}
	totalQueryTerms := len(aggResult.Buckets)
	queryTerms := []string{}

	page := 0
	if record.Page != nil {
		page = *record.Page - 1
	}
	pageSize := 10

	offset := page * pageSize

	if offset > totalQueryTerms {
		code := http.StatusNotFound
		return nil, &code, fmt.Errorf("page not found")
	}

	for i := offset; (i < offset+pageSize) && (i < len(aggResult.Buckets)); i++ {
		keyAsString, ok := aggResult.Buckets[i].Key.(string)
		if !ok {
			return nil, nil, fmt.Errorf("unable to type case bucket key as string")
		}
		queryTerms = append(queryTerms, keyAsString)
	}

	// documentIDs represents the unique documents IDs returned by RS API response
	documentIDs := make(map[string]interface{})

	// query to index map with documentIDs
	// For example, { "harry": { "index1": ["doc_id_1"], "index2": ["doc_id_2"]}}
	queryToIndexMapToDocIDs := make(map[string]map[string][]string)

	// Prepare the request body for RS API
	for _, index := range record.Indices {
		url := "http://" + getMasterCredentials() + "@localhost:" + strconv.Itoa(util.Port) + "/" + index + "/_reactivesearch"

		rsAPIQuery := []querytranslate.Query{}
		// Add a separate query for each query term
		for _, query := range queryTerms {
			id := index + idSeparator + query
			var queryValue interface{}
			queryValue = query
			// exclude all fields, we just need the document ID
			excludeFields := []string{"*"}
			rsAPIQuery = append(rsAPIQuery, querytranslate.Query{
				ID:            &id,
				Value:         &queryValue,
				ExcludeFields: &excludeFields,
			})
		}
		if len(rsAPIQuery) > 0 {
			requestBody := querytranslate.RSQuery{
				Query: rsAPIQuery,
			}
			marshalledRequest, err := json.Marshal(requestBody)
			if err != nil {
				log.Errorln(logTag, ": error encountered while marshalling request body", err)
				return nil, nil, err
			}
			// make the RS API request
			req, _ := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(marshalledRequest))
			req.Header.Add("Content-Type", "application/json")
			req.Header.Add("cache-control", "no-cache")
			res, err := util.HTTPClient().Do(req)
			if err != nil {
				log.Errorln(logTag, ":", err)
				return nil, nil, err
			}
			if res.StatusCode != 200 {
				return nil, &res.StatusCode, fmt.Errorf("error encountered while querying the `" + index + "` index, please make sure that `index` is active and search relevancy settings is applied")
			}
			var rsAPIResponse map[string]struct {
				Hits struct {
					Hits []struct {
						ID string `json:"_id"`
					} `json:"hits"`
				} `json:"hits"`
			}
			err2 := json.NewDecoder(res.Body).Decode(&rsAPIResponse)
			if err2 != nil {
				log.Errorln(logTag, ":", err2)
				return nil, nil, err2
			}
			for key, value := range rsAPIResponse {
				if !util.Contains(querytranslate.RESERVED_KEYS_IN_RESPONSE, key) {
					documentIDsAsArray := []string{}
					for _, hit := range value.Hits.Hits {
						documentID := hit.ID
						// Add the document ID to the map on unique documentIDs
						if documentIDs[documentID] == nil {
							documentIDs[documentID] = true
						}
						documentIDsAsArray = append(documentIDsAsArray, documentID)
					}
					// populate the queryToIndexMapToDocIDs with documentIDs for grade evaluation
					splitString := strings.Split(key, idSeparator)
					if len(splitString) > 1 {
						queryTerm := splitString[1]
						if queryToIndexMapToDocIDs[queryTerm] == nil {
							// If query term not present then add it
							queryToIndexMapToDocIDs[queryTerm] = map[string][]string{
								index: documentIDsAsArray,
							}
						} else {
							queryToIndexMapToDocIDs[queryTerm][index] = documentIDsAsArray
						}
					}
				}
			}
		}
	}

	uniqueDocumentIDs := []string{}
	for docID := range documentIDs {
		uniqueDocumentIDs = append(uniqueDocumentIDs, docID)
	}
	// Get the grade values for documentIDs
	gradeQueryRes, err := util.GetClient7().
		Search().
		Index(es.indexName).
		Query(es7.NewBoolQuery().Must(es7.NewTermsQuery("doc_id.keyword", getInterfaceArray(uniqueDocumentIDs)...))).
		Size(1000).
		Do(ctx)

	var docIDToGradeMap = make(map[string]int)

	for _, hit := range gradeQueryRes.Hits.Hits {
		var esDoc ESDoc
		err := json.Unmarshal(hit.Source, &esDoc)
		if err != nil {
			log.Errorln(logTag, ": error encountered while un-marshaling grade response :", err)
			return nil, nil, err
		}
		if esDoc.Grade != nil && esDoc.DocID != nil {
			docIDToGradeMap[*esDoc.DocID] = *esDoc.Grade
		}
	}

	// query to index map with grade values
	// For example, { "harry": { "index1": 20, "index2": 10}}
	queryToIndexMapToGrades := make(map[string]map[string]int)

	for queryTerm, indexMapToDocIDs := range queryToIndexMapToDocIDs {
		indexMapToGrades := make(map[string]int)
		for index, docIDs := range indexMapToDocIDs {
			finalGrade := 0
			for _, docID := range docIDs {
				grade, found := docIDToGradeMap[docID]
				if found {
					finalGrade += grade
				}
			}
			indexMapToGrades[index] = finalGrade
		}
		queryToIndexMapToGrades[queryTerm] = indexMapToGrades
	}

	return &GradeMetricsResponse{
		Total:   totalQueryTerms,
		Metrics: queryToIndexMapToGrades,
	}, nil, nil
}

func (es *elasticsearch) getDocumentsEs7(ctx context.Context, query string) (map[string]interface{}, error) {
	var gradedResults = map[string]interface{}{}
	res, err := util.GetClient7().
		Search().
		Index(es.indexName).
		Query(es7.NewBoolQuery().Must(es7.NewTermQuery("query.keyword", query))).
		Size(10000).
		Do(ctx)
	if err != nil {
		log.Errorln(logTag, ": error retrieving grades for query=", query, ":", err)
		return gradedResults, err
	}
	for _, v := range res.Hits.Hits {
		var esDoc ESDoc
		err := json.Unmarshal(v.Source, &esDoc)
		if err != nil {
			log.Errorln(logTag, ": error encountered while un-marshaling grade response", query, ":", err)
			return gradedResults, err
		}
		gradedResults[*esDoc.DocID] = esDoc.Grade
	}
	return gradedResults, nil
}
