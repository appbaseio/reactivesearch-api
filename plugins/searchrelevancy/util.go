package searchrelevancy

import (
	"errors"
	"sync"

	"github.com/appbaseio/reactivesearch-api/plugins/querytranslate"
)

// SearchStruct search settings struct
type SearchStruct struct {
	DataField       []string                                `json:"dataField"`
	FieldWeights    []interface{}                           `json:"fieldWeights"` // allow string array with single decimal place value to fix [mapper [search.fieldWeights] cannot be changed from type [float] to [long] [type=illegal_argument_exception]] eg. "1.0"
	SearchOperators bool                                    `json:"searchOperators"`
	QueryString     bool                                    `json:"queryString"`
	Fuzziness       interface{}                             `json:"fuzziness,omitempty"` // string or int
	QueryFormat     *string                                 `json:"queryFormat"`
	RankFeature     *map[string]querytranslate.RankFunction `json:"rankFeature,omitempty"`
	DistinctField   string                                  `json:"distinctField,omitempty"`
}

// AggregationStruct aggregation settings struct
type AggregationStruct struct {
	DataField         map[string]string      `json:"dataField"`
	Size              int                    `json:"size" validate:"gte=0,lte=1000"`
	SortBy            *querytranslate.SortBy `json:"sortBy"`
	IncludeNullValues bool                   `json:"includeNullValues"`
	QueryFormat       *string                `json:"queryFormat"`
}

// ResultStruct result settings struct
type ResultStruct struct {
	Size             int                    `json:"size" validate:"gte=0,lte=1000"`
	IncludeFields    []string               `json:"includeFields"`
	ExcludeFields    []string               `json:"excludeFields"`
	Highlight        bool                   `json:"highlight"`
	HighlightFields  []string               `json:"highlightFields"`
	HighlightOptions map[string]interface{} `json:"highlightOptions"`
	SortOptions      []SortOption           `json:"sortOptions,omitempty"`
}

type SortOption struct {
	Label     string `json:"label"`
	DataField string `json:"dataField"`
	SortBy    string `json:"sortBy"`
}

// LanguageStruct language settings struct
type LanguageStruct struct {
	Language            string   `json:"language"`
	ApplyStopwords      bool     `json:"applyStopwords"`
	CustomStopwords     []string `json:"customStopwords"`
	StemmingExceptions  []string `json:"stemmingExceptions"`
	NormalizeDiacritics bool     `json:"normalizeDiacritics"`
}

// SynonymsStruct synonym settings struct
type SynonymsStruct struct {
	Enabled bool `json:"enabled"`
}

// RulesStruct to identify if rules are query enabled or not
type RulesStruct struct {
	Enabled bool `json:"enabled"`
}

// IndexSettingStruct to store index related settings
type IndexSettingStruct struct {
	EnableNgram            bool          `json:"enableNgram,omitempty"`
	EnableAutosuggestion   bool          `json:"enableAutoSuggestion,omitempty"`
	NgramSettings          NgramSettings `json:"ngramSettings,omitempty"`
	AutosuggestionSettings NgramSettings `json:"autosuggestionSettings,omitempty"`
}

type NgramSettings struct {
	MinGram int `json:"min_gram,omitempty"`
	MaxGram int `json:"max_gram,omitempty"`
}

// SearchRelevancyStruct struct for settings request
type SearchRelevancyStruct struct {
	Search        *SearchStruct       `json:"search"`
	Aggregations  *AggregationStruct  `json:"aggregations"`
	Results       *ResultStruct       `json:"results"`
	Language      *LanguageStruct     `json:"language"`
	Synonyms      *SynonymsStruct     `json:"synonyms"`
	Rules         *RulesStruct        `json:"rules"`
	IndexSettings *IndexSettingStruct `json:"indexSettings"`
}

// SearchRelevancySettingsCache variable to cache settings
var SearchRelevancySettingsCache = map[string]SearchRelevancyStruct{}

// SearchRelevancyCacheMutex to handle concurrent map writes
var SearchRelevancyCacheMutex = sync.RWMutex{}

func getDefaultRelevancySettings() SearchRelevancyStruct {
	var fuzziness interface{} = 0
	preTags := []string{"<mark>"}
	postTags := []string{"</mark>"}
	highlightOptions := map[string]interface{}{
		"number_of_fragments": 5,
		"fragment_size":       100,
		"pre_tags":            preTags,
		"post_tags":           postTags,
	}
	sortBy := querytranslate.Count
	queryFormat := querytranslate.Or.String()

	defaultSetting := SearchRelevancyStruct{
		Search: &SearchStruct{
			DataField:       []string{},
			FieldWeights:    make([]interface{}, 0),
			SearchOperators: false,
			QueryString:     false,
			Fuzziness:       fuzziness,
			QueryFormat:     &queryFormat,
		},
		Aggregations: &AggregationStruct{
			DataField:         map[string]string{},
			Size:              10,
			SortBy:            &sortBy,
			IncludeNullValues: false,
			QueryFormat:       &queryFormat,
		},
		Results: &ResultStruct{
			Size:             10,
			IncludeFields:    []string{"*"},
			ExcludeFields:    []string{},
			HighlightFields:  []string{},
			HighlightOptions: highlightOptions,
			Highlight:        false,
		},
		Language: &LanguageStruct{
			Language:            "universal",
			ApplyStopwords:      true,
			CustomStopwords:     []string{},
			StemmingExceptions:  []string{},
			NormalizeDiacritics: true,
		},
		Synonyms: &SynonymsStruct{
			Enabled: true,
		},
		Rules: &RulesStruct{
			Enabled: true,
		},
		IndexSettings: &IndexSettingStruct{
			EnableNgram: true,
		},
	}

	return defaultSetting
}

// SetSearchRelevancySettingsCache to set all settings in cache
func SetSearchRelevancySettingsCache(settings map[string]SearchRelevancyStruct) {
	SearchRelevancyCacheMutex.Lock()
	SearchRelevancySettingsCache = settings
	SearchRelevancyCacheMutex.Unlock()
}

// GetSearchRelevancySettingsFromCache to get all settings from cache
func GetSearchRelevancySettingsFromCache() map[string]SearchRelevancyStruct {
	return SearchRelevancySettingsCache
}

// UpdateSearchRelevancyCache to update existing setting or add new setting
func UpdateSearchRelevancyCache(index string, setting SearchRelevancyStruct) {
	SearchRelevancyCacheMutex.Lock()
	SearchRelevancySettingsCache[index] = setting
	SearchRelevancyCacheMutex.Unlock()
}

// RemoveFromSearchRelevancyCache function to remove from cache
func RemoveFromSearchRelevancyCache(index string) {
	SearchRelevancyCacheMutex.Lock()
	delete(SearchRelevancySettingsCache, index)
	SearchRelevancyCacheMutex.Unlock()
}

// GetSearchRelevancySettingFromCache to get a setting from cache
func GetSearchRelevancySettingFromCache(indexName string) (SearchRelevancyStruct, error) {
	var searchRelevancySetting SearchRelevancyStruct
	if val, ok := SearchRelevancySettingsCache[indexName]; ok {
		searchRelevancySetting = val
		return searchRelevancySetting, nil
	}

	return searchRelevancySetting, errors.New("Index not found in cache")
}
