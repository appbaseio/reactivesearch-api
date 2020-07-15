package reindexer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/appbaseio/arc/middleware/classify"
	"github.com/appbaseio/arc/util"
	es7 "github.com/olivere/elastic/v7"
)

func postReIndex(ctx context.Context, sourceIndex, newIndexName string, operation ReIndexOperation) error {
	// Fetch all the aliases of old index
	alias, err := aliasesOf(ctx, sourceIndex)

	var aliases = []string{}
	if err != nil {
		return errors.New(`error fetching aliases of index ` + sourceIndex + "\n" + err.Error())
	}

	if alias == "" {
		aliases = append(aliases, sourceIndex)
	} else {
		aliases = append(aliases, alias)
	}

	// Delete old index
	if operation == ReIndexWithDelete {
		err = deleteIndex(ctx, sourceIndex)
		if err != nil {
			return errors.New(`error deleting source index ` + sourceIndex + "\n" + err.Error())
		}
		// Set aliases of old index to the new index.
		err = setAlias(ctx, newIndexName, aliases...)
		if err != nil {
			return errors.New(`error setting alias for ` + newIndexName + "\n" + err.Error())
		}
	}

	// _, err = util.GetClient7().IndexPutSettings(newIndexName).BodyString(fmt.Sprintf(`{"index.number_of_replicas": %d}`, util.GetReplicas())).Do(ctx)
	// if err != nil {
	// 	return err
	// }
	return nil
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
//
// We accept a query param `wait_for_completion` which defaults to true, which when false, we don't create any aliases
// and delete the old index, we instead return the tasks API response.
func reindex(ctx context.Context, sourceIndex string, config *reindexConfig, waitForCompletion bool, destinationIndex string) ([]byte, error) {
	var err error

	// We fetch the index name pointing to the given alias first.
	// If an index has already been reindexed before, user would
	// pass in the alias i.e. the original name of the index when
	// it was first created. We try to fetch the current index name
	// from the given alias. If alias name doesn't exist we get an
	// empty slice of indices, which means the index has never been
	// reindexed before.
	indices, err := getIndicesByAlias(ctx, sourceIndex)
	if err != nil {
		log.Errorln(err)
	}
	if len(indices) > 1 {
		return nil, fmt.Errorf(`multiple indices pointing to alias "%s"`, sourceIndex)
	}
	if len(indices) == 1 {
		sourceIndex = indices[0]
	}

	// If mappings are not passed, we fetch the mappings of the old index.
	if config.Mappings == nil {
		found := util.IsExists(Mappings.String(), config.Action)
		if len(config.Action) == 0 || found {
			config.Mappings, err = mappingsOf(ctx, sourceIndex)
			if err != nil {
				return nil, fmt.Errorf(`error fetching mappings of index "%s": %v`, sourceIndex, err)
			}
		}
	}

	// If settings are not passed, we fetch the settings of the old index.
	if config.Settings == nil {
		found := util.IsExists(Settings.String(), config.Action)
		if len(config.Action) == 0 || found {
			config.Settings, err = settingsOf(ctx, sourceIndex)
			if err != nil {
				return nil, fmt.Errorf(`error fetching settings of index "%s": %v`, sourceIndex, err)
			}
		}
	} else {
		// set number of replicas to 0 while reindexing
		indexSettingsAsMap, ok := config.Settings["index"].(map[string]interface{})
		fmt.Println("=> settings map", indexSettingsAsMap)
		if ok {
			indexSettingsAsMap["number_of_replicas"] = 0
			indexSettingsAsMap["auto_expand_replicas"] = false
		}

		config.Settings["index"] = indexSettingsAsMap
		fmt.Println("=> final settings", config.Settings)
	}

	// Setup the destination index prior to running the _reindex action.
	body := make(map[string]interface{})
	if config.Mappings != nil {
		body["mappings"] = config.Mappings
	}
	if config.Settings != nil {
		body["settings"] = config.Settings
	}
	newIndexName := destinationIndex
	operation := ReIndexWithDelete
	if destinationIndex != "" {
		operation = ReindexWithClone
	}
	if operation == ReIndexWithDelete {
		newIndexName, err = reindexedName(sourceIndex)
	}

	if err != nil {
		return nil, fmt.Errorf(`error generating a new index name for index "%s": %v`, sourceIndex, err)
	}

	// Create the new index.
	err = createIndex(ctx, newIndexName, body)
	if err != nil {
		return nil, err
	}

	/* Copy search relevancy settings if
	- `search_relevancy_settings` object is present
	- and action array has the `search_relevancy` action defined
	*/
	if config.SearchRelevancySettings != nil && util.IsExists(SearchRelevancy.String(), config.Action) {
		// Index a document in .searchrelevancy index for the destination `index`
		err := putSearchRelevancySettings(ctx, newIndexName, *config.SearchRelevancySettings)
		if err != nil {
			return nil, fmt.Errorf(`error while copying search relevancy settings: %v`, err)
		}
	}

	/* Copy Synonyms if `synonyms` action is set in the action array
	 */
	if util.IsExists(Synonyms.String(), config.Action) {
		// Update synonyms by query
		err := updateSynonyms(ctx, sourceIndex, newIndexName)
		if err != nil {
			return nil, fmt.Errorf(`error while updating the synonyms: %v`, err)
		}
	}

	found := util.IsExists(Data.String(), config.Action)

	// do not copy data
	if !(len(config.Action) == 0 || found) {
		return json.Marshal(make(map[string]interface{}))
	}

	// Configure reindex source
	src := es7.NewReindexSource().
		Index(sourceIndex).
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

	if waitForCompletion {
		response, err := reindex.Do(ctx)
		if err != nil {
			return nil, err
		}

		if operation == ReIndexWithDelete {
			err = postReIndex(ctx, sourceIndex, newIndexName, ReIndexWithDelete)
			if err != nil {
				log.Errorln(logTag, " post re-indexing error: ", err)
				return nil, err
			}
		}

		return json.Marshal(response)
	}
	// If wait_for_completion = false, we carry out the re-indexing asynchronously and return the task ID.
	log.Println(logTag, fmt.Sprintf(" Data is > %d so using async reindex", IndexStoreSize))
	response, err := reindex.DoAsync(context.Background())
	if err != nil {
		return nil, err
	}
	taskID := response.TaskId

	go asyncReIndex(taskID, sourceIndex, newIndexName, operation)

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
	settings["index.number_of_shards"] = indexSettings["number_of_shards"]
	settings["index.number_of_replicas"] = 0
	settings["index.auto_expand_replicas"] = false
	analysis, found := indexSettings["analysis"]
	if found {
		settings["analysis"] = analysis
	}

	return settings, nil
}

func aliasesOf(ctx context.Context, indexName string) (string, error) {
	response, err := util.GetClient7().CatAliases().
		Pretty(true).
		Do(ctx)
	if err != nil {
		return "", err
	}

	var alias = ""

	// set alias for original index name only.
	regex := ".*reindexed_[0-9]+"
	r, _ := regexp.Compile(regex)

	for _, row := range response {
		// r.MatchString(indexName) this condition is added to handle existing alias which are created incorrectly
		if row.Index == indexName && r.MatchString(indexName) {
			alias = row.Alias
		}
	}

	return alias, nil
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

	// We only have one alias per index.
	classify.SetIndexAlias(indexName, aliases[0])
	classify.SetAliasIndex(aliases[0], indexName)
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

func getAliasedIndices(ctx context.Context) ([]AliasedIndices, error) {
	var indicesList []AliasedIndices
	v := url.Values{}
	v.Set("format", "json")

	if util.GetSemanticVersion() >= "7.7.0" {
		v.Add("expand_wildcards", "all")
	}

	requestOptions := es7.PerformRequestOptions{
		Method: "GET",
		Path:   "/_cat/indices",
		Params: v,
	}
	response, err := util.GetClient7().PerformRequest(ctx, requestOptions)
	if err != nil {
		return indicesList, err
	}

	if response.StatusCode > 300 {
		return indicesList, errors.New(string(response.Body))
	}

	err = json.Unmarshal(response.Body, &indicesList)
	if err != nil {
		return indicesList, err
	}

	aliases, err := util.GetClient7().CatAliases().
		Pretty(true).
		Do(ctx)
	if err != nil {
		return indicesList, err
	}

	for i, index := range indicesList {
		// oliver PerformRequest gives this values as string, but Frontend will need them as integers
		indicesList[i].Pri, _ = strconv.Atoi(fmt.Sprintf("%v", index.Pri))
		indicesList[i].Rep, _ = strconv.Atoi(fmt.Sprintf("%v", index.Rep))
		indicesList[i].DocsCount, _ = strconv.Atoi(fmt.Sprintf("%v", index.DocsCount))
		indicesList[i].DocsDeleted, _ = strconv.Atoi(fmt.Sprintf("%v", index.DocsDeleted))
		var alias string
		regex := ".*reindexed_[0-9]+"
		rolloverPatter := ".*-[0-9]+"
		rolloverRegex, _ := regexp.Compile(rolloverPatter)
		indexRegex, _ := regexp.Compile(regex)

		for _, row := range aliases {
			// match the alias for rollover index
			if row.Index[:1] == "." && row.Index == index.Index && rolloverRegex.MatchString(index.Index) {
				alias = row.Alias
				break
			} else if row.Index == index.Index && indexRegex.MatchString(index.Index) {
				alias = row.Alias
				break
			}

		}
		if err == nil && alias != "" {
			indicesList[i].Alias = alias
		}

	}

	return indicesList, nil
}

func getAliasIndexMap(ctx context.Context) (map[string]string, error) {
	var res = make(map[string]string)
	aliases, err := util.GetClient7().CatAliases().
		Pretty(true).
		Do(ctx)
	if err != nil {
		return res, err
	}

	for _, alias := range aliases {
		res[alias.Alias] = alias.Index
	}

	return res, nil
}

func getIndexSize(ctx context.Context, indexName string) (int64, error) {
	var res int64
	index := indexName
	aliasesIndexMap, err := getAliasIndexMap(ctx)
	if err != nil {
		return res, err
	}
	if indexNameFromMap, ok := aliasesIndexMap[indexName]; ok {
		index = indexNameFromMap
	}

	stats, err := util.GetClient7().IndexStats(indexName).Do(ctx)
	if err != nil {
		return res, err
	}

	if val, ok := stats.Indices[index]; ok {
		res = val.Primaries.Store.SizeInBytes
		return res, nil
	}

	return res, errors.New(`Invalid index name`)
}

func isTaskCompleted(ctx context.Context, taskID string) (bool, error) {
	res := false

	status, err := util.GetClient7().TasksGetTask().TaskId(taskID).Do(ctx)
	if err != nil {
		log.Errorln(logTag, " Get task status error", err)
		return res, err
	}

	res = status.Completed
	return res, nil
}

// go routine to track async re-indexing process for a given source and destination index.
// it checks every 30s if task is completed or not.
func asyncReIndex(taskID, source, destination string, operation ReIndexOperation) {
	SetCurrentProcess(taskID, source, destination)
	isCompleted := make(chan bool, 1)
	ticker := time.Tick(30 * time.Second)
	ctx := context.Background()

	for {
		select {
		case <-ticker:
			ok, _ := isTaskCompleted(ctx, taskID)
			log.Println(logTag, " "+taskID+" task is still re-indexing data...")
			if ok {
				isCompleted <- true
			}
		case <-isCompleted:
			log.Println(logTag, taskID+" task completed successfully")
			// remove process from current cache
			RemoveCurrentProcess(taskID)
			err := postReIndex(ctx, source, destination, operation)
			if err != nil {
				log.Errorln(logTag, " post re-indexing error: ", err)
			}
			return
		}
	}
}

func putSearchRelevancySettings(ctx context.Context, docID string, record map[string]interface{}) error {
	_, err := util.GetClient7().
		Index().
		Refresh("wait_for").
		Index(getSearchRelevancyIndex()).
		BodyJson(record).
		Id(docID).
		Do(ctx)
	if err != nil {
		log.Errorln(logTag, ": error indexing searchrelevancy record for id=", docID, ":", err)
		return err
	}
	return nil
}

func updateSynonyms(ctx context.Context, sourceIndex string, destinationIndex string) error {
	script := `
		if(ctx._source.index == null) { 
			ctx._source.index = [] 
		} 
		if(ctx._source.index instanceof String) { 
			ctx._source.index = [ctx._source.index] 
		} 
		if (params.index != null) { 
			if (ctx._source.index.indexOf(params.index) == -1) { 
				ctx._source.index.add(params.index) 
			}
		}`
	params := map[string]interface{}{
		"index": destinationIndex,
	}
	switch util.GetVersion() {
	case 6:
		return updateSynonymsEs6(ctx, script, sourceIndex, params)
	default:
		return updateSynonymsEs7(ctx, script, sourceIndex, params)
	}
}
