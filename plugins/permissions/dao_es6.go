package permissions

import (
	"context"
	"encoding/json"

	"github.com/appbaseio/arc/util"
	es6 "gopkg.in/olivere/elastic.v6"
)

func (es *elasticsearch) CheckRoleExistsEs6(ctx context.Context, role string) (bool, error) {
	resp, err := util.GetClient6().Search().
		Index(es.indexName).
		Query(es6.NewTermQuery("role", role)).
		Do(ctx)
	if err != nil {
		return false, err
	}

	return resp.Hits.TotalHits > 0, nil
}

func (es *elasticsearch) GetRawRolePermissionEs6(ctx context.Context, role string) ([]byte, error) {
	resp, err := util.GetClient6().Search().
		Index(es.indexName).
		Query(es6.NewTermQuery("role", role)).
		Size(1).
		FetchSource(true).
		Do(ctx)
	if err != nil {
		return nil, err
	}
	for _, hit := range resp.Hits.Hits {
		src, err := json.Marshal(*hit.Source)
		if err == nil {
			return src, nil
		}
	}
	return nil, nil
}
