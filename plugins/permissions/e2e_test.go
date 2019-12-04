package permissions

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"testing"

	"github.com/appbaseio/arc/model/category"
	"github.com/appbaseio/arc/model/op"
	"github.com/appbaseio/arc/model/permission"
	. "github.com/smartystreets/goconvey/convey"
)

var adminCategories = []category.Category{
	category.Docs,
	category.Search,
	category.Indices,
	category.Cat,
	category.Clusters,
	category.Misc,
	category.User,
	category.Permission,
	category.Analytics,
	category.Streams,
	category.Rules,
	category.Templates,
	category.Suggestions,
}

var defaultOps = []op.Operation{
	op.Read,
}

var adminOps = []op.Operation{
	op.Read,
	op.Write,
	op.Delete,
}

var defaultAdminLimits = permission.Limits{
	IPLimit:          7200,
	DocsLimit:        30,
	SearchLimit:      30,
	IndicesLimit:     30,
	CatLimit:         30,
	ClustersLimit:    30,
	MiscLimit:        30,
	UserLimit:        30,
	PermissionLimit:  30,
	AnalyticsLimit:   30,
	RulesLimit:       30,
	TemplatesLimit:   30,
	SuggestionsLimit: 30,
	StreamsLimit:     30,
}

var p = map[string]interface{}{
	"owner":          "foo",
	"creator":        "foo",
	"role":           "",
	"categories":     adminCategories,
	"acls":           category.ACLsFor(adminCategories...),
	"ops":            adminOps,
	"indices":        []string{"*"},
	"sources":        []string{"0.0.0.0/0"},
	"referers":       []string{"*"},
	"ttl":            -1,
	"limits":         &defaultAdminLimits,
	"description":    "TEST PERMISSION",
	"include_fields": nil,
	"exclude_fields": nil,
}

var updatedPermission = map[string]interface{}{
	"owner":          "foo",
	"creator":        "foo",
	"role":           "",
	"categories":     adminCategories,
	"acls":           category.ACLsFor(adminCategories...),
	"ops":            adminOps,
	"indices":        []string{"*"},
	"sources":        []string{"0.0.0.0/0"},
	"referers":       []string{"*"},
	"ttl":            -1,
	"limits":         &defaultAdminLimits,
	"description":    "TEST PERMISSION UPDATED",
	"include_fields": nil,
	"exclude_fields": nil,
}

var allPermissions = []map[string]interface{}{
	p,
}

func structToMap(response interface{}) interface{} {
	var mockMap map[string]interface{}
	marshalled, _ := json.Marshal(response)
	json.Unmarshal(marshalled, &mockMap)
	return mockMap
}

func makeHttpRequest(method string, url string, requestBody interface{}) (interface{}, error) {
	var response interface{}
	finalURL := TestURL + url
	marshalledRequest, err := json.Marshal(requestBody)
	if err != nil {
		log.Println("error while marshalling req body: ", err)
		return nil, err
	}
	req, _ := http.NewRequest(method, finalURL, bytes.NewBuffer(marshalledRequest))
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("cache-control", "no-cache")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Println("error while sending request: ", err)
		return nil, err
	}
	defer res.Body.Close()
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		log.Println("error reading res body: ", err)
		return nil, err
	}
	err = json.Unmarshal(body, &response)

	if err != nil {
		log.Println("error while unmarshalling res body: ", err)
		return response, err
	}
	return response, nil
}

