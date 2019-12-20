package permissions

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/appbaseio/arc/model/category"
	"github.com/appbaseio/arc/model/op"
	"github.com/appbaseio/arc/model/permission"
	"github.com/appbaseio/arc/util"
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
	category.Auth,
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
	AuthLimit:        30,
}

var createPermissionResponse = map[string]interface{}{
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
		"templates_limit":   0,
		"suggestions_limit": 0,
		"streams_limit":     10,
	},
}

var ipSourceTestPermissionsRequest = map[string]interface{}{
	"role":           "",
	"categories":     adminCategories,
	"acls":           category.ACLsFor(adminCategories...),
	"ops":            adminOps,
	"indices":        []string{"*"},
	"sources":        []string{"10.10.10.10/22"},
	"referers":       []string{"*"},
	"ttl":            -1,
	"limits":         &defaultAdminLimits,
	"description":    "TEST IP SOURCE",
	"include_fields": nil,
	"exclude_fields": nil,
}

var ipSourcesTestPermissionsRequest = map[string]interface{}{
	"role":           "",
	"categories":     adminCategories,
	"acls":           category.ACLsFor(adminCategories...),
	"ops":            adminOps,
	"indices":        []string{"*"},
	"sources":        []string{"10.10.10.10/22","100.100.100.100/24"},
	"referers":       []string{"*"},
	"ttl":            -1,
	"limits":         &defaultAdminLimits,
	"description":    "TEST IP SOURCES",
	"include_fields": nil,
	"exclude_fields": nil,
}

var httpRefererFailTestPermissionRequest = map[string]interface{}{
	"role":           "",
	"categories":     adminCategories,
	"acls":           category.ACLsFor(adminCategories...),
	"ops":            adminOps,
	"indices":        []string{"*"},
	"sources":        []string{"0.0.0.0/0"},
	"referers":       []string{"http://test.com/*"},
	"ttl":            -1,
	"limits":         &defaultAdminLimits,
	"description":    "TEST HTTP REFERER",
	"include_fields": nil,
	"exclude_fields": nil,
}

var sourceFilteringTestPermissionRequest = map[string]interface{}{
	"role":           "",
	"categories":     adminCategories,
	"acls":           category.ACLsFor(adminCategories...),
	"ops":            adminOps,
	"indices":        []string{"*"},
	"sources":        []string{"0.0.0.0/0"},
	"referers":       []string{"*"},
	"ttl":            -1,
	"limits":         &defaultAdminLimits,
	"description":    "TEST SOURCE FILTERING",
	"include_fields": []string{"description","ttl","username"},
	"exclude_fields": nil,
}

var ttlTestPermissionRequest = map[string]interface{}{
	"role":           "",
	"categories":     adminCategories,
	"acls":           category.ACLsFor(adminCategories...),
	"ops":            adminOps,
	"indices":        []string{"*"},
	"sources":        []string{"0.0.0.0/0"},
	"referers":       []string{"*"},
	"ttl":            3,
	"limits":         &defaultAdminLimits,
	"description":    "TEST TTL LIMIT",
	"include_fields": nil,
	"exclude_fields": nil,
}

var createTTLPermissionResponse = map[string]interface{}{
	"role":           "",
	"categories":     adminCategories,
	"acls":           category.ACLsFor(adminCategories...),
	"ops":            adminOps,
	"indices":        []string{"*"},
	"sources":        []string{"0.0.0.0/0"},
	"referers":       []string{"*"},
	"ttl":            3,
	"limits":         &defaultAdminLimits,
	"description":    "TEST TTL LIMIT",
	"include_fields": nil,
	"exclude_fields": nil,
}

var categoryTestPermissionRequest = map[string]interface{}{
	"role":           "",
	"categories":     []string{
		"docs",
		"indices",
		"clusters",
		"misc",
		"user",
		"permission",
		"analytics",
		"streams",
		"rules",
		"templates",
		"suggestions",
		"auth",
	},
	"acls":           category.ACLsFor(adminCategories...),
	"ops":            adminOps,
	"indices":        []string{"*"},
	"sources":        []string{"0.0.0.0/0"},
	"referers":       []string{"*"},
	"ttl":            -1,
	"limits":         &defaultAdminLimits,
	"description":    "TEST CATEGORIES",
	"include_fields": nil,
	"exclude_fields": nil,
}

