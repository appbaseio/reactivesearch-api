package user

import (
	"context"
	"encoding/json"
	"log"

	"github.com/appbaseio-confidential/arc/internal/types/op"
	"github.com/appbaseio-confidential/arc/internal/types/user"
)

func getRawUser(userId string) ([]byte, error) {
	data, err := es.client.Get().
		Index(userIndex).
		Type(userType).
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

func putUser(u user.User) (bool, error) {
	resp, err := es.client.Index().
		Index(userIndex).
		Type(userType).
		Id(u.UserId).
		BodyJson(u).
		Do(context.Background())
	if err != nil {
		return false, err
	}

	raw, _ := json.Marshal(resp)
	log.Printf("es_response: %s\n", string(raw))

	return true, nil
}

func patchUser(userId string, u user.User) (bool, error) {
	// Only consider fields that can be updated
	fields := make(map[string]interface{})
	if u.ACL != nil && len(u.ACL) >= 0 {
		fields["acl"] = u.ACL
	}
	if u.Email != "" {
		fields["email"] = u.Email
	}
	if u.Op != op.Noop {
		fields["op"] = u.Op
	}
	if u.Indices != nil && len(u.Indices) >= 0 {
		fields["indices"] = u.Indices
	}

	resp, err := es.client.Update().
		Index(userIndex).
		Type(userType).
		Id(userId).
		Doc(fields).
		Do(context.Background())
	if err != nil {
		return false, err
	}

	raw, _ := json.Marshal(resp)
	log.Printf("es_response: %s\n", string(raw))
	return true, nil
}

func deleteUser(userId string) (bool, error) {
	resp, err := es.client.Delete().
		Index(userIndex).
		Type(userType).
		Id(userId).
		Do(context.Background())
	if err != nil {
		return false, err
	}

	raw, _ := json.Marshal(resp)
	log.Printf("es_response: %s\n", string(raw))

	return true, nil
}