func TestPermission(t *testing.T) {
	// variables to cache permission credentials
	var username string
	var password string
	var createdAt string
	Convey("Testing permissions", t, func() {
		Convey("Create permission", func() {
			requestBody := permission.Permission{
				Description: "TEST PERMISSION",
			}
			response, err := makeHttpRequest(http.MethodPost, "/_permission", requestBody)

			parsedResponse, _ := response.(map[string]interface{})

			if err != nil {
				t.Fatalf("createPermissionTest Failed %v instead\n", err)
			}
			username, _ = parsedResponse["username"].(string)
			password, _ = parsedResponse["password"].(string)
			createdAt, _ = parsedResponse["created_at"].(string)

			delete(parsedResponse, "username")
			delete(parsedResponse, "password")
			delete(parsedResponse, "created_at")

			mockMap := structToMap(p)

			So(parsedResponse, ShouldResemble, mockMap)
		})

		Convey("Get permission", func() {
			response, err := makeHttpRequest(http.MethodGet, "/_permission/"+username, nil)

			if err != nil {
				t.Fatalf("getPermissionTest Failed %v instead\n", err)
			}
			var getPermissionResponse = p
			getPermissionResponse["username"] = username
			getPermissionResponse["password"] = password
			getPermissionResponse["created_at"] = createdAt
			mockMap := structToMap(getPermissionResponse)

			So(response, ShouldResemble, mockMap)
		})

		Convey("Get permissions", func() {
			response, err := makeHttpRequest(http.MethodGet, "/_permissions", nil)

			if err != nil {
				t.Fatalf("getPermissionTest Failed %v instead\n", err)
			}
			var getPermissionsResponse = allPermissions
			getPermissionsResponse[0]["username"] = username
			getPermissionsResponse[0]["password"] = password
			getPermissionsResponse[0]["created_at"] = createdAt
			var mockMap []interface{}
			parsedResponse, _ := response.([]interface{})
			marshalled, _ := json.Marshal(getPermissionsResponse)
			json.Unmarshal(marshalled, &mockMap)
			So(parsedResponse, ShouldResemble, mockMap)
		})

		Convey("Update permission", func() {
			requestBody := map[string]interface{}{
				"description": "TEST PERMISSION UPDATED",
				"role":        "role",
				"categories": []string{
					"docs",
					"search",
					"indices",
					"clusters",
					"misc",
					"user",
					"permission",
					"analytics",
					"streams",
					"rules",
				},
				"acls": []string{
					"reindex",
					"termvectors",
					"update",
					"create",
					"mtermvectors",
					"bulk",
					"delete",
					"source",
					"delete_by_query",
					"get",
					"mget",
					"update_by_query",
					"index",
					"exists",
					"field_caps",
					"msearch",
					"validate",
					"rank_eval",
					"render",
					"search_shards",
					"search",
					"count",
					"explain",
					"upgrade",
					"settings",
					"indices",
					"split",
					"aliases",
					"stats",
					"template",
					"open",
					"mapping",
					"recovery",
					"analyze",
					"cache",
					"forcemerge",
					"alias",
					"refresh",
					"segments",
					"close",
					"flush",
					"shrink",
					"shard_stores",
					"rollover",
					"remote",
					"cat",
					"nodes",
					"tasks",
					"cluster",
					"scripts",
					"ingest",
					"snapshot",
				},
				"ops": []string{
					"write",
				},
				"ttl": 3600,
				"limits": map[string]interface{}{
					"ip_limit":          7200,
					"docs_limit":        5,
					"search_limit":      2,
					"indices_limit":     10,
					"cat_limit":         0,
					"clusters_limit":    10,
					"misc_limit":        10,
					"user_limit":        10,
					"permission_limit":  10,
					"analytics_limit":   10,
					"rules_limit":       10,
					"templates_limit":   0,
					"suggestions_limit": 0,
					"streams_limit":     10,
				},
			}
			response, err := makeHttpRequest(http.MethodPatch, "/_permission/"+username, requestBody)

			if err != nil {
				t.Fatalf("updatePermissionTest Failed %v instead\n", err)
			}

			parsedResponse, _ := response.(map[string]interface{})

			delete(parsedResponse, "_seq_no")

			var updatePermissionResponse = map[string]interface{}{
				"_index":   ".permissions",
				"_type":    "_doc",
				"_id":      username,
				"_version": 2,
				"result":   "updated",
				"_shards": map[string]interface{}{
					"total":      1,
					"successful": 1,
					"failed":     0,
				},
				"_primary_term": 1,
			}

			mockMap := structToMap(updatePermissionResponse)

			So(parsedResponse, ShouldResemble, mockMap)
		})

		Convey("Delete permission", func() {
			response, err := makeHttpRequest(http.MethodDelete, "/_permission/"+username, nil)

			if err != nil {
				t.Fatalf("deletePermissionTest Failed %v instead\n", err)
			}

			var deletePermissionResponse = map[string]interface{}{
				"code":    200,
				"message": "permission with \"username\"=\"" + username + "\" deleted",
				"status":  "OK",
			}

			mockMap := structToMap(deletePermissionResponse)
			parsedResponse, _ := response.(map[string]interface{})
			delete(parsedResponse, "_seq_no")

			So(parsedResponse, ShouldResemble, mockMap)
		})
	})
}
