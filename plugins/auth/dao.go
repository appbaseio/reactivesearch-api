package auth

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/appbaseio-confidential/arc/internal/types/user"
	"github.com/olivere/elastic"
)

type elasticsearch struct {
	url       string
	indexName string
	typeName  string
	client    *elastic.Client
}

func NewES(url, indexName, typeName string) (*elasticsearch, error) {
	opts := []elastic.ClientOptionFunc{
		elastic.SetURL(url),
		elastic.SetSniff(false),
	}

	// auth only need to make a connection to es,
	// users plugin handles creation of meta index
	client, err := elastic.NewClient(opts...)
	if err != nil {
		return nil, fmt.Errorf("%s: error while initializing elastic client: %v\n", logTag, err)
	}
	es := &elasticsearch{url, indexName, typeName, client}

	return es, nil
}

func (es *elasticsearch) getUser(userId string) (*user.User, error) {
	data, err := es.getRawUser(userId)
	if err != nil {
		return nil, err
	}
	var u user.User
	err = json.Unmarshal(data, &u)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (es *elasticsearch) getRawUser(userId string) ([]byte, error) {
	data, err := es.client.Get().
		Index(es.indexName).
		Type(es.typeName).
		Id(userId).
		FetchSource(true).
		Do(context.Background())
	if err != nil {
		return nil, err
	}
	src, err := data.Source.MarshalJSON()
	if err != nil {
		return nil, err
	}

	return src, nil
}
