package cache

import (
	"context"

	"github.com/appbaseio-confidential/reactivesearch/util"
	log "github.com/sirupsen/logrus"
)

func (es *elasticsearch) savePreferencesEs7(ctx context.Context, cacheConfig CacheConfig) error {
	_, err := util.GetClient7().
		Update().
		Index(es.indexName).
		Upsert(cacheConfig).
		DocAsUpsert(true).
		Doc(cacheConfig).
		RetryOnConflict(5).
		Id(cacheConfigDocID).
		Do(ctx)
	if err != nil {
		log.Errorln(logTag, ": error updating cache config :", err)
		return err
	}
	return nil
}