var allPermissionsResponse = []map[string]interface{}{
	createPermissionResponse,
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
			response, err := util.MakeHttpRequest(http.MethodPost, "/_permission", requestBody)

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
			response, err := util.MakeHttpRequest(http.MethodGet, "/_permission/"+username, nil)

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
			response, err := util.MakeHttpRequest(http.MethodGet, "/_permissions", nil)

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
			response, err := util.MakeHttpRequest(http.MethodPatch, "/_permission/"+username, updatePermissionsRequest)

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

			mockMap := util.StructToMap(updatePermissionResponse)

			So(parsedResponse, ShouldResemble, mockMap)
		})

		Convey("Categories Test", func() {
			response, err := util.MakeHttpRequest(http.MethodPatch, "/_permission/"+username, categoryTestPermissionRequest)
			
			if err != nil {
				t.Fatalf("updatePermission Failed %v instead\n", err)
			}

			response, err = util.MakeHttpRequest(http.MethodGet, "http://" + username + ":" + password + "@localhost:8000/.permissions/_search", nil)

			if err != nil {
				t.Fatalf("categoriesTestFailed Failed %v instead\n", err)
			}

			var categoriesErrorResponse = map[string]interface{} {
				"error": map[string]interface{} {
					"code": 401,
					"message": "credential cannot perform \"read\" operation",
					"status": "Unauthorized",
				},
			}

			parsedResponse, _ := response.(map[string]interface{})

			mockMap := util.StructToMap(categoriesErrorResponse)	
			
			So(parsedResponse, ShouldResemble, mockMap)	
		})

		Convey("Single IP Source", func() {
			response, err := util.MakeHttpRequest(http.MethodPatch, "/_permission/"+username, ipSourceTestPermissionsRequest)
			
			if err != nil {
				t.Fatalf("updatePermission Failed %v instead\n", err)
			}

			response, err = util.MakeHttpRequest(http.MethodGet, "http://" + username + ":" + password + "@localhost:8000/.permissions/_search", nil)
			
			if err != nil {
				t.Fatalf("ipSourceTestFailed Failed %v instead\n", err)
			}
	
			var ipSourceErrorResponse = map[string]interface{} {
				"error": map[string]interface{} {
					"code": 401,
					"message": "permission with username " + username + " doesn't have required sources. reqIP = ::1, sources = [" + ipSourceTestPermissionsRequest["sources"].([]string)[0] + "]",
					"status": "Unauthorized",
				},
			}
			
			parsedResponse, _ := response.(map[string]interface{})

			mockMap := util.StructToMap(ipSourceErrorResponse)	
			
			So(parsedResponse, ShouldResemble, mockMap)	
		})

		Convey("Multiple IP Sources", func() {
			response, err := util.MakeHttpRequest(http.MethodPatch, "/_permission/"+username, ipSourcesTestPermissionsRequest)
			
			if err != nil {
				t.Fatalf("updatePermission Failed %v instead\n", err)
			}

			response, err = util.MakeHttpRequest(http.MethodGet, "http://" + username + ":" + password + "@localhost:8000/.permissions/_search", nil)
			
			if err != nil {
				t.Fatalf("ipSourcesTestFailed Failed %v instead\n", err)
			}
			
			sources := ""
			for _, src := range ipSourcesTestPermissionsRequest["sources"].([]string) {
				sources += src + " "
			}

			var ipSourcesErrorResponse = map[string]interface{} {
				"error": map[string]interface{} {
					"code": 401,
					"message": "permission with username " + username + " doesn't have required sources. reqIP = ::1, sources = [" + sources[:len(sources)-1] + "]",
					"status": "Unauthorized",
				},
			}
			
			parsedResponse, _ := response.(map[string]interface{})

			mockMap := util.StructToMap(ipSourcesErrorResponse)	
			
			So(parsedResponse, ShouldResemble, mockMap)	
		})

		Convey("HTTP Referer Fail Test", func() {
			response, err := util.MakeHttpRequest(http.MethodPatch, "/_permission/"+username, httpRefererFailTestPermissionRequest)
			
			if err != nil {
				t.Fatalf("updatePermission Failed %v instead\n", err)
			}

			response, err = util.MakeHttpRequest(http.MethodGet, "http://" + username + ":" + password + "@localhost:8000/.permissions/_search", nil)
			
			if err != nil {
				t.Fatalf("httpRefererTestFailed Failed %v instead\n", err)
			}

			var httpRefererErrorResponse = map[string]interface{} {
				"error": map[string]interface{} {
					"code": 401,
					"message": "permission doesn't have required referers",
					"status": "Unauthorized",
				},
			}

			parsedResponse, _ := response.(map[string]interface{})

			mockMap := util.StructToMap(httpRefererErrorResponse)	
			
			So(parsedResponse, ShouldResemble, mockMap)	
		})

		Convey("TTL Fail Test", func() {
			response, err := util.MakeHttpRequest(http.MethodPatch, "/_permission/"+username, ttlTestPermissionRequest)
			
			if err != nil {
				t.Fatalf("updatePermission Failed %v instead\n", err)
			}

			time.Sleep(3 * time.Second)

			response, err = util.MakeHttpRequest(http.MethodGet, "http://" + username + ":" + password + "@localhost:8000/.permissions/_search", nil)
			
			if err != nil {
				t.Fatalf("ttlTestFailed Failed %v instead\n", err)
			}
			
			var ttlErrorResponse = map[string]interface{} {
				"error": map[string]interface{} {
					"code": 401,
					"message": "permission with username=" + username + " is expired",
					"status": "Unauthorized",
				},
			}

			parsedResponse, _ := response.(map[string]interface{})

			mockMap := util.StructToMap(ttlErrorResponse)	
			
			So(parsedResponse, ShouldResemble, mockMap)	
		})

		Convey("Source Filtering Test", func() {
			response, err := util.MakeHttpRequest(http.MethodPatch, "/_permission/"+username, sourceFilteringTestPermissionRequest)
			
			if err != nil {
				t.Fatalf("updatePermission Failed %v instead\n", err)
			}

			response, err = util.MakeHttpRequest(http.MethodGet, "http://" + username + ":" + password + "@localhost:8000/.permissions/_search", nil)
			
			if err != nil {
				t.Fatalf("sourceFilteringTestFailed Failed %v instead\n", err)
			}

			var sourceFilteringResponse = map[string]interface{} {
				"description": "TEST SOURCE FILTERING",
				"ttl": -1,
				"username": username,
			}

			parsedResponse, _ := response.(map[string]interface{})
			parsedResponse = parsedResponse["hits"].(map[string]interface{})
			parsedResponse = parsedResponse["hits"].(map[string]interface{})
			parsedResponse = parsedResponse["_source"].(map[string]interface{})

			mockMap := util.StructToMap(sourceFilteringResponse)	
			
			So(parsedResponse, ShouldResemble, mockMap)	
		})

		Convey("Delete permission", func() {
			response, err := util.MakeHttpRequest(http.MethodDelete, "/_permission/"+username, nil)

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
