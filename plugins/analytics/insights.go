package analytics

import (
	"encoding/json"
	"fmt"
)

// InsightType represents the type of the insight
type InsightType int

const (
	// NoResults represents the no_results property in insight
	NoResults InsightType = iota
	// LowClicks represents the low_clicks property in insight
	LowClicks
	// LowSuggestionsClicks represents the `low_suggestions_clicks` property in insight
	LowSuggestionsClicks
	// AvgClickPosition represents the avg_click_position property in insight
	AvgClickPosition
	// PopularSearches represents the popular_searches property in insight
	PopularSearches
	// PopularSearchesWithLowClicks represents the popular_searches_with_low_clicks property in insight
	PopularSearchesWithLowClicks
	// LongTailSearches represents the long_tail_searches property in insight
	LongTailSearches
	// BounceRate represents the bounce_rate property in insight
	BounceRate
	// HighResponseTime represents the high_response_time property in insight
	HighResponseTime
	// Error500 represents the error_500 property in insight
	Error500
	// Error400 represents the error_400 property in insight
	Error400
)

// String is the implementation of Stringer interface that returns the string representation of InsightType type.
func (o InsightType) String() string {
	return [...]string{
		"no_results",
		"low_clicks",
		"low_suggestions_clicks",
		"avg_click_position",
		"popular_searches",
		"popular_searches_with_low_clicks",
		"long_tail_searches",
		"bounce_rate",
		"high_response_time",
		"error_500",
		"error_400",
	}[o]
}

// UnmarshalJSON is the implementation of the Unmarshaler interface for un-marshaling InsightType type.
func (o *InsightType) UnmarshalJSON(bytes []byte) error {
	var insightType string
	err := json.Unmarshal(bytes, &insightType)
	if err != nil {
		return err
	}
	switch insightType {
	case NoResults.String():
		*o = NoResults
	case LowClicks.String():
		*o = LowClicks
	case LowSuggestionsClicks.String():
		*o = LowSuggestionsClicks
	case AvgClickPosition.String():
		*o = AvgClickPosition
	case PopularSearches.String():
		*o = PopularSearches
	case PopularSearchesWithLowClicks.String():
		*o = PopularSearchesWithLowClicks
	case LongTailSearches.String():
		*o = LongTailSearches
	case BounceRate.String():
		*o = BounceRate
	case HighResponseTime.String():
		*o = HighResponseTime
	case Error500.String():
		*o = Error500
	case Error400.String():
		*o = Error400
	default:
		return fmt.Errorf("invalid insightType encountered: %v", insightType)
	}
	return nil
}

// MarshalJSON is the implementation of the Marshaler interface for marshaling InsightType type.
func (o InsightType) MarshalJSON() ([]byte, error) {
	var insightType string
	switch o {
	case NoResults:
		insightType = NoResults.String()
	case LowClicks:
		insightType = LowClicks.String()
	case LowSuggestionsClicks:
		insightType = LowSuggestionsClicks.String()
	case AvgClickPosition:
		insightType = AvgClickPosition.String()
	case PopularSearches:
		insightType = PopularSearches.String()
	case PopularSearchesWithLowClicks:
		insightType = PopularSearchesWithLowClicks.String()
	case LongTailSearches:
		insightType = LongTailSearches.String()
	case BounceRate:
		insightType = BounceRate.String()
	case HighResponseTime:
		insightType = HighResponseTime.String()
	case Error500:
		insightType = Error500.String()
	case Error400:
		insightType = Error400.String()
	default:
		return nil, fmt.Errorf("invalid insightType encountered: %v", o)
	}
	return json.Marshal(insightType)
}

// InsightStatus represents the type of the insight status
type InsightStatus int

const (
	// Saved represents the saved status for insight
	Saved InsightStatus = iota
	// Read represents the read status for insight
	Read
	// Deleted represents the deleted status for insight
	Deleted
)

// String is the implementation of Stringer interface that returns the string representation of InsightStatus type.
func (o InsightStatus) String() string {
	return [...]string{
		"saved",
		"read",
		"deleted",
	}[o]
}

