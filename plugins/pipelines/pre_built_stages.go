package pipelines

import (
	"encoding/json"
	"fmt"

	"github.com/invopop/jsonschema"
)

type PreBuiltStage int

const (
	Authorization PreBuiltStage = iota
	ElasticSearchQuery
	ReactiveSearchQuery
	UseCache
	RecordAnalytics
	PromoteResults
	HideResults
	CustomData
	ReplaceSearchTerm
	AddFilter
	RemoveWords
	ReplaceWords
	SearchRelevancy
	KnnResponse
	HttpRequest
	MongoDBQuery
	SolrQuery
	ZincQuery
	RecordClick
	RecordSaveSearch
	RecordFavorite
	RecordConversion
	SearchboxPreferences
	Boost
	OpenAIEmbeddings
	OpenAIEmbeddingsIndex
	AIAnswer
	ValidateStage
)

// String is the implementation of Stringer interface that returns the string representation of PreBuiltStage type.
func (o PreBuiltStage) String() string {
	return [...]string{
		"authorization",
		"elasticsearchQuery",
		"reactivesearchQuery",
		"useCache",
		"recordAnalytics",
		"promoteResults",
		"hideResults",
		"customData",
		"replaceSearchTerm",
		"addFilter",
		"removeWords",
		"replaceWords",
		"searchRelevancy",
		"kNN",
		"httpRequest",
		"mongoDBQuery",
		"solrQuery",
		"zincQuery",
		"recordClick",
		"recordSaveSearch",
		"recordFavorite",
		"recordConversion",
		"searchboxPreferences",
		"boost",
		"openAIEmbeddings",
		"openAIEmbeddingsIndex",
		"AIAnswer",
		"validateQuery",
	}[o]
}

// UnmarshalJSON is the implementation of the Unmarshaler interface for unmarshaling PreBuiltStage type.
func (o *PreBuiltStage) UnmarshalJSON(bytes []byte) error {
	var preBuiltStage string
	err := json.Unmarshal(bytes, &preBuiltStage)
	if err != nil {
		return err
	}
	switch preBuiltStage {
	case Authorization.String():
		*o = Authorization
	case ElasticSearchQuery.String():
		*o = ElasticSearchQuery
	case ReactiveSearchQuery.String():
		*o = ReactiveSearchQuery
	case UseCache.String():
		*o = UseCache
	case RecordAnalytics.String():
		*o = RecordAnalytics
	case PromoteResults.String():
		*o = PromoteResults
	case SearchRelevancy.String():
		*o = SearchRelevancy
	case HideResults.String():
		*o = HideResults
	case CustomData.String():
		*o = CustomData
	case ReplaceSearchTerm.String():
		*o = ReplaceSearchTerm
	case AddFilter.String():
		*o = AddFilter
	case RemoveWords.String():
		*o = RemoveWords
	case ReplaceWords.String():
		*o = ReplaceWords
	case KnnResponse.String():
		*o = KnnResponse
	case HttpRequest.String():
		*o = HttpRequest
	case MongoDBQuery.String():
		*o = MongoDBQuery
	case SolrQuery.String():
		*o = SolrQuery
	case ZincQuery.String():
		*o = ZincQuery
	case RecordClick.String():
		*o = RecordClick
	case RecordSaveSearch.String():
		*o = RecordSaveSearch
	case RecordFavorite.String():
		*o = RecordFavorite
	case RecordConversion.String():
		*o = RecordConversion
	case SearchboxPreferences.String():
		*o = SearchboxPreferences
	case Boost.String():
		*o = Boost
	case OpenAIEmbeddings.String():
		*o = OpenAIEmbeddings
	case OpenAIEmbeddingsIndex.String():
		*o = OpenAIEmbeddingsIndex
	case AIAnswer.String():
		*o = AIAnswer
	case ValidateStage.String():
		*o = ValidateStage
	default:
		return fmt.Errorf("invalid preBuiltStage encountered: %v", preBuiltStage)
	}
	return nil
}

