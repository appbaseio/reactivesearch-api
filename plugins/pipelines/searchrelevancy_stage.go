package pipelines

import (
	"encoding/json"
	"time"

	"github.com/appbaseio-confidential/reactivesearch/plugins/querytranslate"
	"github.com/appbaseio-confidential/reactivesearch/plugins/rules"
	log "github.com/sirupsen/logrus"
)

type SearchRelevancyStruct struct {
	Search     *querytranslate.Query `json:"search" jsonschema:"title=Search Query Settings" jsonschema_description:"Default query settings applicable to 'search' type of queries. You can find all the properties at [here](https://docs.reactivesearch.io/docs/search/reactivesearch-api/reference)."`
	Suggestion *querytranslate.Query `json:"suggestion" jsonschema:"title=Search Query Settings" jsonschema_description:"Default query settings applicable to 'suggestion' type of queries. You can find all the properties at [here](https://docs.reactivesearch.io/docs/search/reactivesearch-api/reference)."`
	Term       *querytranslate.Query `json:"term" jsonschema:"title=Search Query Settings" jsonschema_description:"Default query settings applicable to 'term' type of queries. You can find all the properties at [here](https://docs.reactivesearch.io/docs/search/reactivesearch-api/reference)."`
	Range      *querytranslate.Query `json:"range" jsonschema:"title=Search Query Settings" jsonschema_description:"Default query settings applicable to 'range' type of queries. You can find all the properties at [here](https://docs.reactivesearch.io/docs/search/reactivesearch-api/reference)."`
	Geo        *querytranslate.Query `json:"geo" jsonschema:"title=Search Query Settings" jsonschema_description:"Default query settings applicable to 'geo' type of queries. You can find all the properties at [here](https://docs.reactivesearch.io/docs/search/reactivesearch-api/reference)."`
}

func GetSearchRelevancyInputSchema() map[string]interface{} {
	schema := GetReflactor().Reflect(&SearchRelevancyStruct{})
	inputsSchema, _ := schema.MarshalJSON()
	var schemaInBytes map[string]interface{}
	json.Unmarshal(inputsSchema, &schemaInBytes)
	return schemaInBytes
}

