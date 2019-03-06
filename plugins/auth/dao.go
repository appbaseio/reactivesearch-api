package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/appbaseio-confidential/arc/model/permission"
	"github.com/appbaseio-confidential/arc/model/user"
	"github.com/appbaseio-confidential/arc/util"
	"github.com/olivere/elastic"
)

type elasticsearch struct {
	url                             string
	userIndex, userType             string
	permissionIndex, permissionType string
	client                          *elastic.Client
}

func newClient(url, userIndex, permissionIndex string) (*elasticsearch, error) {
	// auth only has to establish a connection to es, users, permissions
	// plugin handles the creation of their respective meta indices
	client, err := elastic.NewClient(
		elastic.SetURL(url),
		elastic.SetRetrier(util.NewRetrier()),
		elastic.SetSniff(false),
	)
	if err != nil {
		return nil, fmt.Errorf("%s: error while initializing elastic client: %v", logTag, err)
	}

	es := &elasticsearch{
		url,
		userIndex, "_doc",
		permissionIndex, "_doc",
		client,
	}

	return es, nil
}

func (es *elasticsearch) getCredential(ctx context.Context, username, password string) (interface{}, error) {
	matchUsername := elastic.NewTermQuery("username.keyword", username)
	matchPassword := elastic.NewTermQuery("password.keyword", password)

	query := elastic.NewBoolQuery().
		Must(matchUsername, matchPassword)

	response, err := es.client.Search().
		Index(es.userIndex, es.permissionIndex).
		Query(query).
		FetchSource(true).
		Do(ctx)
	if err != nil {
		return nil, err
	}

	if len(response.Hits.Hits) > 1 {
		return nil, fmt.Errorf(`more than one result for "username"="%s" and "password"="%s"`, username, password)
	}

	// there should be either 0 or 1 hit
	var obj interface{}
	for _, hit := range response.Hits.Hits {
		if hit.Index == es.userIndex {
			var u user.User
			err := json.Unmarshal(*hit.Source, &u)
			if err != nil {
				return nil, err
			}
			obj = &u
		} else if hit.Index == es.permissionIndex {
			var p permission.Permission
			err := json.Unmarshal(*hit.Source, &p)
			if err != nil {
				return nil, err
			}
			obj = &p
		}
	}

	return obj, nil
}

func (es *elasticsearch) putUser(ctx context.Context, u user.User) (bool, error) {
	_, err := es.client.Index().
		Index(es.userIndex).
		Type(es.userType).
		Id(u.Username).
		BodyJson(u).
		Do(ctx)
	if err != nil {
		return false, err
	}

	return true, nil
}

func (es *elasticsearch) getUser(ctx context.Context, username string) (*user.User, error) {
	data, err := es.getRawUser(ctx, username)
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

func (es *elasticsearch) getRawUser(ctx context.Context, username string) ([]byte, error) {
	data, err := es.client.Get().
		Index(es.userIndex).
		Type(es.userType).
		Id(username).
		FetchSource(true).
		Do(ctx)
	if err != nil {
		return nil, err
	}
	src, err := data.Source.MarshalJSON()
	if err != nil {
		return nil, err
	}

	return src, nil
}

func (es *elasticsearch) putPermission(ctx context.Context, p permission.Permission) (bool, error) {
	_, err := es.client.Index().
		Index(es.permissionIndex).
		Type(es.permissionType).
		Id(p.Username).
		BodyJson(p).
		Do(ctx)
	if err != nil {
		return false, err
	}

	return true, nil
}

func (es *elasticsearch) getPermission(ctx context.Context, username string) (*permission.Permission, error) {
	data, err := es.getRawPermission(ctx, username)
	if err != nil {
		return nil, err
	}

	var p permission.Permission
	err = json.Unmarshal(data, &p)
	if err != nil {
		return nil, err
	}

	return &p, nil
}

func (es *elasticsearch) getRawPermission(ctx context.Context, username string) ([]byte, error) {
	resp, err := es.client.Get().
		Index(es.permissionIndex).
		Type(es.permissionType).
		Id(username).
		FetchSource(true).
		Do(ctx)
	if err != nil {
		return nil, err
	}

	src, err := resp.Source.MarshalJSON()
	if err != nil {
		return nil, err
	}

	return src, nil
}
