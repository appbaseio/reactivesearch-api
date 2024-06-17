package suggestions

import (
	"fmt"
	"net/http"
	"strings"
	"time"
	"unicode"

	"github.com/appbaseio-confidential/reactivesearch/util"
)

// returns ${index}_${timestamp} where timestamp is at a day's resolution
func getTimestampedIndex(indexPrefix string, timestamp time.Time) string {
	return indexPrefix + "_" + strings.ToLower(timestamp.UTC().Format("Jan_02"))
}

// Contains tells whether a contains x.
func Contains(a []string, x string) bool {
	for _, n := range a {
		if x == n {
			return true
		}
	}
	return false
}

func remove(slice []string, s string) []string {
	newSlice := []string{}
	for _, n := range slice {
		if s != n {
			newSlice = append(newSlice, n)
		}
	}
	return newSlice
}

func isMn(r rune) bool {
	return unicode.Is(unicode.Mn, r) // Mn: nonspacing marks
}

// preferences for index type of suggestions
type IndexPreferences struct {
	Indices                     []string `json:"indices"`
	ShowDistinctSuggestions     *bool    `json:"showDistinctSuggestions"`
	EnablePredictiveSuggestions *bool    `json:"enablePredictiveSuggestions"`
	MaxPredictedWords           *int     `json:"maxPredictedWords"`
	Size                        *int     `json:"size"`
	ApplyStopwords              *bool    `json:"applyStopwords"`
	CustomStopwords             []string `json:"customStopwords"`
	EnableSynonyms              *bool    `json:"enableSynonyms"`
	IncludeFields               []string `json:"includeFields"`
	ExcludeFields               []string `json:"excludeFields"`
	CategoryField               *string  `json:"categoryField"`
	URLField                    *string  `json:"urlField"`
	CustomQuery                 *string  `json:"customQuery"`
}

// preferences for popular type of suggestions
type PopularPreferences struct {
	Size                int                  `json:"size"`
	MinCount            int64                `json:"minCount"`
	MinChars            int64                `json:"minChars"`
	MinHits             int64                `json:"minHits"`
	NumberOfDays        int64                `json:"numberOfDays"`
	Blacklist           []string             `json:"blacklist"`
	Indices             []string             `json:"indices"`
	ExternalSuggestions []ExternalSuggestion `json:"externalSuggestions"`
	TransformDiacritics bool                 `json:"transformDiacritics"`
	LastSyncTime        int64                `json:"lastSyncedTime"`
	AliasToIndex        string               `json:"aliasToIndex"`
}

// preferences for recent type of suggestions
type RecentPreferences struct {
	Indices  []string `json:"indices"`
	MinHits  *int     `json:"minHits"`
	MinChars *int     `json:"minChars"`
	Size     *int     `json:"size"`
}

// checkIfZincIndexExists will check if the passed zinc index
// exists and accordingly handle errors and such.
func checkIfZincIndexExists(index string) (bool, error) {
	zincClient := util.GetZincClient()

	// Check if the index already exists
	// Make a request to the get settings endpoint of Zinc
	// and check if the status code is 200 to know if it exists
	// or not.
	existsEndpointZinc := fmt.Sprintf("api/%s/_settings", index)
	existsResponse, existsResponseErr := zincClient.MakeRequest(existsEndpointZinc, http.MethodGet, []byte(""), nil)

	if existsResponseErr != nil {
		return false, fmt.Errorf("error while checking if index already exists: %v", existsResponseErr)
	}

	if existsResponse == nil || existsResponse.StatusCode == http.StatusOK {
		return true, nil
	}

	return false, nil
}