func ApplyDefaultSearchSettings(originalQuery querytranslate.Query, defaultSettings querytranslate.Query) querytranslate.Query {
	finalQuery := originalQuery
	normalizedFields := querytranslate.NormalizedDataFields(finalQuery.DataField, finalQuery.FieldWeights)
	if len(normalizedFields) == 0 {
		finalQuery.DataField = defaultSettings.DataField
		finalQuery.FieldWeights = defaultSettings.FieldWeights
	}
	if finalQuery.React == nil {
		finalQuery.React = defaultSettings.React
	}
	if finalQuery.QueryFormat == nil {
		finalQuery.QueryFormat = defaultSettings.QueryFormat
	}
	if finalQuery.CategoryField == nil {
		finalQuery.CategoryField = defaultSettings.CategoryField
	}
	if finalQuery.CategoryValue == nil {
		finalQuery.CategoryValue = defaultSettings.CategoryValue
	}
	if finalQuery.NestedField == nil {
		finalQuery.NestedField = defaultSettings.NestedField
	}
	if finalQuery.From == nil {
		finalQuery.From = defaultSettings.From
	}
	if finalQuery.Size == nil {
		finalQuery.Size = defaultSettings.Size
	}
	if finalQuery.AggregationSize == nil {
		finalQuery.AggregationSize = defaultSettings.AggregationSize
	}
	if finalQuery.SortBy == nil {
		finalQuery.SortBy = defaultSettings.SortBy
	}
	if finalQuery.Value == nil {
		finalQuery.Value = defaultSettings.Value
	}
	if finalQuery.AggregationField == nil {
		finalQuery.AggregationField = defaultSettings.AggregationField
	}
	if finalQuery.After == nil {
		finalQuery.After = defaultSettings.After
	}
	if finalQuery.IncludeNullValues == nil {
		finalQuery.IncludeNullValues = defaultSettings.IncludeNullValues
	}
	if finalQuery.IncludeFields == nil {
		finalQuery.IncludeFields = defaultSettings.IncludeFields
	}
	if finalQuery.ExcludeFields == nil {
		finalQuery.ExcludeFields = defaultSettings.ExcludeFields
	}
	if finalQuery.Fuzziness == nil {
		finalQuery.Fuzziness = defaultSettings.Fuzziness
	}
	if finalQuery.SearchOperators == nil {
		finalQuery.SearchOperators = defaultSettings.SearchOperators
	}
	if finalQuery.Highlight == nil {
		finalQuery.Highlight = defaultSettings.Highlight
	}
	if len(finalQuery.HighlightField) == 0 {
		finalQuery.HighlightField = defaultSettings.HighlightField
	}
	if finalQuery.CustomHighlight == nil {
		finalQuery.CustomHighlight = defaultSettings.CustomHighlight
	}
	if finalQuery.HighlightConfig == nil {
		finalQuery.HighlightConfig = defaultSettings.HighlightConfig
	}
	if finalQuery.Interval == nil {
		finalQuery.Interval = defaultSettings.Interval
	}
	if finalQuery.Aggregations == nil {
		finalQuery.Aggregations = defaultSettings.Aggregations
	}
	if finalQuery.MissingLabel == "" {
		finalQuery.MissingLabel = defaultSettings.MissingLabel
	}
	if finalQuery.ShowMissing == nil {
		finalQuery.ShowMissing = defaultSettings.ShowMissing
	}
	if finalQuery.DefaultQuery == nil {
		finalQuery.DefaultQuery = defaultSettings.DefaultQuery
	}
	if finalQuery.CustomQuery == nil {
		finalQuery.CustomQuery = defaultSettings.CustomQuery
	}
	if finalQuery.Execute == nil {
		finalQuery.Execute = defaultSettings.Execute
	}
	if finalQuery.EnableSynonyms == nil {
		finalQuery.EnableSynonyms = defaultSettings.EnableSynonyms
	}
	if finalQuery.SelectAllLabel == nil {
		finalQuery.SelectAllLabel = defaultSettings.SelectAllLabel
	}
	if finalQuery.Pagination == nil {
		finalQuery.Pagination = defaultSettings.Pagination
	}
	if finalQuery.QueryString == nil {
		finalQuery.QueryString = defaultSettings.QueryString
	}
	if finalQuery.RankFeature == nil {
		finalQuery.RankFeature = defaultSettings.RankFeature
	}
	if finalQuery.DistinctField == nil {
		finalQuery.DistinctField = defaultSettings.DistinctField
	}
	if finalQuery.DistinctFieldConfig == nil {
		finalQuery.DistinctFieldConfig = defaultSettings.DistinctFieldConfig
	}
	if finalQuery.Index == nil {
		finalQuery.Index = defaultSettings.Index
	}
	if finalQuery.EnableRecentSuggestions == nil {
		finalQuery.EnableRecentSuggestions = defaultSettings.EnableRecentSuggestions
	}
	if finalQuery.RecentSuggestionsConfig == nil {
		finalQuery.RecentSuggestionsConfig = defaultSettings.RecentSuggestionsConfig
	}
	if finalQuery.EnablePopularSuggestions == nil {
		finalQuery.EnablePopularSuggestions = defaultSettings.EnablePopularSuggestions
	}
	if finalQuery.PopularSuggestionsConfig == nil {
		finalQuery.PopularSuggestionsConfig = defaultSettings.PopularSuggestionsConfig
	}
	if finalQuery.ShowDistinctSuggestions == nil {
		finalQuery.ShowDistinctSuggestions = defaultSettings.ShowDistinctSuggestions
	}
	if finalQuery.EnablePredictiveSuggestions == nil {
		finalQuery.EnablePredictiveSuggestions = defaultSettings.EnablePredictiveSuggestions
	}
	if finalQuery.MaxPredictedWords == nil {
		finalQuery.MaxPredictedWords = defaultSettings.MaxPredictedWords
	}
	if finalQuery.URLField == nil {
		finalQuery.URLField = defaultSettings.URLField
	}
	if finalQuery.ApplyStopwords == nil {
		finalQuery.ApplyStopwords = defaultSettings.ApplyStopwords
	}
	if finalQuery.Stopwords == nil {
		finalQuery.Stopwords = defaultSettings.Stopwords
	}
	if finalQuery.SearchLanguage == nil {
		finalQuery.SearchLanguage = defaultSettings.SearchLanguage
	}
	if finalQuery.CalendarInterval == nil {
		finalQuery.CalendarInterval = defaultSettings.CalendarInterval
	}
	if finalQuery.SearchBoxId == nil {
		finalQuery.SearchBoxId = defaultSettings.SearchBoxId
	}
	if finalQuery.FeaturedSuggestionsConfig == nil {
		finalQuery.FeaturedSuggestionsConfig = defaultSettings.FeaturedSuggestionsConfig
	}

	return finalQuery
}

