package permissions

import (
	"context"
	"encoding/json"

	"github.com/appbaseio/arc/util"
	es7 "github.com/olivere/elastic/v7"
)

func (es *elasticsearch) checkRoleExistsEs7(ctx context.Context, role string) (bool, error) {
	resp, err := util.GetClient7().Search().
		Index(es.indexName).
		Query(es7.NewTermQuery("role", role)).
		Do(ctx)
	if err != nil {
		return false, err
	}

	return resp.Hits.TotalHits.Value > 0, nil
}

func (es *elasticsearch) getRawRolePermissionEs7(ctx context.Context, role string) ([]byte, error) {
	resp, err := util.GetClient7().Search().
		Index(es.indexName).
		Query(es7.NewTermQuery("role", role)).
		Size(1).
		FetchSource(true).
		Do(ctx)
	if err != nil {
		return nil, err
	}
	for _, hit := range resp.Hits.Hits {
		src, err := json.Marshal(hit.Source)
		if err == nil {
			return src, nil
		}
	}
	return nil, nil
}
