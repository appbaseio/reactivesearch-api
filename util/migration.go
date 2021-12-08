package util

import (
	"fmt"
	"net/http"
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

// Fetch the index mapping manually using the following function
// Make the request directly and return the response accordingly.
func GetIndexMapping(indexName string) (resp *http.Response, err error) {
	// Keep a constant variable to store the URL
	MappingBaseURL := "/%s/_mapping"

	response, err := http.Get(fmt.Sprintf(MappingBaseURL, indexName))
	return response, err
}