// MarshalJSON is the implementation of the Marshaler interface for marshaling PreBuiltStage type.
func (o PreBuiltStage) MarshalJSON() ([]byte, error) {
	var preBuiltStage string
	switch o {
	case Authorization:
		preBuiltStage = Authorization.String()
	case ElasticSearchQuery:
		preBuiltStage = ElasticSearchQuery.String()
	case ReactiveSearchQuery:
		preBuiltStage = ReactiveSearchQuery.String()
	case UseCache:
		preBuiltStage = UseCache.String()
	case RecordAnalytics:
		preBuiltStage = RecordAnalytics.String()
	case PromoteResults:
		preBuiltStage = PromoteResults.String()
	case SearchRelevancy:
		preBuiltStage = SearchRelevancy.String()
	case HideResults:
		preBuiltStage = HideResults.String()
	case CustomData:
		preBuiltStage = CustomData.String()
	case ReplaceSearchTerm:
		preBuiltStage = ReplaceSearchTerm.String()
	case AddFilter:
		preBuiltStage = AddFilter.String()
	case RemoveWords:
		preBuiltStage = RemoveWords.String()
	case ReplaceWords:
		preBuiltStage = ReplaceWords.String()
	case KnnResponse:
		preBuiltStage = KnnResponse.String()
	case HttpRequest:
		preBuiltStage = HttpRequest.String()
	case MongoDBQuery:
		preBuiltStage = MongoDBQuery.String()
	case SolrQuery:
		preBuiltStage = SolrQuery.String()
	case ZincQuery:
		preBuiltStage = ZincQuery.String()
	case RecordClick:
		preBuiltStage = RecordClick.String()
	case RecordSaveSearch:
		preBuiltStage = RecordSaveSearch.String()
	case RecordConversion:
		preBuiltStage = RecordConversion.String()
	case RecordFavorite:
		preBuiltStage = RecordFavorite.String()
	case SearchboxPreferences:
		preBuiltStage = SearchboxPreferences.String()
	case Boost:
		preBuiltStage = Boost.String()
	case OpenAIEmbeddings:
		preBuiltStage = OpenAIEmbeddings.String()
	case OpenAIEmbeddingsIndex:
		preBuiltStage = OpenAIEmbeddingsIndex.String()
	case AIAnswer:
		preBuiltStage = AIAnswer.String()
	case ValidateStage:
		preBuiltStage = ValidateStage.String()
	default:
		return nil, fmt.Errorf("invalid preBuiltStage encountered: %v", o)
	}
	return json.Marshal(preBuiltStage)
}

// To define the schema definitions for stage inputs
func GetStageInputs(o PreBuiltStage) map[string]interface{} {
	switch o {
	case Authorization:
		return nil
	case ElasticSearchQuery:
		return GetElasticsearchQueryInputSchema()
	case ReactiveSearchQuery:
		return nil
	case UseCache:
		return nil
	case RecordAnalytics:
		return nil
	case PromoteResults:
		return GetPromotedResultsInputSchema()
	case SearchRelevancy:
		return GetSearchRelevancyInputSchema()
	case HideResults:
		return GetHideResultsInputSchema()
	case CustomData:
		return GetCustomDataInputSchema()
	case ReplaceSearchTerm:
		return GetReplaceSearchInputSchema()
	case AddFilter:
		return GetAddFilterInputSchema()
	case RemoveWords:
		return GetRemoveWordsInputSchema()
	case ReplaceWords:
		return GetReplaceWordsInputSchema()
	case KnnResponse:
		return GetKNNInputSchema()
	case HttpRequest:
		return GetHTTPRequestInputSchema()
	case MongoDBQuery:
		return GetMongoDBQueryInputSchema()
	case SolrQuery:
		return GetSolrInputSchema()
	case ZincQuery:
		return GetZincInputSchema()
	case SearchboxPreferences:
		return GetSearchboxPreferencesInputSchema()
	case Boost:
		return GetBoostInputSchema()
	case OpenAIEmbeddings:
		return GetOpenAIEmbeddingsInputSchema()
	case OpenAIEmbeddingsIndex:
		return GetOpenAIEmbeddingsIndexInputSchema()
	case AIAnswer:
		return GetAIAnswerInputSchema()
	case ValidateStage:
		return nil
	default:
		return nil
	}
}

