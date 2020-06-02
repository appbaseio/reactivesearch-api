package util

import (
	"context"
	"fmt"

	log "github.com/sirupsen/logrus"
)

// SetDefaultIndexTemplate to set default template for indexes
func SetDefaultIndexTemplate() error {
	replicas := GetReplicas()
	settings := fmt.Sprintf(`{
		"index.number_of_shards": 1,
		"max_ngram_diff": 8,
		"max_shingle_diff": 8,
		"index.number_of_replicas": %d,
		"analysis": {
			"analyzer": {
				"universal": {
					"tokenizer": "standard",
					"filter": [
						"universal_stop"
					]
				},
				"autosuggest_analyzer": {
					"filter": [
						"lowercase",
						"asciifolding",
						"autosuggest_filter"
					],
					"tokenizer": "standard",
					"type": "custom"
				},
				"ngram_analyzer": {
					"filter": [
						"lowercase",
						"asciifolding",
						"ngram_filter"
					],
					"tokenizer": "standard",
					"type": "custom"
				},
				"synonyms": {
					"tokenizer": "standard",
					"filter": [
						"synonym_graph",
						"lowercase"
					]
				}
			},
			"filter": {
				"synonym_graph": {
					"type": "synonym_graph",
					"synonyms": []
				},
				"universal_stop": {
					"type": "stop",
					"stopwords": "_english_"
				},
				"autosuggest_filter": {
					"max_gram": "20",
					"min_gram": "1",
					"token_chars": [
						"letter",
						"digit",
						"punctuation",
						"symbol"
					],
					"type": "edge_ngram"
				},
				"ngram_filter": {
					"max_gram": "9",
					"min_gram": "2",
					"token_chars": [
						"letter",
						"digit",
						"punctuation",
						"symbol"
					],
					"type": "ngram"
				}
			}
		} 
	}`, replicas)

	mappings := `{
		"dynamic_templates": [{
			"strings": {
				"match_mapping_type": "string",
				"mapping": {
					"type": "text",
					"analyzer": "standard",
					"fields": {
						"autosuggest": {
							"type": "text",
							"analyzer": "autosuggest_analyzer",
							"search_analyzer": "standard"
						},
						"keyword": {
							"type": "keyword",
							"ignore_above": 256
						},
						"search": {
							"type": "text",
							"analyzer": "ngram_analyzer",
							"search_analyzer": "standard"
						},
						"synonyms": {
							"type": "text",
							"analyzer": "synonyms"
						},
						"lang": {
							"type": "text",
							"analyzer": "universal"
						}
					}
				}
			}
		}],
		"dynamic": true
	}`

	version := GetVersion()
	if version == 7 {
		defaultSetting := fmt.Sprintf(`{
			"index_patterns": ["*"],
			"settings": %s,
			"mappings": %s
		}`, settings, mappings)
		_, err := GetClient7().IndexPutTemplate("default_temp").BodyString(defaultSetting).Do(context.Background())
		if err != nil {
			log.Errorln("[SET TEMPLATE ERROR V7]", ": ", err)
			return err
		}
	}

	if version == 6 {
		defaultSetting := fmt.Sprintf(`{
			"index_patterns": ["*"],
			"settings": %s,
			"mappings": {
				"_doc": %s
			}
		}`, settings, mappings)
		_, err := GetClient6().IndexPutTemplate("default_temp").BodyString(defaultSetting).Do(context.Background())
		if err != nil {
			log.Errorln("[SET TEMPLATE ERROR V6]", ": ", err)
			return err
		}
	}
	return nil
}
