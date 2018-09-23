package user

import (
	"log"
	"os"

	"github.com/appbaseio-confidential/arc/arc"
	"github.com/appbaseio-confidential/arc/internal/types/user"
)

var (
	userIndex string
	userType  string
	es    *ElasticSearch
)

func init() {
	arc.RegisterPlugin(arc.NewPlugin("user", arc.InitFunc(setup), routes))
}

func setup() {
	log.Println("[INFO] initializing plugin: 'user'")

	// TODO: Consider panicking if invalid url fetched?
	esURL := os.Getenv("USER_ES_URL")
	userIndex = os.Getenv("USER_ES_INDEX")
	userType = "type_user"
	mapping := user.IndexMapping

	var err error
	es, err = NewES(esURL, userIndex, mapping)
	if err != nil {
		log.Fatalf("error initializing user's elasticsearch dao: %v", err)
		return
	}
}
