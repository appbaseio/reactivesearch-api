package util

import (
	"context"
)

// SetDefaultIndexTemplate to set default template for indexes
func SetDefaultIndexTemplate() error {
	version := GetVersion()
	if version == 7 {
		defaultSetting := `{
			"template": "*",
			"settings": {
			"number_of_shards": 1,
			"max_ngram_diff": 8,
			"max_shingle_diff": 8,
			"analysis": {
				"analyzer": {
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
				}
				},
				"filter": {
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
			},
			"mappings": {
			"dynamic_templates": [
				{
				"strings": {
					"match_mapping_type": "string",
					"mapping": {
					"type": "text",
					"fields": {
						"autosuggest": {
						"type": "text",
						"analyzer": "autosuggest_analyzer",
						"search_analyzer": "simple"
						},
						"keyword": {
						"type": "keyword",
						"ignore_above": 256
						},
						"search": {
						"type": "text",
						"analyzer": "ngram_analyzer",
						"search_analyzer": "simple"
						}
					},
					"analyzer": "standard"
					}
				}
				}
			],
			"dynamic": true
			}
		}`
		_, err := GetClient7().IndexPutTemplate("default_temp").BodyString(defaultSetting).Do(context.Background())
		if err != nil {
			return err
		}
	}
	return nil
}
