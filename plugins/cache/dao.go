package cache

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/appbaseio-confidential/reactivesearch/util"
	log "github.com/sirupsen/logrus"
)

type elasticsearch struct {
	indexName string
}

func initPlugin(cacheIndex, mapping string) (*elasticsearch, error) {
	es := &elasticsearch{cacheIndex}

	ctx := context.Background()

	// Check if the rules index already exists
	exists, err := util.GetClient7().IndexExists(cacheIndex).Do(ctx)
	if err != nil {
		return es, fmt.Errorf("error while checking if index already exists: %v", err)
	}
	if exists {
		log.Printf("%s: index named '%s' already exists, skipping...", logTag, cacheIndex)
		return es, nil
	}

	replicas := util.GetReplicas()
	settings := fmt.Sprintf(mapping, util.HiddenIndexSettings(), replicas)

	// Meta index does not exists, create a new one
	_, err = util.GetClient7().CreateIndex(cacheIndex).Body(settings).Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("error while creating index named %s: %v", cacheIndex, err)
	}

	log.Printf("%s successfully created index named '%s'", logTag, cacheIndex)
	return es, nil
}

func (es *elasticsearch) savePreferences(ctx context.Context, cacheConfig CacheConfig) error {
	return es.savePreferencesEs7(ctx, cacheConfig)
}

// Get saved preferences
func (es *elasticsearch) getPreferences(ctx context.Context) (CacheConfig, error) {
	var record = CacheConfig{}
	response, err := util.GetClient7().Get().
		Index(es.indexName).
		Id(cacheConfigDocID).
		Do(ctx)
	if err != nil {
		log.Warnln(logTag, ": preferences not found", err)
		return record, err
	}
	err = json.Unmarshal(response.Source, &record)
	if err != nil {
		log.Errorln(logTag, ": error retrieving cache preferences", err)
		return record, err
	}
	return record, nil
}

func (es *elasticsearch) populateDefaultConfig() (*CacheConfig, error) {
	cacheConfig, err := es.getPreferences(context.Background())
	if err != nil {
		log.Warnln(logTag, ": default preferences not found. Updating default preferences", err)
		// 5 minutes, we store it in seconds
		defaultCacheDuration := int64(5 * 60)
		defaultSize := int64(128)
		defaultIndices := []string{"*"}
		enabledCache := false
		defaultCacheConfig := CacheConfig{
			EnableCache: &enabledCache,
			MaxSize:     &defaultSize,
			MaxDuration: &defaultCacheDuration,
			Indices:     &defaultIndices,
		}
		err := es.savePreferences(context.Background(), defaultCacheConfig)
		if err != nil {
			log.Errorln(logTag, ": error storing default cache preferences", err)
			return nil, err
		}
		return &defaultCacheConfig, nil
	}
	return &cacheConfig, nil
}

// Clear the cached items
func (es *elasticsearch) clearCache() {
	searchCache.Clear()
	if redisSearchCache != nil {
		redisSearchCache.FlushDB(ctx)
	}
}
