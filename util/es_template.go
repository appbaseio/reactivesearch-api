package util

import (
	"context"
	"fmt"

	log "github.com/sirupsen/logrus"
)

// SetDefaultIndexTemplate sets default template for user indexes
func SetDefaultIndexTemplate() error {
	replicas := GetReplicas()
	settings := fmt.Sprintf(`{
		"index.number_of_shards": 1,
		"index.number_of_replicas": %d,
		"index.auto_expand_replicas": "0-1",
		"index.max_ngram_diff": 8,
		"index.max_shingle_diff": 8,
		"index.mapping": {
			"total_fields": {
				"limit": "10000"
			}
		},
		"index.analysis": {
			"analyzer": {
				"universal": {
					"filter": [
						"cjk_width",
						"lowercase",
						"asciifolding",
						"universal_stop"
					],
					"tokenizer": "standard"
				},
				"universal_delimiter_analyzer": {
					"filter": [
						"delimiter",
						"flatten_graph",
						"cjk_width",
						"lowercase",
						"asciifolding",
						"universal_stop",
						"stemmer"
					],
					"tokenizer": "whitespace"
				},
				"autosuggest_analyzer": {
					"filter": [
						"cjk_width",
						"lowercase",
						"asciifolding"
					],
					"tokenizer": "autosuggest_tokenizer"
				},
				"ngram_analyzer": {
					"filter": [
						"lowercase",
						"asciifolding",
						"ngram_filter"
					],
					"tokenizer": "standard"
				},
				"synonyms": {
					"filter": [
						"synonym_graph",
						"lowercase"
					],
					"tokenizer": "standard"
				},
				"ngram_search_analyzer": {
					"filter": [
						"cjk_width",
						"lowercase",
						"asciifolding"
					],
					"tokenizer": "standard"
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
				"ngram_filter": {
					"max_gram": "7",
					"min_gram": "3",
					"type": "ngram"
				},
				"delimiter": {
					"catenate_all": "true",
					"catenate_numbers": "true",
					"catenate_words": "true",
					"split_on_numerics": "true",
					"generate_word_parts": "true",
					"generate_number_parts": "true",
					"preserve_original": "false",
					"split_on_case_change": "true",
					"stem_english_possessive": "true",
					"type": "word_delimiter_graph"
				}
			},
			"tokenizer": {
				"autosuggest_tokenizer": {
					"type": "edge_ngram",
					"min_gram": 1,
					"max_gram": 20,
					"token_chars": [
						"letter",
						"digit",
						"punctuation",
						"symbol",
						"whitespace"
					]
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
					"index_phrases": true,
					"index_prefixes": {
						"min_chars": 1,
						"max_chars": 12
					},
					"fields": {
						"autosuggest": {
							"type": "text",
							"analyzer": "autosuggest_analyzer",
							"search_analyzer": "ngram_search_analyzer"
						},
						"keyword": {
							"type": "keyword",
							"ignore_above": 256
						},
						"search": {
							"type": "text",
							"analyzer": "ngram_analyzer",
							"search_analyzer": "ngram_search_analyzer"
						},
						"synonyms": {
							"type": "text",
							"analyzer": "synonyms"
						},
						"lang": {
							"type": "text",
							"analyzer": "universal",
							"index_options": "offsets"
						},
						"delimiter": {
							"type": "text",
							"analyzer": "universal_delimiter_analyzer",
							"index_options": "offsets"
						}
					}
				}
			}
		},
		{
			"double": {
				"match_mapping_type": "double",
				"mapping": {
					"type": "keyword"
				}
			}
		},
		{
			"long": {
				"match_mapping_type": "long",
				"mapping": {
					"type": "keyword"
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
			"mappings": %s,
			"order": 10
		}`, settings, mappings)
		_, err := GetClient7().IndexPutTemplate("arc_index_template_v1").BodyString(defaultSetting).Do(context.Background())
		if err != nil {
			log.Errorln("[SET USER INDEX TEMPLATE ERROR V7]", ": ", err)
			return err
		}
	}

	if version == 6 {
		defaultSetting := fmt.Sprintf(`{
			"index_patterns": ["*"],
			"settings": %s,
			"mappings": {
				"_doc": %s
			},
			"order": 10
		}`, settings, mappings)
		_, err := GetClient6().IndexPutTemplate("arc_index_template_v1").BodyString(defaultSetting).Do(context.Background())
		if err != nil {
			log.Errorln("[SET USER INDEX TEMPLATE ERROR V6]", ": ", err)
			return err
		}
	}
	return nil
}

// SetSystemIndexTemplate sets default template for system indexes
func SetSystemIndexTemplate() error {
	replicas := GetReplicas()
	settings := fmt.Sprintf(`{
		"index.number_of_shards": 1,
		"index.number_of_replicas": %d,
		"index.auto_expand_replicas": "0-1",
		"index.mapping": {
			"total_fields": {
				"limit": "10000"
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
						"keyword": {
							"type": "keyword",
							"ignore_above": 256
						}
					}
				}
			}
		},
		{
			"double": {
				"match_mapping_type": "double",
				"mapping": {
					"type": "double"
				}
			}
		},
		{
			"long": {
				"match_mapping_type": "long",
				"mapping": {
					"type": "long"
				}
			}
		}],
		"dynamic": true
	}`

	version := GetVersion()
	if version == 7 {
		defaultSetting := fmt.Sprintf(`{
			"index_patterns": [".*"],
			"settings": %s,
			"mappings": %s,
			"order": 20
		}`, settings, mappings)
		_, err := GetClient7().IndexPutTemplate("system_index_template_v1").BodyString(defaultSetting).Do(context.Background())
		if err != nil {
			log.Errorln("[SET SYSTEM INDEX TEMPLATE ERROR V7]", ": ", err)
			return err
		}
	}

	if version == 6 {
		defaultSetting := fmt.Sprintf(`{
			"index_patterns": [".*"],
			"settings": %s,
			"mappings": {
				"_doc": %s
			},
			"order": 10
		}`, settings, mappings)
		_, err := GetClient6().IndexPutTemplate("system_index_template_v1").BodyString(defaultSetting).Do(context.Background())
		if err != nil {
			log.Errorln("[SET SYSTEM INDEX TEMPLATE ERROR V6]", ": ", err)
			return err
		}
	}
	return nil
}
