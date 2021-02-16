package reindex

import (
	"context"

	"github.com/appbaseio/arc/util"
	es7 "github.com/olivere/elastic/v7"
	log "github.com/sirupsen/logrus"
)

func updateSynonymsEs7(ctx context.Context, script string, index string, params map[string]interface{}) error {
	query := es7.NewTermQuery("index.keyword", index)
	_, err := util.GetClient7().
		UpdateByQuery().
		Query(query).
		Index(getSynonymsIndex()).
		Script(es7.NewScript(script).Params(params)).
		Do(ctx)
	if err != nil {
		log.Errorln(logTag, ": error updating synonyms for index=", index, ":", err)
		return err
	}
	return nil
}
