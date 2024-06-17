package storedquery

import (
	"encoding/json"

	log "github.com/sirupsen/logrus"
)

// storedQueries represents the struct of a list of saved stored queries in the .storedquery index
var storedQueries []ESStoredQueryCached

// SetStoredQueriesToCache sets the queries
func SetStoredQueriesToCache(queries []ESStoredQueryDOC) error {
	tempQueries := []ESStoredQueryCached{}
	for _, query := range queries {
		queryInBytes := []byte(*query.Query)
		var queryAsMap interface{}
		err := json.Unmarshal(queryInBytes, &queryAsMap)
		if err != nil {
			log.Errorln(logTag, "error encountered while un-marshalling")
		}
		tempQueries = append(tempQueries, ESStoredQueryCached{
			ID:          query.ID,
			Index:       query.Index,
			Description: query.Description,
			Query:       &queryAsMap,
			CreatedAt:   query.CreatedAt,
			UpdatedAt:   query.UpdatedAt,
			// store query in bytes to avoid json marshal at query time
			QueryInBytes: queryInBytes,
			Params:       query.Params,
		})
	}
	storedQueries = tempQueries
	return nil
}

// GetStoredQueriesFromCache returns a list of cached stored queries
func GetStoredQueriesFromCache() []ESStoredQueryCached {
	return storedQueries
}

// AddStoredQueryToCache adds a query to cache
func AddStoredQueryToCache(query ESStoredQueryDOC) error {
	_, loc := IsStoredQueryExistsInCache(*query.ID)
	queryInBytes := []byte(*query.Query)
	var queryAsMap interface{}
	err := json.Unmarshal(queryInBytes, &queryAsMap)
	if err != nil {
		log.Errorln(logTag, "error encountered while un-marshalling")
	}
	if *query.ID != "" && loc != nil {
		// update the query
		storedQueries[*loc] = ESStoredQueryCached{
			ID:           query.ID,
			Index:        query.Index,
			Description:  query.Description,
			Query:        &queryAsMap,
			QueryInBytes: queryInBytes,
			Params:       query.Params,
			CreatedAt:    query.CreatedAt,
			UpdatedAt:    query.UpdatedAt,
		}
	} else {
		// append at the start
		storedQueries = append([]ESStoredQueryCached{{
			ID:           query.ID,
			Index:        query.Index,
			Description:  query.Description,
			Query:        &queryAsMap,
			QueryInBytes: queryInBytes,
			Params:       query.Params,
			CreatedAt:    query.CreatedAt,
			UpdatedAt:    query.UpdatedAt,
		}}, storedQueries...)
	}
	return nil
}

// IsStoredQueryExistsInCache checks if a query is present in cache
func IsStoredQueryExistsInCache(id string) (*ESStoredQueryCached, *int) {
	for loc, query := range storedQueries {
		if *query.ID == id {
			return &query, &loc
		}
	}
	return nil, nil
}

// DeleteStoredQueryToCache deletes a query from cache
func DeleteStoredQueryToCache(id string) bool {
	_, loc := IsStoredQueryExistsInCache(id)
	if loc != nil {
		storedQueries = append(storedQueries[:*loc], storedQueries[*loc+1:]...)
		return true
	}
	return false
}

// GetStoredQueryFromCache returns a stored query by ID
func GetStoredQueryFromCache(id string) *ESStoredQueryCached {
	query, _ := IsStoredQueryExistsInCache(id)
	return query
}
