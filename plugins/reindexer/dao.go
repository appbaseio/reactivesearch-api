package reindexer

import (
	"context"
	"fmt"
	"log"

	"github.com/appbaseio-confidential/arc/util"
	"github.com/olivere/elastic"
)

type elasticsearch struct {
	url    string
	client *elastic.Client
}

func newClient(url string) (*elasticsearch, error) {
	client, err := elastic.NewClient(
		elastic.SetURL(url),
		elastic.SetRetrier(util.NewRetrier()),
	)
	if err != nil {
		return nil, fmt.Errorf("%s: error while initializing elastic client: %v", logTag, err)
	}
	es := &elasticsearch{url, client}

	return es, nil
}

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
func (es *elasticsearch) reindex(ctx context.Context, indexName string, mappings, settings map[string]interface{}, includes, excludes, types []string) error {
	var err error

	// We fetch the index name pointing to the given alias first.
	// If an index has already been reindexed before, user would
	// pass in the alias i.e. the original name of the index when
	// it was first created. We try to fetch the current index name
	// from the given alias. If alias name doesn't exist we get an
	// empty slice of indices, which means the index has never been
	// reindexed before.
	indices, err := es.getIndicesByAlias(ctx, indexName)
	if err != nil {
		log.Println(err)
	}
	if len(indices) > 1 {
		return fmt.Errorf(`multiple indices pointing to alias "%s"`, indexName)
	}
	if len(indices) == 1 {
		indexName = indices[0]
	}

	// If mappings are not passed, we fetch the mappings of the old index.
	if mappings == nil {
		mappings, err = es.mappingsOf(ctx, indexName)
		if err != nil {
			return fmt.Errorf(`error fetching mappings of index "%s": %v`, indexName, err)
		}
	}

	// If settings are not passed, we fetch the settings of the old index.
	if settings == nil {
		settings, err = es.settingsOf(ctx, indexName)
		if err != nil {
			return fmt.Errorf(`error fetching settings of index "%s": %v`, indexName, err)
		}
	}

	// Setup the destination index prior to running the _reindex action.
	body := make(map[string]interface{})
	body["mappings"] = mappings
	body["settings"] = settings

	newIndexName, err := reindexedName(indexName)
	if err != nil {
		return fmt.Errorf(`error generating a new index name for index "%s": %v`, indexName, err)
	}

	err = es.createIndex(ctx, newIndexName, body)
	if err != nil {
		return err
	}

	// _source filtering to include/exclude fields while reindexing.
	ctxSource := make(map[string][]string)
	if includes != nil && len(includes) > 0 {
		ctxSource["includes"] = includes
	}
	if excludes != nil && len(excludes) > 0 {
		ctxSource["excludes"] = excludes
	}

	// Configure reindex source.
	reindexSource := make(map[string]interface{})
	reindexSource["index"] = indexName
	if types != nil && len(types) > 0 {
		reindexSource["types"] = types
	}
	if len(ctxSource) > 0 {
		reindexSource["_source"] = ctxSource
	}

	// Configure reindex destination.
	reindexDest := make(map[string]interface{})
	reindexDest["index"] = newIndexName

	// Configure reindex body.
	reindexBody := make(map[string]interface{})
	reindexBody["source"] = reindexSource
	reindexBody["dest"] = reindexDest

	// Reindex action.
	_, err = es.client.Reindex().
		Body(reindexBody).
		Do(ctx)
	if err != nil {
		return fmt.Errorf(`error reindexing index "%s": %v`, indexName, err)
	}

	// Fetch all the aliases of old index
	aliases, err := es.aliasesOf(ctx, indexName)
	if err != nil {
		return err
	}
	aliases = append(aliases, indexName)

	// Delete old index
	err = es.deleteIndex(ctx, indexName)
	if err != nil {
		return fmt.Errorf(`error deleting index "%s": %v\n`, indexName, err)
	}

	// Set aliases of old index to the new index.
	err = es.setAlias(ctx, newIndexName, aliases...)
	if err != nil {
		return fmt.Errorf(`error setting alias "%s" for index "%s"`, indexName, newIndexName)
	}

	return nil
}

func (es *elasticsearch) mappingsOf(ctx context.Context, indexName string) (map[string]interface{}, error) {
	response, err := es.client.GetMapping().
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

func (es *elasticsearch) settingsOf(ctx context.Context, indexName string) (map[string]interface{}, error) {
	response, err := es.client.IndexGetSettings().
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

func (es *elasticsearch) aliasesOf(ctx context.Context, indexName string) ([]string, error) {
	response, err := es.client.CatAliases().
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

func (es *elasticsearch) createIndex(ctx context.Context, indexName string, body map[string]interface{}) error {
	response, err := es.client.CreateIndex(indexName).
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

func (es *elasticsearch) deleteIndex(ctx context.Context, indexName string) error {
	response, err := es.client.DeleteIndex(indexName).
		Do(ctx)
	if err != nil {
		return err
	}

	if !response.Acknowledged {
		return fmt.Errorf(`error deleting index "%s", acknowledged=false`, indexName)
	}

	return nil
}

func (es *elasticsearch) setAlias(ctx context.Context, indexName string, aliases ...string) error {
	var addAliasActions []elastic.AliasAction
	for _, alias := range aliases {
		addAliasAction := elastic.NewAliasAddAction(alias).
			Index(indexName)
		addAliasActions = append(addAliasActions, addAliasAction)
	}

	response, err := es.client.Alias().
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

func (es *elasticsearch) getIndicesByAlias(ctx context.Context, alias string) ([]string, error) {
	response, err := es.client.Aliases().
		Index(alias).
		Do(ctx)
	if err != nil {
		return nil, err
	}
	return response.IndicesByAlias(alias), nil
}
