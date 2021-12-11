package util

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	log "github.com/sirupsen/logrus"
)

type Error struct {
	Message string
	Err     error
}

type Migration interface {
	// ConditionCheck method allows you to control the script
	// execution only when a certain confition met
	ConditionCheck() (bool, *Error)
	// This function allows you to execute the migration logic
	// Execute the non-blocking scripts in a go routine and return the Error as nil
	Script() *Error
	// To determine wether to run script synchronously or asynchronously.
	// Sync scripts will cause the fatal error if failed
	IsAsync() bool
}

var migrationScripts []Migration

func GetMigrationScripts() []Migration {
	return migrationScripts
}

// AddMigrationScript allows you to add a migration script
func AddMigrationScript(migration Migration) {
	migrationScripts = append(migrationScripts, migration)
}

// Handle unstrctured JSON data from the mapping endpoint
type IndexMappingResponse map[string]interface{}

// Fetch the index mapping manually using the following function
// Make the request directly and return the response accordingly.
// We will extract the unstructured JSON data from the endpoint
// and parse it to a map so that it can be directly used.
//
// On error, an empty data body will be returned along with the
// error itself.
//
// Errors will be returned accordingly and verbosed if the error
// occurs while extracting the JSON data. There will be no verbose
// if the error occurs while hitting the endpoint. Those errors are
// expected to be handled by the calling function.
func GetIndexMapping(indexName string) (resp IndexMappingResponse, err error) {
	// Keep a constant variable to store the URL
	MappingBaseURL := GetESURL() + "/%s/_mapping"

	// Declare the mapping response variable
	var data IndexMappingResponse

	response, err := http.Get(fmt.Sprintf(MappingBaseURL, indexName))

	if err != nil {
		return data, err
	}

	defer response.Body.Close()

	// Read the body into bytes and try to unmarshall
	// into JSON data.
	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		log.Errorln("error while reading JSON data: ", err)
		return data, err
	}

	json.Unmarshal(body, &data)
	return data, err
}
