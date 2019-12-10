package users

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/appbaseio/arc/model/category"
	"github.com/appbaseio/arc/util"
	. "github.com/smartystreets/goconvey/convey"
)

var createUserRequest = map[string]interface{}{
	"username":   "john",
	"password":   "appleseed",
	"email":      "john@appleseed.com",
	"indices":    []string{"logs-*"},
	"categories": []string{"docs"},
	"ops":        []string{"read"},
}

var createUserResponse = map[string]interface{}{
	"username":           "john",
	"email":              "john@appleseed.com",
	"indices":            []string{"logs-*"},
	"categories":         []string{"docs"},
	"ops":                []string{"read"},
	"is_admin":           false,
	"password_hash_type": "bcrypt",
	"acls":               category.ACLsFor([]category.Category{category.Docs}...),
}

var updateUserRequest = map[string]interface{}{
	"password": "new_password",
	"email":    "john_2@appleseed.com",
	"indices":  []string{"*"},
	"ops":      []string{"read", "write"},
}

var defaultUser = map[string]interface{}{
	"username":           "foo",
	"password_hash_type": "bcrypt",
	"is_admin":           true,
	"categories": []string{
		"docs",
		"search",
		"indices",
		"cat",
		"clusters",
		"misc",
		"user",
		"permission",
		"analytics",
		"streams",
		"rules",
		"templates",
		"suggestions",
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
		"cat",
		"remote",
		"nodes",
		"tasks",
		"cluster",
		"scripts",
		"ingest",
		"snapshot",
	},
	"email": "",
	"ops": []string{
		"read",
		"write",
		"delete",
	},
	"indices": []string{
		"*",
	},
}

func TestUser(t *testing.T) {
	username, _ := createUserRequest["username"].(string)
	Convey("Testing users", t, func() {
		Convey("Create an user", func() {
			response, err := util.MakeHttpRequest(http.MethodPost, "/_user", createUserRequest)

			parsedResponse, _ := response.(map[string]interface{})

			if err != nil {
				t.Fatalf("createUserTest Failed %v instead\n", err)
			}

			delete(parsedResponse, "password")
			delete(parsedResponse, "created_at")

			mockMap := util.StructToMap(createUserResponse)

			So(parsedResponse, ShouldResemble, mockMap)
		})

		Convey("Get user", func() {
			response, err := util.MakeHttpRequest(http.MethodGet, "/_user/"+username, nil)

			if err != nil {
				t.Fatalf("getUserTest Failed %v instead\n", err)
			}

			parsedResponse, _ := response.(map[string]interface{})

			if err != nil {
				t.Fatalf("createUserTest Failed %v instead\n", err)
			}

			delete(parsedResponse, "password")
			delete(parsedResponse, "created_at")

			mockMap := util.StructToMap(createUserResponse)

			So(parsedResponse, ShouldResemble, mockMap)
		})

		Convey("Get users", func() {
			response, err := util.MakeHttpRequest(http.MethodGet, "/_users", nil)

			if err != nil {
				t.Fatalf("getUsersTest Failed %v instead\n", err)
			}
			var getUsersResponse = []map[string]interface{}{
				defaultUser,
				createUserResponse,
			}

			var mockMap []interface{}
			parsedResponse, _ := response.([]interface{})

			for _, v := range parsedResponse {
				parsedValue, _ := v.(map[string]interface{})
				delete(parsedValue, "created_at")
				delete(parsedValue, "password")
			}
			marshalled, _ := json.Marshal(getUsersResponse)
			json.Unmarshal(marshalled, &mockMap)
			So(parsedResponse, ShouldResemble, mockMap)
		})

		Convey("Update user", func() {
			response, err := util.MakeHttpRequest(http.MethodPatch, "/_user/"+username, updateUserRequest)

			if err != nil {
				t.Fatalf("updateUserTest Failed %v instead\n", err)
			}

			parsedResponse, _ := response.(map[string]interface{})

			delete(parsedResponse, "_seq_no")

			var updateUserResponse = map[string]interface{}{
				"_index":   ".users",
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

			mockMap := util.StructToMap(updateUserResponse)

			So(parsedResponse, ShouldResemble, mockMap)
		})

		Convey("Delete user", func() {
			response, err := util.MakeHttpRequest(http.MethodDelete, "/_user/"+username, nil)

			if err != nil {
				t.Fatalf("deleteUserTest Failed %v instead\n", err)
			}

			var deleteUserResponse = map[string]interface{}{
				"code":    200,
				"message": "user with \"username\"=\"" + username + "\" deleted",
				"status":  "OK",
			}

			mockMap := util.StructToMap(deleteUserResponse)

			So(response, ShouldResemble, mockMap)
		})
	})
}
