package auth

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/appbaseio-confidential/arc/internal/types/permission"
	"github.com/appbaseio-confidential/arc/internal/types/user"
	"github.com/olivere/elastic"
)

type elasticsearch struct {
	url             string
	userIndex       string
	userType        string
	permissionIndex string
	permissionType  string
	client          *elastic.Client
}

func NewES(url, userIndex, userType, permissionIndex, permissionType string) (*elasticsearch, error) {
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
	es := &elasticsearch{
		url,
		userIndex,
		userType,
		permissionIndex,
		permissionType,
		client,
	}

	return es, nil
}

func (es *elasticsearch) putUser(u user.User) (bool, error) {
	_, err := es.client.Index().
		Index(es.userIndex).
		Type(es.userType).
		Id(u.UserId).
		BodyJson(u).
		Do(context.Background())
	if err != nil {
		return false, err
	}

	//raw, _ := json.Marshal(resp)
	//log.Printf("%s: es_response: %s\n", logTag, string(raw))

	return true, nil
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
		Index(es.userIndex).
		Type(es.userType).
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

func (es *elasticsearch) putPermission(p permission.Permission) (bool, error) {
	_, err := es.client.Index().
		Index(es.permissionIndex).
		Type(es.permissionType).
		Id(p.UserName).
		BodyJson(p).
		Do(context.Background())
	if err != nil {
		return false, err
	}

	//raw, _ := json.Marshal(resp)
	//log.Printf("%s: es_response: %s\n", logTag, string(raw))

	return true, nil
}

func (es *elasticsearch) getPermission(username string) (*permission.Permission, error) {
	data, err := es.getRawPermission(username)
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

func (es *elasticsearch) getRawPermission(username string) ([]byte, error) {
	resp, err := es.client.Get().
		Index(es.permissionIndex).
		Type(es.permissionType).
		Id(username).
		FetchSource(true).
		Do(context.Background())
	if err != nil {
		return nil, err
	}

	//raw, _ := json.Marshal(resp)
	//log.Printf("%s: es_response: %v", logTag, string(raw))

	src, err := resp.Source.MarshalJSON()
	if err != nil {
		return nil, err
	}

	return src, nil
}
