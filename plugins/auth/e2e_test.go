package auth

import (
	"net/http"
	"testing"

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
	category.Functions,
	category.ReactiveSearch,
	category.SearchRelevancy,
	category.Synonyms,
}

var adminOps = []op.Operation{
	op.Read,
	op.Write,
	op.Delete,
}

var defaultAdminLimits = permission.Limits{
	IPLimit:              7200,
	DocsLimit:            30,
	SearchLimit:          30,
	IndicesLimit:         30,
	CatLimit:             30,
	ClustersLimit:        30,
	MiscLimit:            30,
	UserLimit:            30,
	PermissionLimit:      30,
	AnalyticsLimit:       30,
	RulesLimit:           30,
	TemplatesLimit:       30,
	SuggestionsLimit:     30,
	StreamsLimit:         30,
	AuthLimit:            30,
	FunctionsLimit:       30,
	ReactiveSearchLimit:  30,
	SearchRelevancyLimit: 30,
}

var createPermissionResponse = map[string]interface{}{
	"owner":          "foo",
	"creator":        "foo",
	"role":           "admin",
	"categories":     adminCategories,
	"acls":           category.ACLsFor(adminCategories...),
	"ops":            adminOps,
	"indices":        []string{"*"},
	"sources":        []string{"0.0.0.0/0"},
	"referers":       []string{"*"},
	"ttl":            -1,
	"limits":         &defaultAdminLimits,
	"description":    "TEST PERMISSION WITH ROLE",
	"include_fields": nil,
	"exclude_fields": nil,
	"expired":        false,
}

var updatePermissionsRequest = map[string]interface{}{
	"description": "TEST PERMISSION UPDATED",
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

var roleName = "admin"

var savePublicKeyRequest = map[string]interface{}{
	"public_key": "LS0tLS1CRUdJTiBQVUJMSUMgS0VZLS0tLS0KTUlJQklqQU5CZ2txaGtpRzl3MEJBUUVGQUFPQ0FROEFNSUlCQ2dLQ0FRRUFuenlpczFaamZOQjBiQmdLRk1Tdgp2a1R0d2x2QnNhSnE3UzV3QStremVWT1ZwVld3a1dkVmhhNHMzOFhNL3BhL3lyNDdhdjcrejNWVG12RFJ5QUhjCmFUOTJ3aFJFRnBMdjljajVsVGVKU2lieXIvTXJtL1l0akNaVldnYU9ZSWh3clh3S0xxUHIvMTFpbldzQWtmSXkKdHZIV1R4WllFY1hMZ0FYRnVVdWFTM3VGOWdFaU5Rd3pHVFUxdjBGcWtxVEJyNEI4blczSENONDdYVXUwdDhZMAplK2xmNHM0T3hRYXdXRDc5SjkvNWQzUnkwdmJWM0FtMUZ0R0ppSnZPd1JzSWZWQ2hEcFlTdFRjSFRDTXF0dldiClY2TDExQldrcHpHWFNXNEh2NDNxYStHU1lPRDJRVTY4TWI1OW9TazJPQitCdE9McEpvZm1iR0VHZ3Ztd3lDSTkKTXdJREFRQUIKLS0tLS1FTkQgUFVCTElDIEtFWS0tLS0t",
	"role_key":   roleName,
}

var savePublicKeyResponse = map[string]interface{}{
	"message": "Public key saved successfully.",
}

func TestRBAC(t *testing.T) {
	var username string
	var password string
	var createdAt string
	build := util.BuildArc{}
	util.StartArc(&build)
	build.Start()
	defer build.Close()
	Convey("Testing RBAC", t, func() {
		Convey("Save the public key", func() {
			response, err, _ := util.MakeHttpRequest(http.MethodPut, "/_public_key", savePublicKeyRequest)

			if err != nil {
				t.Fatalf("savePublicKeyTest Failed %v instead\n", err)
			}

			So(response, ShouldResemble, savePublicKeyResponse)
		})

		Convey("Get the public key", func() {
			response, err, _ := util.MakeHttpRequest(http.MethodGet, "/_public_key", nil)

			if err != nil {
				t.Fatalf("getPublicKeyTest Failed %v instead\n", err)
			}

			So(response, ShouldResemble, savePublicKeyRequest)
		})

		Convey("Create permission with role", func() {
			requestBody := permission.Permission{
				Description: "TEST PERMISSION WITH ROLE",
			}
			response, err, _ := util.MakeHttpRequest(http.MethodPost, "/_role/"+roleName, requestBody)

			parsedResponse, _ := response.(map[string]interface{})

			if err != nil {
				t.Fatalf("createPermissionWithRoleTest Failed %v instead\n", err)
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

		Convey("Get permission with role", func() {
			response, err, _ := util.MakeHttpRequest(http.MethodGet, "/_role/"+roleName, nil)

			if err != nil {
				t.Fatalf("getPermissionWithRoleTest Failed %v instead\n", err)
			}
			var getPermissionResponse = createPermissionResponse
			getPermissionResponse["username"] = username
			getPermissionResponse["password"] = password
			getPermissionResponse["created_at"] = createdAt
			mockMap := util.StructToMap(getPermissionResponse)

			So(response, ShouldResemble, mockMap)
		})

		Convey("Update permission with role", func() {
			response, err, _ := util.MakeHttpRequest(http.MethodPatch, "/_role/"+roleName, updatePermissionsRequest)

			if err != nil {
				t.Fatalf("updatePermissionWithRoleTest Failed %v instead\n", err)
			}

			parsedResponse, _ := response.(map[string]interface{})

			delete(parsedResponse, "_seq_no")

			var updatePermissionResponse = map[string]interface{}{
				"code":    200,
				"message": "Permission is updated successfully",
				"status":  "OK",
			}

			mockMap := util.StructToMap(updatePermissionResponse)

			So(parsedResponse, ShouldResemble, mockMap)
		})

		Convey("Delete permission with role", func() {
			response, err, _ := util.MakeHttpRequest(http.MethodDelete, "/_role/"+roleName, nil)

			if err != nil {
				t.Fatalf("deletePermissionWithRoleTest Failed %v instead\n", err)
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
