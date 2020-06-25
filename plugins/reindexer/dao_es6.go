package reindexer

import (
	"context"

	"github.com/appbaseio/arc/util"
	log "github.com/sirupsen/logrus"
	es6 "gopkg.in/olivere/elastic.v6"
)

func updateSynonymsEs6(ctx context.Context, script string, index string, params map[string]interface{}) error {
	query := es6.NewTermQuery("index.keyword", index)
	_, err := util.GetClient6().
		UpdateByQuery().
		Type(typeName).
		Query(query).
		Index(getSynonymsIndex()).
		Script(es6.NewScript(script).Params(params)).
		Do(ctx)
	if err != nil {
		log.Errorln(logTag, ": error updating synonyms for index=", index, ":", err)
		return err
	}
	return nil
}
