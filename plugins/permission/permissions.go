package permission

import (
	"context"
	"encoding/json"
	"log"

	"github.com/appbaseio-confidential/arc/internal/types/op"
	"github.com/appbaseio-confidential/arc/internal/types/permission"
)

func getRawPermissions(username string) ([]byte, error) {
	resp, err := es.client.Get().
		Index(permissionIndex).
		Type(permissionType).
		Id(username).
		FetchSource(true).
		Do(context.Background())
	if err != nil {
		return nil, err
	}

	raw, _ := json.Marshal(resp)
	log.Printf("es_response: %v", raw)

	src, err := resp.Source.MarshalJSON()
	if err != nil {
		return nil, err
	}

	return src, nil
}

func putPermission(p permission.Permission) (bool, error) {
	resp, err := es.client.Index().
		Index(permissionIndex).
		Type(permissionType).
		Id(p.UserName).
		BodyJson(p).
		Do(context.Background())
	if err != nil {
		return false, err
	}

	raw, _ := json.Marshal(resp)
	log.Printf("es_response: %s\n", raw)

	return true, nil
}

func patchPermission(username string, p permission.Permission) (bool, error) {
	fields := make(map[string]interface{})
	if p.ACL != nil {
		fields["acl"] = p.ACL
	}
	if p.Op != op.Noop {
		fields["op"] = p.Op
	}
	if p.Indices != nil {
		fields["indices"] = p.Indices
	}

	resp, err := es.client.Update().
		Index(permissionIndex).
		Type(permissionType).
		Id(username).
		Doc(fields).
		Do(context.Background())
	if err != nil {
		return false, nil
	}

	raw, _ := json.Marshal(resp)
	log.Printf("es_response: %s\n", raw)

	return true, nil
}

func deletePermission(userId string) (bool, error) {
	resp, err := es.client.Delete().
		Index(permissionIndex).
		Type(permissionType).
		Id(userId).
		Do(context.Background())
	if err != nil {
		return false, err
	}

	raw, _ := json.Marshal(resp)
	log.Printf("es_response: %s\n", raw)

	return true, nil
}