// UnmarshalJSON is the implementation of the Unmarshaler interface for un-marshaling InsightStatus type.
func (o *InsightStatus) UnmarshalJSON(bytes []byte) error {
	var insightStatus string
	err := json.Unmarshal(bytes, &insightStatus)
	if err != nil {
		return err
	}
	switch insightStatus {
	case Saved.String():
		*o = Saved
	case Read.String():
		*o = Read
	case Deleted.String():
		*o = Deleted
	default:
		return fmt.Errorf("invalid insightStatus encountered: %v", insightStatus)
	}
	return nil
}

// MarshalJSON is the implementation of the Marshaler interface for marshaling InsightStatus type.
func (o InsightStatus) MarshalJSON() ([]byte, error) {
	var insightStatus string
	switch o {
	case Saved:
		insightStatus = Saved.String()
	case Read:
		insightStatus = Read.String()
	case Deleted:
		insightStatus = Deleted.String()
	default:
		return nil, fmt.Errorf("invalid insightStatus encountered: %v", o)
	}
	return json.Marshal(insightStatus)
}

// InsightStatusRequest represents the request body to update the status
type InsightStatusRequest struct {
	ID     *InsightType  `json:"id"`
	Status InsightStatus `json:"status"`
}

// InsightStatusEsDoc represents the ES doc for insight status
type InsightStatusEsDoc struct {
	Saved   []InsightType `json:"saved"`
	Read    []InsightType `json:"read"`
	Deleted []InsightType `json:"deleted"`
}

// Recommendation represents the structure of the insight recommendation
type Recommendation struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	// Represents the short link for e.g '/cluster/no-results-searches'
	ShortLink string `json:"short_link"`
	// Represents the full link for e.g 'https://dash.reactivesearch.io/cluster/no-results-searches'
	LongLink string `json:"long_link"`
}

// Insight represents the insight object
type Insight struct {
	// Represents the insight title, must be a valid expression
	Title string `json:"title"`
	// Represents the insight description, must be a valid expression
	Description     string           `json:"description"`
	ShortLink       string           `json:"short_link"`
	Recommendations []Recommendation `json:"recommendations"`
	// expression from the `expr` package to evaluate the insight
	Condition string `json:"condition"`
}

// InsightResponse represents the insight response object
type InsightResponse struct {
	/* Represents the recommendation title, for an example: "There are {value} no result searches".
	{value} is a variable which will be replaced by the actual value
	*/
	Title           string           `json:"title"`
	Description     string           `json:"description"`
	Recommendations []Recommendation `json:"recommendations"`
	ShortLink       string           `json:"short_link"`
}

// InsightResponseType represents the insight endpoint response
type InsightResponseType struct {
	ID      InsightType     `json:"id"`
	Insight InsightResponse `json:"insight"`
}

// InsightConfigType represents the insight object in the config
type InsightConfigType struct {
	ID      InsightType `json:"id"`
	Insight Insight     `json:"insight"`
}

// Link represents a link
type Link struct {
	ShortLink string
	LongLink  string
}

// Links represents all link
type Links struct {
	Language          Link
	Rules             Link
	SearchSettings    Link
	Synonyms          Link
	QuerySuggestions  Link
	ResultSettings    Link
	IndexSettings     Link
	RequestLogs       Link
	NoResultsSearches Link
	PopularSearches   Link
}

var links = Links{
	Language: Link{
		ShortLink: "app/:index/languages",
	},
	Rules: Link{
		ShortLink: "cluster/rules",
	},
	SearchSettings: Link{
		ShortLink: "app/:index/search",
	},
	Synonyms: Link{
		ShortLink: "app/:index/synonyms",
	},
	QuerySuggestions: Link{
		ShortLink: "app/:index/query-suggestions",
	},
	ResultSettings: Link{
		ShortLink: "app/:index/results",
	},
	IndexSettings: Link{
		ShortLink: "app/:index/index-settings",
	},
	RequestLogs: Link{
		ShortLink: "app/:index/request-logs",
	},
	NoResultsSearches: Link{
		ShortLink: "app/:index/no-results-searches",
	},
	PopularSearches: Link{
		ShortLink: "app/:index/popular-searches",
	},
}

