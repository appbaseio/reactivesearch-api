package permission

import (
	"log"
	"os"

	"github.com/appbaseio-confidential/arc/arc"
	"github.com/appbaseio-confidential/arc/internal/types/permission"
)

var (
	permissionIndex string
	permissionType  string
	es              *ElasticSearch
)

func init() {
	arc.RegisterPlugin(arc.NewPlugin("permission", arc.InitFunc(setup), routes))
}

func setup() {
	log.Println("[INFO] initializing plugin: 'permission'")

	// TODO: Consider panicking if invalid url fetched?
	url := os.Getenv("PERMISSION_ES_URL")
	permissionIndex = os.Getenv("PERMISSION_ES_INDEX")
	permissionType = "type_permission"
	mapping := permission.IndexMapping

	var err error
	es, err = NewES(url, permissionIndex, mapping)
	if err != nil {
		log.Fatalf("[ERROR] initializing permission's elasticsearch dao: %v", err)
	}
}
