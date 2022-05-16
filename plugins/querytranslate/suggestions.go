package querytranslate

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/appbaseio/reactivesearch-api/util"
)

type ActionType int

const (
	Navigate ActionType = iota
	Function
)

// String is the implementation of Stringer interface that returns the string representation of ActionType type.
func (o ActionType) String() string {
	return [...]string{
		"navigate",
		"function",
	}[o]
}

// UnmarshalJSON is the implementation of the Unmarshaler interface for unmarshaling ActionType type.
func (o *ActionType) UnmarshalJSON(bytes []byte) error {
	var sectionType string
	err := json.Unmarshal(bytes, &sectionType)
	if err != nil {
		return err
	}
	switch sectionType {
	case Navigate.String():
		*o = Navigate
	case Function.String():
		*o = Function
	default:
		return fmt.Errorf("invalid suggestion type encountered: %v", sectionType)
	}
	return nil
}

// MarshalJSON is the implementation of the Marshaler interface for marshaling ActionType type.
func (o ActionType) MarshalJSON() ([]byte, error) {
	var sectionType string
	switch o {
	case Navigate:
		sectionType = Navigate.String()
	case Function:
		sectionType = Function.String()
	default:
		return nil, fmt.Errorf("invalid suggestion type encountered: %v", o)
	}
	return json.Marshal(sectionType)
}

// SuggestionHIT represents the structure of the suggestion object in RS API response
type SuggestionHIT struct {
	Value string  `json:"value"`
	Label string  `json:"label"`
	URL   *string `json:"url"`
	// Default Suggestions properties
	SectionLabel  *string        `json:"sectionLabel"`
	SectionId     *string        `json:"sectionId"`
	Description   *string        `json:"description"`
	Action        *ActionType    `json:"action"`
	SubAction     *string        `json:"subAction"`
	Icon          *string        `json:"icon"`
	IconURL       *string        `json:"iconURL"`
	Type          SuggestionType `json:"_suggestion_type"`
	Category      *string        `json:"_category"`
	Count         *int           `json:"_count"`
	RSScore       float64        `json:"_rs_score"`
	MatchedTokens []string       `json:"_matched_tokens"`
	// ES response properties
	Id     string                 `json:"_id"`
	Index  *string                `json:"_index"`
	Score  float64                `json:"_score"`
	Source map[string]interface{} `json:"_source"`
}

type SuggestionHitResponse struct {
	Total    interface{}     `json:"total"`
	MaxScore interface{}     `json:"max_score"`
	Hits     []SuggestionHIT `json:"hits"`
}

// Response of the suggestions API similar to the ES response
type SuggestionESResponse struct {
	Took int                   `json:"took"`
	Hits SuggestionHitResponse `json:"hits"`
}

// TODO: Add MinCount to recent suggestion
// RecentSuggestionsOptions represents the options to configure recent suggestions
type RecentSuggestionsOptions struct {
	Size         *int                   `json:"size,omitempty"`
	Index        *string                `json:"index,omitempty"`
	MinHits      *int                   `json:"minHits,omitempty"`
	MinChars     *int                   `json:"minChars,omitempty"`
	CustomEvents map[string]interface{} `json:"customEvents,omitempty"`
	SectionLabel *string                `json:"sectionLabel,omitempty"`
}

// PopularSuggestionsOptions represents the options to configure popular suggestions
type PopularSuggestionsOptions struct {
	Size         *int                   `json:"size,omitempty"`
	Index        *string                `json:"index,omitempty"`
	ShowGlobal   *bool                  `json:"showGlobal,omitempty"`
	MinChars     *int                   `json:"minChars,omitempty"`
	MinCount     *int                   `json:"minCount,omitempty"`
	CustomEvents map[string]interface{} `json:"customEvents,omitempty"`
	SectionLabel *string                `json:"sectionLabel,omitempty"`
}

// FeaturedSuggestionsOptions represents the options to configure default suggestions
type FeaturedSuggestionsOptions struct {
	FeaturedSuggestionsGroupId   *string   `json:"featuredSuggestionsGroupId,omitempty"`
	VisibleSuggestionsPerSection *int      `json:"visibleSuggestionsPerSection,omitempty"`
	MaxSuggestionsPerSection     *int      `json:"maxSuggestionsPerSection,omitempty"`
	SectionsOrder                *[]string `json:"sectionsOrder,omitempty"`
}

