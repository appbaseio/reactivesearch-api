package elasticsearch

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/appbaseio/reactivesearch-api/util"
	"github.com/robfig/cron"
	log "github.com/sirupsen/logrus"
)

// CacheIndexesForTenants will fetch all the indexes from the system
// ES and then filter them into different tenants and accordingly
// cache them into the cache map
//
// This function will only execute if SLS is enabled and Multi-Tenant
// is enabled
func (es *elasticsearch) CacheIndexesForTenants() error {
	if util.IsSLSDisabled() || !util.MultiTenant {
		return nil
	}

	// Make a _cat/indices call to get all the indexes for the tenant
	indices, indicesFetchErr := es.systemESClient.CatIndices().Do(context.Background())
	if indicesFetchErr != nil {
		return indicesFetchErr
	}

	for _, index := range indices {
		// Use the name of the index to extract the tenant_id and then
		// cache it accordingly.
		strippedIndexName, tenantId := util.RemoveTenantID(index.Index)

		// Not likely, but there can be indexes that do not have the
		// tenantId appended to the name of the index. In such a case,
		// we can skip these indexes
		if tenantId == "" {
			continue
		}

		SetIndexToCache(tenantId, strippedIndexName)
	}

	return nil
}

// InitCacheIndexes cache the tenant to index map and
// will run a cronjob to update the cache of tenant to index map
func (es *elasticsearch) InitCacheIndexes() error {
	firstCacheErr := es.CacheIndexesForTenants()
	if firstCacheErr != nil {
		return firstCacheErr
	}

	syncCronJob := cron.New()
	syncCronJob.AddFunc("@every 60s", func() {
		err := es.CacheIndexesForTenants()
		if err != nil {
			log.Warnln(": error while syncing tenant to index cache! ", err.Error())
		}
	})
	syncCronJob.Start()

	return nil
}

// UpdateNDJsonRequestBody will update the nd-json body with the passed indices
// so that all possible known indices have the tenant_id appended to the
// name of the index.
func UpdateNDJsonRequestBody(body string, indices []string, tenantID string, isBulk bool) string {
	indexKey := `"index"`
	if isBulk {
		indexKey = `"_index"`
	}

	patternsToReplace := []string{
		indexKey + `: "%s"`,
		indexKey + `:"%s"`,
	}

	for _, cachedIndex := range indices {
		for _, pattern := range patternsToReplace {
			body = strings.Replace(string(body), fmt.Sprintf(pattern, cachedIndex), fmt.Sprintf(pattern, util.AppendTenantID(cachedIndex, tenantID)), -1)
		}
	}

	return body
}

// IsIndexLimitExceeded will check if the users index limit has exceeded
// based on the passed index.
//
// The logic here is that if the passed index name exists in the cached
// indices (which will be truncated based on size) then it can go ahead.
//
// However, if it is an unrecognized index and the number of new addable
// indexes are 0 then limit exceeded will be considered.
func IsIndexLimitExceeded(domain string, index string) bool {
	// Fetch the indexes for the tenant
	cachedIndexes := GetCachedIndices(domain)
	for _, indexName := range cachedIndexes {
		if indexName == index {
			return false
		}
	}

	// Check if the length of indexes exceeds the allowed length
	instanceDetails := util.GetSLSInstanceByDomain(domain)
	return instanceDetails.Tier.LimitForPlan().Indexes.IsLimitExceeded(len(cachedIndexes))
}

// VALID_INDEX_CREATE_ROUTES will contain the list of routes that can
// create an index
var VALID_INDEX_CREATE_ROUTES = map[string][]string{
	"/{index}": {
		http.MethodPut,
	},
	"/{index}/_doc/{id}": {
		http.MethodPost, http.MethodPut,
	},
	"/{index}/_doc/{id}/_update": {
		http.MethodPost,
	},
	"/{index}/_bulk": {
		http.MethodPut, http.MethodPost,
	},
}

// IsIndexRoute will check if the passed path is an index path
func (wh *WhitelistedRoute) IsIndexRoute(req *http.Request) bool {
	methods, exists := VALID_INDEX_CREATE_ROUTES[wh.Path]
	if !exists {
		return false
	}

	methodUsed := req.Method

	for _, method := range methods {
		if methodUsed == method {
			return true
		}
	}

	return false
}
