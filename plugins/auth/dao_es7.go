package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	log "github.com/sirupsen/logrus"

	"github.com/appbaseio/reactivesearch-api/model/credential"
	"github.com/appbaseio/reactivesearch-api/model/permission"
	"github.com/appbaseio/reactivesearch-api/model/user"
	"github.com/appbaseio/reactivesearch-api/util"
	es7 "github.com/olivere/elastic/v7"
)

func (es *elasticsearch) getPublicKeyEs7(ctx context.Context, publicKeyIndex, publicKeyDocID string) (publicKey, error) {
	var record = publicKey{}
	response, err := util.GetInternalClient7().Get().
		Index(publicKeyIndex).
		Id(publicKeyDocID).
		Do(ctx)
	if response == nil {
		return record, errors.New("public key record not found")
	}
	err = json.Unmarshal(response.Source, &record)
	if err != nil {
		log.Errorln(logTag, ": error retrieving publickey record", err)
		return record, err
	}
	return record, nil
}

func (es *elasticsearch) getCredentialEs7(ctx context.Context, username string) (credential.AuthCredential, error) {
	matchUsername := es7.NewTermQuery("username.keyword", username)

	response, err := util.GetInternalClient7().Search().
		Index(es.userIndex, es.permissionIndex).
		Query(matchUsername).
		FetchSource(true).
		Do(ctx)
	if err != nil {
		return nil, err
	}

	if len(response.Hits.Hits) > 1 {
		return nil, fmt.Errorf(`more than one result for "username"="%s"`, username)
	}

	// there should be either 0 or 1 hit
	var obj credential.AuthCredential
	for _, hit := range response.Hits.Hits {
		if hit.Index == es.userIndex {
			var u user.User
			if hit.Source != nil {
				err := json.Unmarshal(hit.Source, &u)
				if err != nil {
					return nil, err
				}
				obj = &u
			}
		} else if hit.Index == es.permissionIndex {
			var p permission.Permission

			// unmarshal into permission
			err := json.Unmarshal(hit.Source, &p)
			if err != nil {
				return nil, err
			}

			obj = &p
		}
	}

	if obj == nil {
		return nil, fmt.Errorf(`invalid username or password`)
	}

	return obj, nil
}

func (es *elasticsearch) getRawRolePermissionEs7(ctx context.Context, role string) ([]byte, error) {
	resp, err := util.GetInternalClient7().Search().
		Index(es.permissionIndex).
		Type(es.permissionType).
		Query(es7.NewTermQuery("role.keyword", role)).
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