// IndexSuggestionsOptions represents the options to configure index suggestions
type IndexSuggestionsOptions struct {
	SectionLabel *string `json:"sectionLabel,omitempty"`
}

// DocField contains properties of the field and the doc it belongs to
type DocField struct {
	value  string
	rawHit ESDoc
}

// RankField contains info about a field's matching value to a user query
type RankField struct {
	fieldValue    string
	userQuery     string
	score         float64
	matchedTokens []string
}

// ESDoc contains info about an ES document
type ESDoc struct {
	Index        string                 `json:"_index"`
	Type         string                 `json:"type"`
	Id           string                 `json:"_id"`
	Score        float64                `json:"_score"`
	Source       map[string]interface{} `json:"_source"`
	Highlight    map[string]interface{} `json:"highlight"`
	ParsedSource map[string]interface{}
}

type SuggestionsConfig struct {
	// Data fields to parse suggestions.
	// If not defined then we'll extract fields from source object.
	DataFields []string
	// Query value
	Value                       string
	ShowDistinctSuggestions     *bool
	EnablePredictiveSuggestions *bool
	MaxPredictedWords           *int
	EnableSynonyms              *bool
	ApplyStopwords              *bool
	Stopwords                   *[]string
	URLField                    *string
	HighlightField              []string
	HighlightConfig             *map[string]interface{}
	CategoryField               *string
	Language                    *string
	IndexSuggestionsConfig      *IndexSuggestionsOptions
}

// getIndexSuggestions gets the index suggestions based on user query config and search engine response
func getIndexSuggestions(config SuggestionsConfig, rawHits []ESDoc) []SuggestionHIT {
	// before parsing any suggestions, normalize the query
	config.Value = normalizeValue(config.Value)
	// set priority to highlight fields if present
	if len(config.HighlightField) != 0 {
		config.DataFields = config.HighlightField
	} else if len(config.DataFields) == 0 && len(rawHits) > 0 {
		// extract fields from first hit source
		config.DataFields = extractFieldsFromSource(rawHits[0].Source)
	}

	// parse hits with highlighting
	var parsedHits = make([]ESDoc, 0)
	for _, hit := range rawHits {
		parsedHits = append(parsedHits, addFieldHighlight(hit))
	}

	// keep track of suggestions list
	var suggestionsList = make([]SuggestionHIT, 0)

	// keep track of suggestions label, label must be unique
	var labelsList = make([]string, 0)

	getDefaultSuggestions(config, parsedHits, &suggestionsList, &labelsList)

	// sort suggestions based on the rank
	// First priority is given to the _rs_score
	// Second priority is given to the _score
	sort.SliceStable(suggestionsList, func(i, j int) bool {
		if suggestionsList[i].RSScore > suggestionsList[j].RSScore {
			return true
		}
		if suggestionsList[i].RSScore == suggestionsList[j].RSScore {
			return suggestionsList[i].Score > suggestionsList[j].Score
		}
		return false
	})

	if config.EnablePredictiveSuggestions != nil && *config.EnablePredictiveSuggestions {
		suggestionsList = getPredictiveSuggestions(config, &suggestionsList)
	}

	if config.ShowDistinctSuggestions != nil && *config.ShowDistinctSuggestions {
		// keep track of document ids for suggestions
		var idMap = make(map[interface{}]bool)
		filteredSuggestions := make([]SuggestionHIT, 0)
		for _, suggestion := range suggestionsList {
			if !idMap[suggestion.Id] {
				filteredSuggestions = append(filteredSuggestions, suggestion)
				idMap[suggestion.Id] = true
			}
		}
		return filteredSuggestions
	}
	return suggestionsList
}

// getDefaultSuggestions traverses over ES docs and checks for a suggestion match against each field from the query
// A suggestion is considered matching if
func getDefaultSuggestions(
	config SuggestionsConfig,
	parsedHits []ESDoc,
	suggestionsList *[]SuggestionHIT,
	labelsList *[]string,
) {
	// iterate over ES docs
	for _, hit := range parsedHits {
		// iterate over fields
		for _, field := range config.DataFields {
			parseResponseTree(hit.ParsedSource, field, suggestionsList, labelsList, hit, config)
		}
	}
}