// To define the title for stage
func GetStageTitle(o PreBuiltStage) string {
	switch o {
	case Authorization:
		return "Authorization"
	case ElasticSearchQuery:
		return "Elasticsearch"
	case ReactiveSearchQuery:
		return "Reactivesearch"
	case UseCache:
		return "Apply Cached Response"
	case RecordAnalytics:
		return "Record Analytics"
	case PromoteResults:
		return "Promote Results"
	case SearchRelevancy:
		return "Search Relevancy"
	case HideResults:
		return "Hide Results"
	case CustomData:
		return "Apply Custom Data"
	case ReplaceSearchTerm:
		return "Replace Search Term"
	case AddFilter:
		return "Add Filter"
	case RemoveWords:
		return "Remove Words"
	case ReplaceWords:
		return "Replace Words"
	case KnnResponse:
		return "kNN Response"
	case HttpRequest:
		return "HTTP Request"
	case MongoDBQuery:
		return "MongoDB"
	case SolrQuery:
		return "Solr"
	case ZincQuery:
		return "Zinc"
	case RecordClick:
		return "Record Click"
	case RecordConversion:
		return "Record Conversion"
	case RecordSaveSearch:
		return "Record Saved Search"
	case RecordFavorite:
		return "Record Favorite"
	case SearchboxPreferences:
		return "Searchbox Preferences"
	case Boost:
		return "Boost"
	case OpenAIEmbeddings:
		return "Open AI Embeddings"
	case OpenAIEmbeddingsIndex:
		return "Open AI Embeddings Index"
	case AIAnswer:
		return "Answer AI"
	case ValidateStage:
		return "Validate Query"
	default:
		return ""
	}
}

