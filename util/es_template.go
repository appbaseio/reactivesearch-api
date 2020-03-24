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
						"synonym": {
							"tokenizer": "standard",
							"filter": [
								"synonym_graph"
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
			},
			"mappings": {
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
								},
								"synonym": {
									"type": "text",
									"analyzer": "synonym"
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
			}
		}`
		_, err := GetClient7().IndexPutTemplate("default_temp").BodyString(defaultSetting).Do(context.Background())
		if err != nil {
			return err
		}
	}
	return nil
}