// parseResponseTree parses the suggestions from a response tree
// e.g. could be "a", or "a.b", or "a.b.c"
// This function uses recursion to traverse the response tree
func parseResponseTree(
	responseTree map[string]interface{},
	field string,
	suggestionsList *[]SuggestionHIT,
	labelsList *[]string,
	rawHit ESDoc,
	config SuggestionsConfig,
) {
	// if field path itself contains a string, then we're set
	if responseTree[field] != nil {
		responseSubTree := responseTree[field]
		// val := extractSuggestion(topLabel)
		valAsString, ok := responseSubTree.(string)
		if ok && valAsString != "" {
			docField := DocField{
				value:  valAsString,
				rawHit: rawHit,
			}
			populateDefaultSuggestions(labelsList, suggestionsList, docField, config)
		}
	}

	// To handle field names with dots, e.g. "a.b.c", there's a recursive call to this function
	fieldNodes := strings.Split(field, ".")
	responseSubTree := responseTree[fieldNodes[0]]

	// if the type of field is array of strings
	// then we need to pick first matching value as the label
	rstAsArray, ok := responseSubTree.([]interface{})
	if ok && len(rstAsArray) > 0 {
		for _, i := range rstAsArray {
			labelAsString, ok := i.(string)
			// find the matching label
			if ok && strings.Contains(strings.ToLower(labelAsString), strings.ToLower(config.Value)) {
				responseSubTree = labelAsString
				break
			}
			// array can also contain objects
			rssTree, ok := i.(map[string]interface{})
			if ok {
				// nested fields of the 'variants.title' variety
				childField := field[len(fieldNodes[0])+1:]
				parseResponseTree(rssTree, childField, suggestionsList, labelsList, rawHit, config)
			}
		}
	}

	if responseSubTree != nil {
		if len(fieldNodes) > 1 {
			// nested fields of the 'foo.bar.zoo' variety
			childField := field[len(fieldNodes[0])+1:]
			responseSubTree, ok := responseSubTree.(map[string]interface{})
			if ok {
				parseResponseTree(responseSubTree, childField, suggestionsList, labelsList, rawHit, config)
			}
		} else {
			valAsString, ok := responseSubTree.(string)
			if ok {
				docField := DocField{
					value:  valAsString,
					rawHit: rawHit,
				}
				populateDefaultSuggestions(labelsList, suggestionsList, docField, config)
			}
		}
	}
}

// populateDefaultSuggestions populates the default (i.e. non-predictive) suggestions using field values of each doc
func populateDefaultSuggestions(
	labelsList *[]string,
	suggestionsList *[]SuggestionHIT,
	docField DocField,
	config SuggestionsConfig,
) {
	if !util.Contains(*labelsList, parseSuggestionLabel(docField.value, config)) {
		var url *string
		if config.URLField != nil {
			urlString, ok := docField.rawHit.Source[*config.URLField].(string)
			if ok {
				url = &urlString
			}
		}
		var category *string
		if config.CategoryField != nil {
			categoryString, ok := docField.rawHit.Source[*config.CategoryField].(string)
			if ok {
				category = &categoryString
			}
		}
		// stores the normalized field value to match agains the normalized query value
		fieldValue := normalizeValue(GetTextFromHTML(docField.value))
		// TODO: This won't work on query synonyms, need to account for that
		rankField := FindMatch(fieldValue, config.Value, config)

		sectionId := "index"
		var sectionLabel *string
		if config.IndexSuggestionsConfig != nil {
			sectionLabel = config.IndexSuggestionsConfig.SectionLabel
		}
		// helpful for debugging
		// fmt.Println("query: ", config.Value, ", field value: ", fieldValue, ", match score: ", rankField.score, ", matched tokens: ", rankField.matchedTokens)
		suggestion := SuggestionHIT{
			Value:         fieldValue,
			Label:         docField.value,
			URL:           url,
			Type:          Index,
			Category:      category,
			RSScore:       rankField.score,
			MatchedTokens: rankField.matchedTokens,
			// ES response properties
			Id:           docField.rawHit.Id,
			Index:        &docField.rawHit.Index,
			Source:       docField.rawHit.Source,
			Score:        docField.rawHit.Score,
			SectionId:    &sectionId,
			SectionLabel: sectionLabel,
		}

		*labelsList = append(*labelsList, parseSuggestionLabel(docField.value, config))
		*suggestionsList = append(*suggestionsList, suggestion)
	}
}