// To define the description for stage
func GetStageDescripton(o PreBuiltStage) string {
	switch o {
	case Authorization:
		return "Authorize users with Appbase permissions."
	case ElasticSearchQuery:
		return `To perform a request to Elasticsearch Backend.

Elasticsearch stage supports async execution. In case of asynchronous execution, you're expected to write a merge script to consume the response from elasticsearch.
The elasticsearch response would be present in global script context with stage Id (defaults to elasticsearchQuery) as key.`
	case ReactiveSearchQuery:
		return "Reactivesearch stage translates the reactivesearch query to equivalent query of search backend."
	case UseCache:
		return "To apply the cached response. It is recommended to apply cached response after authentication."
	case RecordAnalytics:
		return "To record Appbase search analytics. Record Analytics stage is only applicable for reactivesearch requests."
	case PromoteResults:
		return "Helps in promoting results at a certain position in your result set. For example, when a user searches for iphone you want to promote air pods."
	case SearchRelevancy:
		return "Search Relevancy stage is useful to define the default settings for search queries. For example, to set the default size as 10 for search queries."
	case HideResults:
		return "It helps in hiding certain results from getting included in the actual search results. For example, you want to hide products that not available in the store, or you want to hide results that contain irrelevant data."
	case CustomData:
		return "Helps in sending the custom JSON data in the search response. This will be helpful when you want to send some extra information to the frontend, which can help in rendering more specific information."
	case ReplaceSearchTerm:
		return "It helps in replacing the user's entire search query with another query. Helps in showing relevant results to users, especially when you are aware of the analytics that certain search term is returning no results."
	case AddFilter:
		return "Add Filter action allows you to define the term filters that will get applied on the search type of queries. For example, if somebody searches for iphone then you may want to apply a brand filter with value as apple."
	case RemoveWords:
		return `Removing words is the progressive loosening of query constraints to include more results when none are initially found.

For example, imagine an online smartphone shop that sold a limited inventory of iPhones in only 16GB and 32GB varieties. Users searching for “iphone 5 64gb” would see no results. This is not ideal behavior - it would be far better to show users some iPhone 5 results instead of a blank page.`
	case ReplaceWords:
		return "It allows you to replace words in search query. For example, if you make tv a synonym for television, the stage can replace tv with television so that only television is used to search."
	case KnnResponse:
		return "k Nearest Neighbor (kNN) modifies the ElasticSearch Query with a script_score condition to re-rank the top n results."
	case HttpRequest:
		return "This stage is useful to perform HTTP requests."
	case MongoDBQuery:
		return `To perform a request to MongoDB.

MongoDB stage supports async execution. In case of asynchronous execution, you're expected to write a merge script to consume the response from mongoDB.
The mongoDB response would be present in global script context with stage Id (defaults to mongoDBQuery) as key.`
	case SolrQuery:
		// TODO: Add description for whether or not async is supported
		return `To perform a request to a Solr search engine`
	case ZincQuery:
		return `To perform a request to Zinc`
	case RecordClick:
		return `Record click event for a search query term or a previously searched query (represented by a X-Search-Id)`
	case RecordConversion:
		return `Record conversion event for a previously searched query (represented by a X-Search-Id)`
	case RecordSaveSearch:
		return `Record the search state of a ReactiveSearch query, useful for replaying a query at a later time`
	case RecordFavorite:
		return `Record a favorite (aka like) user action for a search result document (aka hit)`
	case SearchboxPreferences:
		return `Searchbox Preferences stage is useful to specify the searchbox preference id for making suggestion queries. A searchbox id represents the design + layout, popular, recent, and endpoint settings for a suggestions experience, configurable from the ReactiveSearch dashboard`
	case Boost:
		return `Boost stage is useful to boost search results by a specific field’s value, for example, show items with 'holiday-sale' and 'premium' values for 'tag' field above other items.`
	case OpenAIEmbeddings:
		return `Get vector embeddings from OpenAI based on the passed text value`
	case OpenAIEmbeddingsIndex:
		return `This stage will generate the vector embedding of the inputKeys passed and inject the output to the request body`
	case AIAnswer:
		return `This stage will generate answers based on the top results from the request query and return them`
	case ValidateStage:
		return `This stage will generate the validate response of the request body and return that in the response. This stage, when triggered will stop the execution of the pipeline.`
	default:
		return ""
	}
}

func (o PreBuiltStage) JSONSchema() *jsonschema.Schema {
	stagesList := []PreBuiltStage{
		Authorization,
		ElasticSearchQuery,
		ReactiveSearchQuery,
		UseCache,
		RecordAnalytics,
		PromoteResults,
		HideResults,
		CustomData,
		ReplaceSearchTerm,
		AddFilter,
		RemoveWords,
		ReplaceWords,
		SearchRelevancy,
		KnnResponse,
		HttpRequest,
		MongoDBQuery,
		SolrQuery,
		ZincQuery,
		RecordClick,
		RecordConversion,
		RecordFavorite,
		RecordSaveSearch,
		SearchboxPreferences,
		Boost,
		OpenAIEmbeddings,
		OpenAIEmbeddingsIndex,
		AIAnswer,
		ValidateStage,
	}
	var stages = make(map[string]interface{})
	for _, stage := range stagesList {
		stages[stage.String()] = map[string]interface{}{
			"title":       GetStageTitle(stage),
			"description": GetStageDescripton(stage),
			"inputs":      GetStageInputs(stage),
		}
	}
	additionalProperties := map[string]interface{}{
		"stages": stages,
	}

	// extract stage names
	var stageNames = make([]interface{}, 0)
	for _, v := range stagesList {
		stageNames = append(stageNames, v.String())
	}
	return &jsonschema.Schema{
		Type:   "string",
		Enum:   stageNames,
		Extras: additionalProperties,
	}
}
