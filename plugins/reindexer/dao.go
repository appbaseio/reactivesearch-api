package reindexer

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/appbaseio/arc/util"
	es7 "github.com/olivere/elastic/v7"
)

// Reindex Inplace: https://www.elastic.co/guide/en/elasticsearch/reference/current/reindex-upgrade-inplace.html
//
// 1. Create a new index and copy the mappings and settings from the old index.
// 2. Set the refresh_interval to -1 and the number_of_replicas to 0 for efficient reindexing.
// 3. Reindex all documents from the old index into the new index using the reindex API.
// 4. Reset the refresh_interval and number_of_replicas to the values used in the old index.
// 5. Wait for the index status to change to green.
// 6. In a single update aliases request:
// 	  a. Delete the old index.
//	  b. Add an alias with the old index name to the new index.
// 	  c. Add any aliases that existed on the old index to the new index.
//
// We accept a query param `wait_for_completion` which defaults to true, which when false, we don't create any aliases
// and delete the old index, we instead return the tasks API response.
func reindex(ctx context.Context, indexName string, config *reindexConfig, waitForCompletion bool) ([]byte, error) {
	var err error

	// We fetch the index name pointing to the given alias first.
	// If an index has already been reindexed before, user would
	// pass in the alias i.e. the original name of the index when
	// it was first created. We try to fetch the current index name
	// from the given alias. If alias name doesn't exist we get an
	// empty slice of indices, which means the index has never been
	// reindexed before.
	indices, err := getIndicesByAlias(ctx, indexName)
	if err != nil {
		log.Println(err)
	}
	if len(indices) > 1 {
		return nil, fmt.Errorf(`multiple indices pointing to alias "%s"`, indexName)
	}
	if len(indices) == 1 {
		indexName = indices[0]
	}

	// If mappings are not passed, we fetch the mappings of the old index.
	if config.Mappings == nil {
		config.Mappings, err = mappingsOf(ctx, indexName)
		if err != nil {
			return nil, fmt.Errorf(`error fetching mappings of index "%s": %v`, indexName, err)
		}
	}

	// If settings are not passed, we fetch the settings of the old index.
	if config.Settings == nil {
		config.Settings, err = settingsOf(ctx, indexName)
		if err != nil {
			return nil, fmt.Errorf(`error fetching settings of index "%s": %v`, indexName, err)
		}
	}

	// Setup the destination index prior to running the _reindex action.
	body := make(map[string]interface{})
	body["mappings"] = config.Mappings
	body["settings"] = config.Settings

	newIndexName, err := reindexedName(indexName)
	if err != nil {
		return nil, fmt.Errorf(`error generating a new index name for index "%s": %v`, indexName, err)
	}

	// Create the new index.
	err = createIndex(ctx, newIndexName, body)
	if err != nil {
		return nil, err
	}

	// Configure reindex source
	src := es7.NewReindexSource().
		Index(indexName).
		Type(config.Types...).
		FetchSourceIncludeExclude(config.Include, config.Exclude)

	// Configure reindex dest
	dest := es7.NewReindexDestination().
		Index(newIndexName)

	// Reindex action.
	reindex := util.GetClient7().Reindex().
		Source(src).
		Destination(dest).
		WaitForCompletion(waitForCompletion)

	// If wait_for_completion = true, then we carry out the task synchronously along with three more steps:
	// 	- fetch any aliases of the old index
	//  - delete the old index
	//  - set the aliases of the old index to the new index
	if waitForCompletion {
		response, err := reindex.Do(ctx)
		if err != nil {
			return nil, err
		}

		// Fetch all the aliases of old index
		aliases, err := aliasesOf(ctx, indexName)
		if err != nil {
			return nil, fmt.Errorf(`error fetching aliases of index "%s": %v`, indexName, err)
		}
		aliases = append(aliases, indexName)

		// Delete old index
		err = deleteIndex(ctx, indexName)
		if err != nil {
			return nil, fmt.Errorf(`error deleting index "%s": %v\n`, indexName, err)
		}

		// Set aliases of old index to the new index.
		err = setAlias(ctx, newIndexName, aliases...)
		if err != nil {
			return nil, fmt.Errorf(`error setting alias "%s" for index "%s"`, indexName, newIndexName)
		}

		return json.Marshal(response)
	}

	// If wait_for_completion = false, we carry out the reindexing asynchronously and return the task ID.
	response, err := reindex.DoAsync(context.Background())
	if err != nil {
		return nil, err
	}
	taskID := response.TaskId

	// Get the reindex task by ID
	task, err := util.GetClient7().TasksGetTask().TaskId(taskID).Do(context.Background())
	if err != nil {
		return nil, err
	}

	return json.Marshal(task)
}

