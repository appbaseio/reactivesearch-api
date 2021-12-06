package querytranslate

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"github.com/appbaseio/reactivesearch-api/util"
	"github.com/bbalet/stopwords"
	"github.com/kljensen/snowball"
	"github.com/lithammer/fuzzysearch/fuzzy"
	"github.com/microcosm-cc/bluemonday"
	log "github.com/sirupsen/logrus"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

// Do this once for each unique policy, and use the policy for the life of the program
// Policy creation/editing is not safe to use in multiple goroutines
var p = bluemonday.StrictPolicy()

type SuggestionType int

const (
	Index SuggestionType = iota
	Popular
	Recent
	Promoted
)

// String is the implementation of Stringer interface that returns the string representation of SuggestionType type.
func (o SuggestionType) String() string {
	return [...]string{
		"index",
		"popular",
		"recent",
		"promoted",
	}[o]
}

// UnmarshalJSON is the implementation of the Unmarshaler interface for unmarshaling SuggestionType type.
func (o *SuggestionType) UnmarshalJSON(bytes []byte) error {
	var suggestionType string
	err := json.Unmarshal(bytes, &suggestionType)
	if err != nil {
		return err
	}
	switch suggestionType {
	case Index.String():
		*o = Index
	case Popular.String():
		*o = Popular
	case Recent.String():
		*o = Recent
	case Promoted.String():
		*o = Promoted
	default:
		return fmt.Errorf("invalid suggestion type encountered: %v", suggestionType)
	}
	return nil
}

// MarshalJSON is the implementation of the Marshaler interface for marshaling SuggestionType type.
func (o SuggestionType) MarshalJSON() ([]byte, error) {
	var suggestionType string
	switch o {
	case Index:
		suggestionType = Index.String()
	case Popular:
		suggestionType = Popular.String()
	case Recent:
		suggestionType = Recent.String()
	case Promoted:
		suggestionType = Promoted.String()
	default:
		return nil, fmt.Errorf("invalid suggestion type encountered: %v", o)
	}
	return json.Marshal(suggestionType)
}

// SuggestionHIT represents the structure of the suggestion object in RS API response
type SuggestionHIT struct {
	Value         string         `json:"value"`
	Label         string         `json:"label"`
	URL           *string        `json:"url"`
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
	Size     *int    `json:"size,omitempty"`
	Index    *string `json:"index,omitempty"`
	MinHits  *int    `json:"minHits,omitempty"`
	MinChars *int    `json:"minChars,omitempty"`
}

// PopularSuggestionsOptions represents the options to configure popular suggestions
type PopularSuggestionsOptions struct {
	Size       *int    `json:"size,omitempty"`
	Index      *string `json:"index,omitempty"`
	ShowGlobal *bool   `json:"showGlobal,omitempty"`
	MinChars   *int    `json:"minChars,omitempty"`
	MinCount   *int    `json:"minCount,omitempty"`
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
}

func stemmedTokens(source string, language string) []string {
	tokens := strings.Split(source, " ")
	var stemmedTokens []string
	for _, token := range tokens {
		// stem the token
		stemmedToken, _ := snowball.Stem(token, language, false)
		stemmedTokens = append(stemmedTokens, stemmedToken)
	}
	return stemmedTokens
}

// removeStopwords removes stopwords including considering the suggestions config
func removeStopwords(value string, config SuggestionsConfig) string {
	ln := "en"
	if config.Language != nil && LanguagesToISOCode[*config.Language] != "" {
		ln = LanguagesToISOCode[*config.Language]
	}
	var userStopwords []string
	// load any custom stopwords the user has
	// a highlighted phrase shouldn't be limited due to stopwords
	if config.ApplyStopwords != nil && *config.ApplyStopwords {
		// apply any custom stopwords
		if config.Stopwords != nil {
			userStopwords = *config.Stopwords
		}
	}
	if len(userStopwords) > 0 {
		stopwords.LoadStopWordsFromString(strings.Join(userStopwords, " "), ln, " ")
	}
	cleanContent := stopwords.CleanString(value, ln, true)
	return NormalizeValue(cleanContent)
}

// NormalizeValue changes a query's value to remove special chars and spaces
// e.g. Android - Black would be "android black"
// e.g. "Wendy's burger  " would be "wendy burger"
func NormalizeValue(value string) string {
	// Trim the spaces and tokenize
	tokenizedValue := strings.Split(strings.TrimSpace(value), " ")
	var finalValue []string
	for _, token := range tokenizedValue {
		sT := SanitizeString(token)
		if len(sT) > 0 {
			finalValue = append(finalValue, strings.ToLower(sT))
		}
	}
	return strings.ToLower(strings.TrimSpace(strings.Join(finalValue, " ")))
}

