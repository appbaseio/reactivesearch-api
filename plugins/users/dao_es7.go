package users

import (
	"context"
	"encoding/json"

	"github.com/appbaseio/arc/util"
)

func (es *elasticsearch) getRawUsersEs7(ctx context.Context) ([]byte, error) {
	response, err := util.GetClient7().Search().
		Index(es.indexName).
		Do(ctx)

	if err != nil {
		return nil, err
	}

	var users []json.RawMessage
	for _, hit := range response.Hits.Hits {
		users = append(users, hit.Source)
	}

	return json.Marshal(users)
}