func mappingsOf(ctx context.Context, indexName string) (map[string]interface{}, error) {
	response, err := util.GetClient7().GetMapping().
		Index(indexName).
		Do(ctx)
	if err != nil {
		return nil, err
	}

	result, found := response[indexName]
	if !found {
		return nil, fmt.Errorf(`mappings result for index "%s" not found`, indexName)
	}
	indexMappings, ok := result.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf(`cannot cast indexMappings for index "%s" to map[string]interface{}`, indexName)
	}

	mappings, found := indexMappings["mappings"]
	if !found {
		return nil, fmt.Errorf(`mappings for index "%s" not found`, indexName)
	}
	m, ok := mappings.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf(`cannot cast mappings for index "%s" to map[string]interface{}`, indexName)
	}

	return m, nil
}

func settingsOf(ctx context.Context, indexName string) (map[string]interface{}, error) {
	response, err := util.GetClient7().IndexGetSettings().
		Index(indexName).
		Do(ctx)
	if err != nil {
		return nil, err
	}

	info, found := response[indexName]
	if !found {
		return nil, fmt.Errorf("settings for index %s not found", indexName)
	}

	indexSettings, ok := info.Settings["index"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("error casting index settings to map[string]interface{}")
	}
	settings := make(map[string]interface{})

	settings["index"] = make(map[string]interface{})
	settings["number_of_shards"] = indexSettings["number_of_shards"]
	settings["number_of_replicas"] = indexSettings["number_of_replicas"]
	analysis, found := info.Settings["analysis"]
	if found {
		settings["analysis"] = analysis
	}

	return settings, nil
}

func aliasesOf(ctx context.Context, indexName string) ([]string, error) {
	response, err := util.GetClient7().CatAliases().
		Pretty(true).
		Do(ctx)
	if err != nil {
		return nil, err
	}

	var aliases []string
	for _, row := range response {
		if row.Index == indexName {
			aliases = append(aliases, row.Alias)
		}
	}

	return aliases, nil
}

func createIndex(ctx context.Context, indexName string, body map[string]interface{}) error {
	response, err := util.GetClient7().CreateIndex(indexName).
		BodyJson(body).
		Do(ctx)
	if err != nil {
		return err
	}

	if !response.Acknowledged {
		return fmt.Errorf(`failed to create index named "%s", acknowledged=false`, indexName)
	}

	return nil
}

func deleteIndex(ctx context.Context, indexName string) error {
	response, err := util.GetClient7().DeleteIndex(indexName).
		Do(ctx)
	if err != nil {
		return err
	}

	if !response.Acknowledged {
		return fmt.Errorf(`error deleting index "%s", acknowledged=false`, indexName)
	}

	return nil
}

func setAlias(ctx context.Context, indexName string, aliases ...string) error {
	var addAliasActions []es7.AliasAction
	for _, alias := range aliases {
		addAliasAction := es7.NewAliasAddAction(alias).
			Index(indexName)
		addAliasActions = append(addAliasActions, addAliasAction)
	}

	response, err := util.GetClient7().Alias().
		Action(addAliasActions...).
		Do(ctx)
	if err != nil {
		return err
	}

	if !response.Acknowledged {
		return fmt.Errorf(`unable to set aliases "%v" for index "%s"`, aliases, indexName)
	}

	return nil
}

func getIndicesByAlias(ctx context.Context, alias string) ([]string, error) {
	response, err := util.GetClient7().Aliases().
		Index(alias).
		Do(ctx)
	if err != nil {
		return nil, err
	}
	return response.IndicesByAlias(alias), nil
}