// findMatch matches the user query against the field value to return scores and matched tokens
func findMatch(fieldValueRaw string, userQueryRaw string, config SuggestionsConfig) RankField {
	// remove stopwords from fieldValue and userQuery
	fieldValue := removeStopwords(fieldValueRaw, config)
	userQuery := removeStopwords(userQueryRaw, config)
	var rankField = RankField{
		fieldValue:    fieldValue,
		userQuery:     userQuery,
		score:         0,
		matchedTokens: nil,
	}
	stemLanguage := "english"
	if config.Language != nil {
		if util.Contains(StemLanguages, *config.Language) {
			stemLanguage = *config.Language
		}
	}
	fieldValues := strings.Split(fieldValue, " ")
	stemmedFieldValues := stemmedTokens(fieldValue, stemLanguage)
	stemmeduserQuery := stemmedTokens(userQuery, stemLanguage)
	foundMatches := make([]bool, len(stemmeduserQuery))
	for i, token := range stemmeduserQuery {

		// eliminate single char tokens from consideration
		if len(token) > 1 {
			foundMatch := false
			// start with the default distance of 1.0
			bestDistance := 1.0
			ranks := fuzzy.RankFindNormalizedFold(token, stemmedFieldValues)
			var bestTarget string
			for _, element := range ranks {
				switch element.Distance {
				case 0:
					// Perfect match, we can skip iteration and just return
					bestDistance = math.Min(0, bestDistance)
					foundMatch = true
					bestTarget = element.Target
				case 1:
					// 1 edit distance
					bestDistance = math.Min(1.0, bestDistance)
					foundMatch = true
					if bestTarget == "" {
						bestTarget = element.Target
					}
				}
			}
			matchIndex := sliceIndex(len(stemmedFieldValues), func(i int) bool {
				return stemmedFieldValues[i] == bestTarget
			})
			if matchIndex != -1 {
				rankField.matchedTokens = append(rankField.matchedTokens, fieldValues[matchIndex])
			}
			foundMatches[i] = foundMatch
			// token of user query matched one of the tokens of field values
			if foundMatch {
				rankField.score += 1.0 - (bestDistance / 2)
				// add score for a consecutive match
				if i > 0 {
					if foundMatches[i] && foundMatches[i-1] {
						rankField.score += 0.1
					}
				}
			}
		}
	}
	return rankField
}

const preTags = `<b class="highlight">`
const postTags = `</b>`

type Tags struct {
	PreTags  string
	PostTags string
}

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

func isMn(r rune) bool {
	return unicode.Is(unicode.Mn, r) // Mn: nonspacing marks
}

// replaces diacritics with their equivalent
func replaceDiacritics(query string) string {
	t := transform.Chain(norm.NFD, transform.RemoveFunc(isMn), norm.NFC)
	queryKey, _, _ := transform.String(t, query)
	return queryKey
}

// populateDefaultSuggestions populates the default (i.e. non-predictive) suggestions using field values of each doc
func populateDefaultSuggestions(
	labelsList *[]string,
	suggestionsList *[]SuggestionHIT,
	docField DocField,
	config SuggestionsConfig,
) {
	if !util.Contains(*labelsList, removeStopwords(ParseSuggestionLabel(docField.value, config.Language), config)) {
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
		fieldValue := NormalizeValue(getTextFromHTML(docField.value))
		// TODO: This won't work on query synonyms, need to account for that
		rankField := findMatch(fieldValue, config.Value, config)
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
			Id:     docField.rawHit.Id,
			Index:  &docField.rawHit.Index,
			Source: docField.rawHit.Source,
			Score:  docField.rawHit.Score,
		}

		*labelsList = append(*labelsList, removeStopwords(ParseSuggestionLabel(docField.value, config.Language), config))
		*suggestionsList = append(*suggestionsList, suggestion)
	}
}

// extracts the string from HTML tags
func getTextFromHTML(body string) string {

	// The policy can then be used to sanitize lots of input and it is safe to use the policy in multiple goroutines
	html := p.Sanitize(
		body,
	)

	return html
}

