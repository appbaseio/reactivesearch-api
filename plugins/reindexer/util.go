package reindexer

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	log "github.com/sirupsen/logrus"
)

// AliasedIndices struct
type AliasedIndices struct {
	Alias        string `json:"alias"`
	Health       string `json:"health"`
	Status       string `json:"status"`
	Index        string `json:"index"`
	UUID         string `json:"uuid"`
	Pri          int    `json:"pri"`
	Rep          int    `json:"rep"`
	DocsCount    int    `json:"docs.count"`
	DocsDeleted  int    `json:"docs.deleted"`
	StoreSize    string `json:"store.size"`
	PriStoreSize string `json:"pri.store.size"`
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
