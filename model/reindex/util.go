package reindex

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"unicode"

	"github.com/appbaseio/reactivesearch-api/middleware/classify"
	"github.com/appbaseio/reactivesearch-api/util"
	log "github.com/sirupsen/logrus"
)

type ReindexConfig struct {
	Mappings                map[string]interface{}  `json:"mappings"`
	Settings                map[string]interface{}  `json:"settings"`
	SearchRelevancySettings *map[string]interface{} `json:"search_relevancy_settings"`
	Include                 []string                `json:"include_fields"`
	Exclude                 []string                `json:"exclude_fields"`
	Types                   []string                `json:"types"`
	Action                  []string                `json:"action,omitempty"`
	Script                  string                  `json:"script"`
}

// AliasedIndices struct
type AliasedIndices struct {
	Alias        string      `json:"alias"`
	Health       string      `json:"health"`
	Status       string      `json:"status"`
	Index        string      `json:"index"`
	UUID         string      `json:"uuid"`
	Pri          interface{} `json:"pri"`
	Rep          interface{} `json:"rep"`
	DocsCount    interface{} `json:"docs.count"`
	DocsDeleted  interface{} `json:"docs.deleted"`
	StoreSize    string      `json:"store.size"`
	PriStoreSize string      `json:"pri.store.size"`
}

type TaskResponseFailure struct {
	Index  string `json:"index"`
	Type   string `json:"type"`
	ID     string `json:"id"`
	Status int32  `json:"status"`
	Cause  struct {
		Type     string `json:"type"`
		Reason   string `json:"reason"`
		CausedBy struct {
			Type   string `json:"type"`
			Reason string `json:"reason"`
		} `json:"caused_by"`
	} `json:"cause"`
}

type TaskResponseStruct struct {
	Completed bool `json:"completed"`
	Response  struct {
		Failures []TaskResponseFailure `json:"failures"`
	} `json:"response"`
}

// CurrentlyReIndexingProcess map of  taskID [source, destinations] indexes for which indexing process is going on
var CurrentlyReIndexingProcess = make(map[string][]string)

// CurrentlyReIndexingProcessMutex to stop concurrent writes on map
var CurrentlyReIndexingProcessMutex = sync.RWMutex{}

// IndexStoreSize to decide whether to use async or sync re-indexing
const IndexStoreSize = int64(100000000)

// ParseStoreSize converts Elasticsearch _cat/indices size strings (e.g. "1.2kb",
// "100mb") or plain byte counts to bytes. A dash or empty value is treated as zero.
func ParseStoreSize(size string) (int64, error) {
	size = strings.TrimSpace(strings.ToLower(size))
	if size == "" || size == "-" {
		return 0, nil
	}

	multiplier := int64(1)
	units := []struct {
		suffix string
		mult   int64
	}{
		{"pb", 1024 * 1024 * 1024 * 1024 * 1024},
		{"tb", 1024 * 1024 * 1024 * 1024},
		{"gb", 1024 * 1024 * 1024},
		{"mb", 1024 * 1024},
		{"kb", 1024},
		{"b", 1},
	}
	for _, unit := range units {
		if strings.HasSuffix(size, unit.suffix) {
			size = strings.TrimSuffix(size, unit.suffix)
			multiplier = unit.mult
			break
		}
	}

	size = strings.TrimSpace(size)
	if size == "" {
		return 0, fmt.Errorf("invalid store size")
	}

	value, err := strconv.ParseFloat(size, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid store size %q: %w", size, err)
	}
	if value < 0 {
		return 0, fmt.Errorf("invalid store size %q", size)
	}

	return int64(value * float64(multiplier)), nil
}

func isNumericStoreSize(size string) bool {
	if size == "" {
		return false
	}
	for _, r := range size {
		if !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}

// reindexedName calculates from the name the number of times an index has been
// reindexed to generate the successive name for the index. For example: for an
// index named "twitter", the funtion returns "twitter_reindexed_1", and for an
// index named "foo_reindexed_3", the function returns "foo_reindexed_4". The
// basic check here is to check if the index name ends with a suffix "reindexed_{x}",
// and if it doesn't the function assumes the index has never been reindexed.
func reindexedName(indexName string) (string, error) {
	const pattern = `.*reindexed_[0-9]+`
	matched, err := regexp.MatchString(pattern, indexName)
	if err != nil {
		log.Errorln(logTag, ":", err)
		return "", err
	}

	if matched {
		tokens := strings.Split(indexName, "_")
		size := len(tokens)
		number, err := strconv.Atoi(tokens[size-1])
		if err != nil {
			return "", err
		}
		tokens[size-1] = fmt.Sprintf("%d", number+1)
		indexName = strings.Join(tokens, "_")
	} else {
		indexName += "_reindexed_1"
	}

	return indexName, nil
}

// InitIndexAliasCache to set cache on ReactiveSearch initialization
func InitIndexAliasCache() {
	ctx := context.Background()
	indexAlias, _ := GetAliasedIndices(ctx)

	for _, aliasIndex := range indexAlias {
		if aliasIndex.Alias != "" {
			classify.SetIndexAlias(aliasIndex.Index, aliasIndex.Alias)
		}
	}
	log.Debugln(logTag, "=> Index Alias Cache", classify.GetIndexAliasCache())
}

// InitAliasIndexCache to set alias -> index cache on initialization
func InitAliasIndexCache() {
	ctx := context.Background()
	aliasIndexMap, _ := GetAliasIndexMap(ctx)
	classify.SetAliasIndexCache(aliasIndexMap)
	log.Debugln(logTag, "=> Alias Index Cache", classify.GetAliasIndexCache())
}

// SetCurrentProcess set indexes for current reindexing process
func SetCurrentProcess(taskID, source, destination string) {
	CurrentlyReIndexingProcessMutex.Lock()
	CurrentlyReIndexingProcess[taskID] = []string{source, destination}
	CurrentlyReIndexingProcessMutex.Unlock()
}

// RemoveCurrentProcess remove indexes for current reindexing process
func RemoveCurrentProcess(taskID string) {
	CurrentlyReIndexingProcessMutex.Lock()
	delete(CurrentlyReIndexingProcess, taskID)
	CurrentlyReIndexingProcessMutex.Unlock()
}

// IsReIndexInProcess check if index is Processing currently
func IsReIndexInProcess(source, destination string) bool {
	for _, processingIndexes := range CurrentlyReIndexingProcess {
		if processingIndexes[0] == source || processingIndexes[0] == destination {
			return true
		}
		if processingIndexes[1] == source || processingIndexes[1] == destination {
			return true
		}
	}

	return false
}

// Returns the index name for search relevancy
func getSearchRelevancyIndex() string {
	searchRelevancyIndex := os.Getenv("SEARCH_RELEVANCY_ES_INDEX")
	if searchRelevancyIndex == "" {
		searchRelevancyIndex = ".searchrelevancy"
	}
	return util.MetaIndexName(searchRelevancyIndex)
}

// Returns the index name for synonyms
func getSynonymsIndex() string {
	synonymsIndex := os.Getenv("SYNONYMS_ES_INDEX")
	if synonymsIndex == "" {
		synonymsIndex = ".rs-synonyms"
	}
	return util.MetaIndexName(synonymsIndex)
}