// getPredictiveSuggestions creates predictive suggestions based on the default suggestions
// What are predictive suggestions? Instead of displaying the entire field value as a suggestion (value may be too long to fit into the searchbox), they only display the next relevant words, preferrably as a suffix or as a prefix
func getPredictiveSuggestions(config SuggestionsConfig, suggestions *[]SuggestionHIT) []SuggestionHIT {
	var suggestionsList = make([]SuggestionHIT, 0)
	var suggestionsMap = make(map[string]bool)
	if config.Value != "" {
		tags := getPredictiveSuggestionsTags(config.HighlightConfig)
		for _, suggestion := range *suggestions {
			fieldValues := strings.Split(NormalizeValue(suggestion.Label), " ")
			fvl := len(fieldValues)
			queryValues := strings.Split(NormalizeValue(config.Value), " ")
			suffixStarts := 0
			prefixEnds := max(fvl-1, 0)
			// helpful for debugging
			// fmt.Println("predictive suggestion: ", ", query is: ", config.Value, ", field value is: ", strings.Join(fieldValues, " "))
			for _, qToken := range queryValues {
				matchIndex := sliceIndex(fvl, func(i int) bool { return strings.Contains(fieldValues[i], qToken) })
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
				// highlightPhrase can additionally not contain any matched tokens as they would duplicate
				hltValues := strings.Split(highlightPhrase, " ")
				ignore := false
				for _, qToken := range queryValues {
					if sliceIndex(len(hltValues), func(i int) bool { return hltValues[i] == qToken }) != -1 {
						ignore = true
					}
				}
				if !ignore && len(highlightPhrase) > 1 {
					// helpful for debugging
					// fmt.Println("in suffix: highlighted phrase is: ", highlightPhrase)
					var matchQuery string
					// case where we replace the matching field value query in place of the actual query
					if sliceIndex(len(queryValues), func(i int) bool { return strings.Contains(fieldValues[prefixEnds+1], queryValues[i]) }) != -1 {
						matchQuery = strings.Join(fieldValues[prefixEnds+1:suffixStarts], " ")
					} else {
						matchQuery = config.Value
					}
					suggestionPhrase := matchQuery + " " + tags.PreTags + highlightPhrase + tags.PostTags
					suggestionValue := matchQuery + " " + highlightPhrase
					// transform diacritics chars when comparing for uniqueness of predictive suggestions
					if !suggestionsMap[replaceDiacritics(suggestionValue)] {
						predictiveSuggestion := suggestion
						predictiveSuggestion.Label = suggestionPhrase
						predictiveSuggestion.Value = suggestionValue
						suggestionsList = append(suggestionsList, predictiveSuggestion)
						// update map
						suggestionsMap[replaceDiacritics(suggestionValue)] = true
						matched = true
					}
				}
			}
			if prefixEnds >= 0 && !matched {
				highlightPhrase := getHighlightedPhrase(strings.Join(fieldValues[:prefixEnds+1], " "), max(maxPredictedWords, 1), config)
				// highlightPhrase can additionally not contain any matched tokens as they would duplicate
				hltValues := strings.Split(highlightPhrase, " ")
				ignore := false
				for _, qToken := range queryValues {
					if sliceIndex(len(hltValues), func(i int) bool { return hltValues[i] == qToken }) != -1 {
						ignore = true
					}
				}
				// helpful for debugging
				// fmt.Println("in prefix: highlighted phrase is: ", highlightPhrase)
				if !ignore && len(highlightPhrase) > 1 {
					var matchQuery string
					// case where we replace the matching field value query in place of the actual query
					if suffixStarts > 0 && sliceIndex(len(queryValues), func(i int) bool { return strings.Contains(fieldValues[suffixStarts-1], queryValues[i]) }) != -1 {
						matchQuery = strings.Join(fieldValues[prefixEnds+1:suffixStarts], " ")
					} else {
						matchQuery = config.Value
					}
					suggestionPhrase := tags.PreTags + highlightPhrase + tags.PostTags + " " + matchQuery
					suggestionValue := highlightPhrase + " " + matchQuery
					matched = true
					// transform diacritics chars when comparing for uniqueness of predictive suggestions
					if !suggestionsMap[replaceDiacritics(suggestionValue)] {
						predictiveSuggestion := suggestion
						predictiveSuggestion.Label = suggestionPhrase
						predictiveSuggestion.Value = suggestionValue
						suggestionsList = append(suggestionsList, predictiveSuggestion)
						// update map
						suggestionsMap[replaceDiacritics(suggestionValue)] = true
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

// Highlights the fields by replacing the actual value with markup
func highlightResults(source ESDoc) ESDoc {
	source.ParsedSource = make(map[string]interface{})
	// clone map
	for k, v := range source.Source {
		source.ParsedSource[k] = v
	}

	if source.Highlight != nil {
		for highlightItem, highlightedValue := range source.Highlight {
			highlightValueArray, ok := highlightedValue.([]interface{})
			if ok && len(highlightValueArray) > 0 {
				highlightValue := highlightValueArray[0]
				source.ParsedSource[highlightItem] = highlightValue
			}
		}
	}
	return source
}

// To parse the elasticsearch hits with highlighted fields
func parseHits(hits []ESDoc) []ESDoc {
	var results = make([]ESDoc, 0)
	for _, hit := range hits {
		results = append(results, highlightResults(hit))
	}
	return results
}

// Removes the punctuation from a string
func strip(s string) string {
	var result strings.Builder
	for i := 0; i < len(s); i++ {
		b := s[i]
		if ('a' <= b && b <= 'z') ||
			('A' <= b && b <= 'Z') ||
			('0' <= b && b <= '9') ||
			b == ' ' {
			result.WriteByte(b)
		}
	}
	return result.String()
}

// Util method to extract the fields from elasticsearch source object
// It can handle nested objects and arrays too.
// Example 1:
// Input: { a: 1, b: { b_1: 2, b_2: 3}}
// Output: ['a', 'b.b_1', 'b.b_2']
// Example 2:
// Input: { a: 1, b: [{c: 1}, {d: 2}, {c: 3}]}
// Output: ['a', 'b.c', 'b.d']
func getFields(source interface{}, prefix string) map[string]interface{} {
	dataFields := make(map[string]interface{})
	sourceAsMap, ok := source.(map[string]interface{})
	if ok {
		for field := range sourceAsMap {
			var key string
			if prefix != "" {
				key = prefix + "." + field
			} else {
				key = field
			}
			if sourceAsMap[field] != nil {
				mapValue, ok := sourceAsMap[field].(map[string]interface{})
				if ok {
					mergeMaps(dataFields, getFields(mapValue, key))
				} else {
					mapValueAsArray, ok := sourceAsMap[field].([]interface{})
					if ok {
						mergeMaps(dataFields, getFields(mapValueAsArray, key))
					} else {
						mergeMaps(dataFields, map[string]interface{}{
							key: true,
						})
					}
				}
			}
		}
	} else {
		sourceAsArray, ok := source.([]interface{})
		if ok {
			for field := range sourceAsArray {
				var key string
				if prefix != "" {
					key = prefix
				} else {
					key = strconv.Itoa(field)
				}
				if sourceAsArray[field] != nil {
					mapValue, ok := sourceAsArray[field].(map[string]interface{})
					if ok {
						mergeMaps(dataFields, getFields(mapValue, key))
					} else {
						mapValueAsArray, ok := sourceAsArray[field].([]interface{})
						if ok {
							mergeMaps(dataFields, getFields(mapValueAsArray, key))
						} else {
							mergeMaps(dataFields, map[string]interface{}{
								key: true,
							})
						}
					}
				}
			}
		}
	}

	return dataFields
}

func extractFieldsFromSource(source map[string]interface{}) []string {
	dataFields := []string{}
	var sourceAsInterface interface{} = source
	dataFieldsMap := getFields(sourceAsInterface, "")
	for k := range dataFieldsMap {
		dataFields = append(dataFields, k)
	}
	return dataFields
}

// getIndexSuggestions gets the index suggestions based on user query config and search engine response
func getIndexSuggestions(config SuggestionsConfig, rawHits []ESDoc) []SuggestionHIT {
	// before parsing any suggestions, normalize the query
	config.Value = NormalizeValue(config.Value)
	// set priority to highlight fields if present
	if len(config.HighlightField) != 0 {
		config.DataFields = config.HighlightField
	} else if len(config.DataFields) == 0 && len(rawHits) > 0 {
		// extract fields from first hit source
		config.DataFields = extractFieldsFromSource(rawHits[0].Source)
	}

	// parse hits
	parsedHits := parseHits(rawHits)
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

// Removes the extra spaces from a string
func RemoveSpaces(str string) string {
	return strings.Join(strings.Fields(str), " ")
}

// SanitizeString removes special chars and spaces from a string
func SanitizeString(str string) string {
	// remove extra spaces
	s := str
	specialChars := []string{"'", "/", "{", "(", "[", "-", "+", ".", "^", ":", ",", "]", ")",
		"}"}
	// Remove special characters
	for _, c := range specialChars {
		s = strings.ReplaceAll(s, c, "")
	}
	return RemoveSpaces(s)
}

// Returns the parsed suggestion label to be compared for duplicate suggestions
func ParseSuggestionLabel(label string, language *string) string {
	// trim spaces
	parsedLabel := RemoveSpaces(label)
	// convert to lower case
	parsedLabel = strings.ToLower(parsedLabel)
	stemLanguage := "english"
	if language != nil {
		if util.Contains(StemLanguages, *language) {
			stemLanguage = *language
		}
	}
	// stem word
	stemmed, err := snowball.Stem(parsedLabel, stemLanguage, true)
	if err != nil {
		log.Errorln(logTag, ":", err)
	} else {
		parsedLabel = stemmed
	}
	// remove stopwords
	return RemoveSpaces(parsedLabel)
}