const preTags = `<b class="highlight">`
const postTags = `</b>`

type Tags struct {
	PreTags  string
	PostTags string
}

// getPredictiveSuggestionsTags parses the highlightConfig (if specified)
// and returns the tags
func getPredictiveSuggestionsTags(highlightConfig *map[string]interface{}) Tags {
	var preTags = `<b class="highlight">`
	var postTags = `</b>`

	if highlightConfig != nil {
		config := *highlightConfig
		if config["pre_tags"] != nil {
			tagsAsString, ok := config["pre_tags"].(string)
			if ok {
				preTags = tagsAsString
			} else {
				tagsAsArray, ok := config["pre_tags"].([]interface{})
				if ok {
					tags := []string{}
					for _, tag := range tagsAsArray {
						tagsAsString, ok := tag.(string)
						if ok {
							tags = append(tags, tagsAsString)
						}
					}
					preTags = strings.Join(tags, "")
				}
			}
		}
		if config["post_tags"] != nil {
			tagsAsString, ok := config["post_tags"].(string)
			if ok {
				postTags = tagsAsString
			} else {
				tagsAsArray, ok := config["post_tags"].([]interface{})
				if ok {
					tags := []string{}
					for _, tag := range tagsAsArray {
						tagsAsString, ok := tag.(string)
						if ok {
							tags = append(tags, tagsAsString)
						}
					}
					postTags = strings.Join(tags, "")
				}
			}
		}
	}

	return Tags{
		PreTags:  preTags,
		PostTags: postTags,
	}
}

// getDefaultSuggestionsHighlight returns default suggestion highlight settings, i.e. fields
// and tags to apply based on the highlightField and dataField
// properties
func getDefaultSuggestionsHighlight(query Query) map[string]interface{} {
	highlightFields := make(map[string]interface{})
	fields := query.HighlightField
	if len(fields) == 0 {
		// use data fields as highlighted field
		dataFields := NormalizedDataFields(query.DataField, []float64{})
		for _, v := range dataFields {
			fields = append(fields, v.Field)
		}
	}
	for _, v := range fields {
		highlightFields[v] = make(map[string]interface{})
	}
	return map[string]interface{}{
		"fields":    highlightFields,
		"pre_tags":  preTags,
		"post_tags": postTags,
	}
}

