//go:build !unit
// +build !unit

package permissions

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/appbaseio/reactivesearch-api/model/category"
	"github.com/appbaseio/reactivesearch-api/model/op"
	"github.com/appbaseio/reactivesearch-api/model/permission"
	"github.com/appbaseio/reactivesearch-api/util"
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
	category.Suggestions,
	category.Auth,
	category.ReactiveSearch,
	category.SearchRelevancy,
	category.Synonyms,
	category.SearchGrader,
	category.UIBuilder,
	category.Logs,
	category.Cache,
	category.StoredQuery,
	category.Sync,
	category.Pipelines,
}

var adminOps = []op.Operation{
	op.Read,
	op.Write,
	op.Delete,
}

var defaultAdminLimits = permission.Limits{
	IPLimit:               7200,
	DocsLimit:             30,
	SearchLimit:           30,
	IndicesLimit:          30,
	CatLimit:              30,
	ClustersLimit:         30,
	MiscLimit:             30,
	UserLimit:             30,
	PermissionLimit:       30,
	AnalyticsLimit:        30,
	RulesLimit:            30,
	SuggestionsLimit:      30,
	StreamsLimit:          30,
	AuthLimit:             30,
	ReactiveSearchLimit:   30,
	SearchRelevancyLimit:  30,
	SearchGraderLimit:     30,
	EcommIntegrationLimit: 30,
	LogsLimit:             30,
	SynonymsLimit:         30,
	CacheLimit:            30,
	StoredQueryLimit:      30,
	SyncLimit:             30,
	PipelinesLimit:        30,
}

var createPermissionResponse = map[string]interface{}{
	"owner":          "foo",
	"creator":        "foo",
	"role":           "",
	"categories":     adminCategories,
	"acls":           category.ACLsFor(adminCategories...),
	"ops":            adminOps,
	"sources":        []string{"0.0.0.0/0"},
	"referers":       []string{"*"},
	"ttl":            -1,
	"limits":         &defaultAdminLimits,
	"description":    "TEST PERMISSION",
	"include_fields": nil,
	"exclude_fields": nil,
	"expired":        false,
}

var updatePermissionsRequest = map[string]interface{}{
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
		"suggestions_limit": 0,
		"streams_limit":     10,
	},
}

var allPermissionsResponse = []map[string]interface{}{
	createPermissionResponse,
}

func TestPermission(t *testing.T) {
	// variables to cache permission credentials
	var username string
	var password string
	var createdAt string
	build := util.BuildArc{}
	util.StartArc(&build)
	build.Start()
	defer build.Close()
	Convey("Testing permissions", t, func() {
		Convey("Create permission", func() {
			requestBody := permission.Permission{
				Description: "TEST PERMISSION",
			}
			response, err, _ := util.MakeHttpRequest(http.MethodPost, "/_permission", requestBody)

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

			mockMap := util.StructToMap(createPermissionResponse)

			So(parsedResponse, ShouldResemble, mockMap)
		})

		Convey("Get permission", func() {
			response, err, _ := util.MakeHttpRequest(http.MethodGet, "/_permission/"+username, nil)

			if err != nil {
				t.Fatalf("getPermissionTest Failed %v instead\n", err)
			}
			var getPermissionResponse = createPermissionResponse
			getPermissionResponse["username"] = username
			getPermissionResponse["password"] = password
			getPermissionResponse["created_at"] = createdAt
			mockMap := util.StructToMap(getPermissionResponse)

			So(response, ShouldResemble, mockMap)
		})

		Convey("Get permissions", func() {
			response, err, _ := util.MakeHttpRequest(http.MethodGet, "/_permissions", nil)

			if err != nil {
				t.Fatalf("getPermissionsTest Failed %v instead\n", err)
			}
			var getPermissionsResponse = allPermissionsResponse
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
			response, err, _ := util.MakeHttpRequest(http.MethodPatch, "/_permission/"+username, updatePermissionsRequest)

			if err != nil {
				t.Fatalf("updatePermissionTest Failed %v instead\n", err)
			}

			parsedResponse, _ := response.(map[string]interface{})

			delete(parsedResponse, "_seq_no")

			var updatePermissionResponse = map[string]interface{}{
				"code":    200,
				"message": "permission is updated successfully",
				"status":  "OK",
			}

			mockMap := util.StructToMap(updatePermissionResponse)

			So(parsedResponse, ShouldResemble, mockMap)
		})

		Convey("Delete permission", func() {
			response, err, _ := util.MakeHttpRequest(http.MethodDelete, "/_permission/"+username, nil)

			if err != nil {
				t.Fatalf("deletePermissionTest Failed %v instead\n", err)
			}

			var deletePermissionResponse = map[string]interface{}{
				"code":    200,
				"message": "permission with \"username\"=\"" + username + "\" deleted",
				"status":  "OK",
			}

			mockMap := util.StructToMap(deletePermissionResponse)
			parsedResponse, _ := response.(map[string]interface{})
			delete(parsedResponse, "_seq_no")

			So(parsedResponse, ShouldResemble, mockMap)
		})
	})
}