func executeSearchRelevancyStage(
	stage ESPipelineStage,
	parsedInputs *string,
	scriptContextInBytes []byte,
	rsAPIRequest *ReactiveSearchQueryContext,
	scriptEnvs map[string]interface{},
	async bool,
	startTime *time.Time) ([]byte, bool, *Error) {
	if parsedInputs == nil {
		return scriptContextInBytes, false, nil
	}

	var scriptContext rules.ScriptContext
	err := json.Unmarshal(scriptContextInBytes, &scriptContext)
	if err != nil {
		log.Errorln(logTag, ":", err)
		return nil, false, &Error{
			Err: err,
		}
	}
	var rsAPIBody querytranslate.RSQuery
	err2 := json.Unmarshal([]byte(scriptContext.Request.Body), &rsAPIBody)
	if err2 != nil {
		log.Errorln(logTag, ":", err2)
		return nil, false, &Error{
			Err: err2,
		}
	}
	// parse stage inputs
	// Unmarshal the input into a map[string]interface{} since it's
	// a string now.
	var inputsMapped = make(map[string]interface{})
	marshalErr := json.Unmarshal([]byte(*parsedInputs), &inputsMapped)
	if marshalErr != nil {
		log.Errorln(logTag, ": error while unmarshalling inputs: ", marshalErr)
		return nil, false, &Error{
			Err: marshalErr,
		}
	}

	inputAsBytes, err := json.Marshal(inputsMapped)
	if err != nil {
		log.Errorln(logTag, ":", err)
		return nil, false, &Error{
			Err: err,
		}
	}
	var searchRelevancySettings SearchRelevancyStruct
	err3 := json.Unmarshal(inputAsBytes, &searchRelevancySettings)
	if err3 != nil {
		log.Errorln(logTag, ":", err3)
		return nil, false, &Error{
			Err: err3,
		}
	}
	// apply search relevancy settings
	for i, query := range rsAPIBody.Query {
		switch query.Type {
		case querytranslate.Search:
			if searchRelevancySettings.Search != nil {
				rsAPIBody.Query[i] = ApplyDefaultSearchSettings(query, *searchRelevancySettings.Search)
			}
		case querytranslate.Suggestion:
			if searchRelevancySettings.Suggestion != nil {
				rsAPIBody.Query[i] = ApplyDefaultSearchSettings(query, *searchRelevancySettings.Suggestion)
			}
		case querytranslate.Term:
			if searchRelevancySettings.Term != nil {
				rsAPIBody.Query[i] = ApplyDefaultSearchSettings(query, *searchRelevancySettings.Term)
			}
		case querytranslate.Geo:
			if searchRelevancySettings.Geo != nil {
				rsAPIBody.Query[i] = ApplyDefaultSearchSettings(query, *searchRelevancySettings.Geo)
			}
		case querytranslate.Range:
			if searchRelevancySettings.Range != nil {
				rsAPIBody.Query[i] = ApplyDefaultSearchSettings(query, *searchRelevancySettings.Range)
			}
		}
	}
	// Update rsAPIBody
	rsAPIRequest.Put(&rsAPIBody)

	// write updated body to context
	rsBodyInBytes, err := json.Marshal(rsAPIBody)
	if err != nil {
		log.Errorln(logTag, ":", err)
		return nil, false, &Error{
			Err: err,
		}
	}
	scriptContext.Request.Body = string(rsBodyInBytes)

	contextInBytes, err := json.Marshal(scriptContext)
	if err != nil {
		log.Errorln(logTag, ":", err)
		return nil, false, &Error{
			Err: err,
		}
	}
	return contextInBytes, false, nil
}