// getPredictiveSuggestions creates predictive suggestions based on the default suggestions
// What are predictive suggestions? Instead of displaying the entire field value as a suggestion (value may be too long to fit into the searchbox), they only display the next relevant words, preferrably as a suffix or as a prefix
func getPredictiveSuggestions(config SuggestionsConfig, suggestions *[]SuggestionHIT) []SuggestionHIT {
	var suggestionsList = make([]SuggestionHIT, 0)
	var suggestionsMap = make(map[string]bool)
	var language = "english"
	if config.Language != nil {
		language = *config.Language
	}
	if config.Value != "" {
		tags := getPredictiveSuggestionsTags(config.HighlightConfig)
		for _, suggestion := range *suggestions {
			fieldValues := strings.Split(normalizeValue(GetTextFromHTML(suggestion.Label)), " ")
			stemmedFvls := stemmedTokens(strings.Join(fieldValues, " "), language)
			fvl := len(fieldValues)
			normQuery := normalizeValue(config.Value)
			queryValues := strings.Split(normQuery, " ")
			// remove stopwords as long as query itself isn't completely removed
			removedStopwords := removeStopwords(normQuery, config)
			if removedStopwords != "" {
				queryValues = strings.Split(removedStopwords, " ")
			}
			stemmedQvls := stemmedTokens(strings.Join(queryValues, " "), language)
			suffixStarts := 0
			prefixEnds := max(fvl-1, 0)
			// helpful for debugging
			// fmt.Println("predictive suggestion: ", ", query is: ", config.Value, ", field value is: ", strings.Join(fieldValues, " "))
			// fmt.Println("stemmed fvl: ", stemmedFvls, ", stemmed query: ", stemmedQvls)
			for _, qToken := range stemmedQvls {
				matchIndex := sliceIndex(fvl, func(i int) bool { return strings.Contains(stemmedFvls[i], qToken) })
				if matchIndex != -1 {
					prefixEnds = min(prefixEnds, matchIndex-1)
					suffixStarts = max(suffixStarts, matchIndex+1)
				}
			}
			var matched = false
			maxPredictedWords := 2
			if config.MaxPredictedWords != nil {
				maxPredictedWords = *config.MaxPredictedWords
			}
			// helpful for debugging
			// fmt.Println("prefix ends: ", prefixEnds)
			// fmt.Println("suffix starts: ", suffixStarts)

			if suffixStarts > 0 {
				highlightPhrase := getHighlightedPhrase(strings.Join(fieldValues[suffixStarts:], " "), max(maxPredictedWords, 1), config)
				// ignore if highlightPhrase contains any of the query tokens
				stemmedHighlightPhrase := strings.Join(stemmedTokens(highlightPhrase, language), " ")
				ignore := false
				for _, qToken := range stemmedQvls {
					if strings.Contains(stemmedHighlightPhrase, qToken) {
						ignore = true
					}
				}
				if !ignore && len(highlightPhrase) > 0 {
					// helpful for debugging
					// fmt.Println("in suffix: highlighted phrase is: ", highlightPhrase)
					matchQuery := config.Value
					suggestionPhrase := fmt.Sprintf("%s %s%s%s", matchQuery, tags.PreTags, highlightPhrase, tags.PostTags)
					suggestionValue := matchQuery + " " + highlightPhrase
					// transform diacritics chars when comparing for uniqueness of predictive suggestions
					if !suggestionsMap[CompressAndOrder(suggestionValue, config)] {
						predictiveSuggestion := suggestion
						predictiveSuggestion.Label = suggestionPhrase
						predictiveSuggestion.Value = suggestionValue
						suggestionsList = append(suggestionsList, predictiveSuggestion)
						// update map
						suggestionsMap[CompressAndOrder(suggestionValue, config)] = true
						matched = true
					}
				}
			}
			if prefixEnds >= 0 && !matched {
				highlightPhrase := getHighlightedPhrase(strings.Join(fieldValues[:prefixEnds+1], " "), max(maxPredictedWords, 1), config)
				// ignore if highlightPhrase contains any of the query tokens
				stemmedHighlightPhrase := strings.Join(stemmedTokens(highlightPhrase, language), " ")
				ignore := false
				for _, qToken := range stemmedQvls {
					if strings.Contains(stemmedHighlightPhrase, qToken) {
						ignore = true
					}
				}
				// helpful for debugging
				// fmt.Println("in prefix: highlighted phrase is: ", highlightPhrase, ", ignore: ", ignore)
				if !ignore && len(highlightPhrase) > 0 {
					matchQuery := config.Value
					suggestionPhrase := tags.PreTags + highlightPhrase + tags.PostTags + " " + matchQuery
					suggestionValue := highlightPhrase + " " + matchQuery
					// transform diacritics chars when comparing for uniqueness of predictive suggestions
					if !suggestionsMap[CompressAndOrder(suggestionValue, config)] {
						predictiveSuggestion := suggestion
						predictiveSuggestion.Label = suggestionPhrase
						predictiveSuggestion.Value = suggestionValue
						suggestionsList = append(suggestionsList, predictiveSuggestion)
						// update map
						suggestionsMap[CompressAndOrder(suggestionValue, config)] = true
						matched = true
					}
				}
			}
		}
	}
	return suggestionsList
}

// getHighlightedPhrase takes a candidate phrase (prefix or suffix) of the field value based on the match found with the search query.
// It returns up to maxTokens length of phrase ignoring for any stopwords in between
func getHighlightedPhrase(candidateWord string, maxTokens int, config SuggestionsConfig) string {
	var wordsPhrase []string
	if config.ApplyStopwords != nil && *config.ApplyStopwords {
		wordsPhrase = strings.Split(removeStopwords(candidateWord, config), " ")
	} else {
		wordsPhrase = strings.Split(candidateWord, " ")
	}
	if len(wordsPhrase) > maxTokens {
		return strings.TrimSpace(strings.Join(wordsPhrase[:maxTokens], " "))
	}
	return strings.TrimSpace(strings.Join(wordsPhrase, " "))
}