/*
InsightConfig can be used to configure the insights and recommendations.
Please follow the instructions carefully before editing:
1. Insight title must be a valid expression from `expr` library, for example: '`"There are "+TotalNoResultsSearches+" no result searches"`'.
2. Insight is driven by the `Condition` property which is also an expression.
3. Check the `DetailedSummary` struct in `plugins/analytics/util.go` to know the env. variables available for expression.
4. You can also use markdown for the string fields for eg. 'Get the [reactivesearch.io](reactivesearch.io) support plan'
5. The order of the insights and recommendations will be determined from this config, so any changes in the order will affect the order in the UI.
*/
var InsightConfig = []InsightConfigType{
	{
		ID: NoResults,
		Insight: Insight{
			Title:       `"There are "+FormatNumber(ToFloat(TotalNoResultsSearches))+" no results searches"`,
			Description: `"No results searches create a poor user experience for your users."`,
			Condition:   "TotalNoResultsSearches > 0",
			ShortLink:   links.NoResultsSearches.ShortLink,
			Recommendations: []Recommendation{
				{
					Title:       "Language Settings",
					Description: "Set the language, stemming and stop words settings in the Language settings.",
					ShortLink:   links.Language.ShortLink,
				},
				{
					Title:       "Search Settings",
					Description: "Ensure that the searchable fields are set. You can also enable the typo tolerance setting.",
					ShortLink:   links.SearchSettings.ShortLink,
				},
				{
					Title:       "Synonyms",
					Description: "Map the no results search terms to data that's present in your index.",
					ShortLink:   links.Synonyms.ShortLink,
				},
				{
					Title:       "Query Rules",
					Description: "Create a query rule to promote results for the no results search terms.",
					ShortLink:   links.Rules.ShortLink,
				},
			},
		},
	},
	{
		ID: LowClicks,
		Insight: Insight{
			Title:       `"Less than "+AvgClickRate+"% of your searches receive a click"`,
			Description: `"Your searches saw an average click rate of "+AvgClickRate+"%."`,
			Condition:   "AvgClickRate > 0 and AvgClickRate <= 5",
			Recommendations: []Recommendation{
				{
					Title:       "Language Settings",
					Description: "Set the language, stemming and stop words settings in the Language settings.",
					ShortLink:   links.Language.ShortLink,
				},
				{
					Title:       "Search Settings",
					Description: "Ensure that the searchable fields are set correctly and weighted appropriately.",
					ShortLink:   links.SearchSettings.ShortLink,
				},
				{
					Title:       "Query Rules",
					Description: "Create a query rule to promote more relevant results.",
					ShortLink:   links.Rules.ShortLink,
				},
			},
		},
	},
	{
		ID: LowSuggestionsClicks,
		Insight: Insight{
			Title:       `"Less than "+AvgSuggestionsClickRate+"% of your auto suggestion searches receive a click"`,
			Description: `"Your auto suggestion searches saw an average click rate of "+AvgSuggestionsClickRate+"%."`,
			Condition:   "AvgSuggestionsClickRate > 0 and AvgSuggestionsClickRate <= 5",
			Recommendations: []Recommendation{
				{
					Title:       "Language Settings",
					Description: "Set the language, stemming and stop words settings in the Language settings.",
					ShortLink:   links.Language.ShortLink,
				},
				{
					Title:       "Search Settings",
					Description: "Ensure that the searchable fields are set correctly and weighted appropriately.",
					ShortLink:   links.SearchSettings.ShortLink,
				},
				{
					Title:       "Query Rules",
					Description: "Create a query rule to promote more relevant results.",
					ShortLink:   links.Rules.ShortLink,
				},
				{
					Title:       "Query suggestions",
					Description: "Set query suggestions to show suggestions recommendations based on analytics data.",
					ShortLink:   links.QuerySuggestions.ShortLink,
				},
			},
		},
	},
	{
		ID: AvgClickPosition,
		Insight: Insight{
			Title:       `"The average click position for your results is "+AvgClickPosition`,
			Description: `"A higher value for average click position indicates that your users need to scroll through before they find what they are looking for."`,
			Condition:   "AvgClickPosition >= 5",
			Recommendations: []Recommendation{
				{
					Title:       "Search Settings",
					Description: "Ensure that the searchable fields are set correctly and weighted appropriately. You can test with other search settings.",
					ShortLink:   links.SearchSettings.ShortLink,
				},
			},
		},
	},
	{
		ID: PopularSearches,
		Insight: Insight{
			Title:       `"You have "+FormatNumber(ToFloat(PopularSearches))" popular searches"`,
			Description: `"Congrats! These terms are searched for at least 100 times by your users."`,
			Condition:   "PopularSearches > 0",
			ShortLink:   links.PopularSearches.ShortLink,
			Recommendations: []Recommendation{
				{
					Title:       "Query Rules",
					Description: "Create a query rule for these terms to merchandise other items.",
					ShortLink:   links.Rules.ShortLink,
				},
			},
		},
	},
	{
		ID: PopularSearchesWithLowClicks,
		Insight: Insight{
			Title:       `"Less than "+AvgClickRatePopularSearches+"% of your popular searches receive a click"`,
			Description: `"On an average, your popular searches saw a click rate of "+AvgClickRatePopularSearches+"%."`,
			Condition:   "PopularSearches > 0 and AvgClickRatePopularSearches <= 5",
			ShortLink:   links.PopularSearches.ShortLink,
			Recommendations: []Recommendation{
				{
					Title:       "Language Settings",
					Description: "Set the language, stemming and stop words settings in the Language settings.",
					ShortLink:   links.Language.ShortLink,
				},
				{
					Title:       "Search Settings",
					Description: "Ensure that the searchable fields are set correctly and weighted appropriately.",
					ShortLink:   links.SearchSettings.ShortLink,
				},
				{
					Title:       "Query Rules",
					Description: "Create a query rule to promote more relevant results.",
					ShortLink:   links.Rules.ShortLink,
				},
			},
		},
	},
	{
		ID: LongTailSearches,
		Insight: Insight{
			Title:           `"Analyze the long tail search terms"`,
			Description:     `"Long-tail searches often tell you what your power users are looking for."`,
			Condition:       "AvgQueryLength >= 4",
			Recommendations: []Recommendation{},
		},
	},
	{
		ID: BounceRate,
		Insight: Insight{
			Title:       `"Your search bounce rate is "+AvgBounceRate+"%"`,
			Description: `"Bounce rate indicates sessions where a user left without interacting with the search."`,
			Condition:   "AvgBounceRate > 50",
			Recommendations: []Recommendation{
				{
					Title:       "Improve your search experience",
					Description: "You can use external tools such as Hotjar to qualitatively understand the gaps in your search experience.",
				},
			},
		},
	},
	{
		ID: HighResponseTime,
		Insight: Insight{
			Title:       `FormatNumber(ToFloat(HighResponseTimeSearches))+" searches took more than a second"`,
			Description: `"At least 10% of your total searches are taking more than a second to return. A lower response time provides a better user experience."`,
			Condition:   "HighResponseTimeSearches > 0",
			Recommendations: []Recommendation{
				{
					Title:       "Review Results Settings",
					Description: "You can set what fields to return and optimize your highlighting settings to improve the search response time.",
					ShortLink:   links.ResultSettings.ShortLink,
				},
				{
					Title:       "Review Index settings",
					Description: "An optimal shard and replica setting can provide a better search response time.",
					ShortLink:   links.IndexSettings.ShortLink,
				},
			},
		},
	},
	{
		ID: Error500,
		Insight: Insight{
			Title:       `FormatNumber(ToFloat(InternalServerErrors))+" searches returned an internal server error"`,
			Description: `"Internal server errors indicate an issue with the underlying ElasticSearch service."`,
			Condition:   "InternalServerErrors > 0",
			ShortLink:   links.RequestLogs.ShortLink,
			Recommendations: []Recommendation{
				{
					Title:       "Optimize your ElasticSearch configuration",
					Description: "Check with your service provider on what can be done to improve your search infrastructure's availability. If your search volume is growing, you may also want to look at upgrading the search servers.",
				},
			},
		},
	},
	{
		ID: Error400,
		Insight: Insight{
			Title:       `FormatNumber(ToFloat(BadRequestErrors))+" searches failed due to a client error"`,
			Condition:   "BadRequestErrors > 0",
			Description: `"Your searches failed due to a bad request or an authentication error."`,
			ShortLink:   links.RequestLogs.ShortLink,
			Recommendations: []Recommendation{
				{
					Title: "Analyze your logs for errors.",
				},
			},
		},
	},
}
