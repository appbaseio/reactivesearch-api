package suggestions

import (
	"context"
	"encoding/json"
	"os"

	"github.com/appbaseio-confidential/reactivesearch/util"
	es7 "github.com/olivere/elastic/v7"
	log "github.com/sirupsen/logrus"
)

type SuggestionsPreferencesMigration struct {
	es *elasticsearch
}

type suggestionsMetaResponse struct {
	MinCount            int64                `json:"min_count"`
	MinChars            int64                `json:"min_chars"`
	MinHits             int64                `json:"min_hits"`
	NumberOfDays        int64                `json:"number_of_days"`
	Blacklist           []string             `json:"blacklist"`
	Indices             []string             `json:"indices"`
	ExternalSuggestions []ExternalSuggestion `json:"external_suggestions"`
	LastSyncTime        int64                `json:"last_synced_time"`
	TransformDiacritics bool                 `json:"transform_diacritics"`
}

func (m SuggestionsPreferencesMigration) ConditionCheck() (bool, *util.Error) {
	errorMsg := `Error occurred while checking condition for suggestions preferences. 
	Try restarting once if it doesn't fix the issue then please contact us by dropping a mail at support@appbase.io.`
	// Only run migration script when suggestions preferences are stored in the meta index
	oldSuggestionsPreferencesIndex := os.Getenv(envSuggestionsMetaEsIndex)
	if oldSuggestionsPreferencesIndex == "" {
		oldSuggestionsPreferencesIndex = defaultSuggestionsMetaEsIndex
	}
	// check if index exists
	exists, err := util.GetClient7().IndexExists(oldSuggestionsPreferencesIndex).Do(context.Background())
	if err != nil {
		log.Errorln(logTag, ":", err)
		return false, &util.Error{
			Message: errorMsg,
			Err:     err,
		}
	}
	// if index exists then sync the properties to the suggestions preferences index
	// Delete the meta index after populating the suggestions preferences
	if exists {
		return true, nil
	}
	return false, nil
}

func (m SuggestionsPreferencesMigration) Script() *util.Error {
	log.Println(logTag, "Running migration script for suggestions preferences....This process may take some time.")
	errorMsg := `Error occurred while updating the suggestions preferences.
	Try restarting once if it doesn't fix the issue then please contact us by dropping a mail at support@appbase.io.`
	oldSuggestionsPreferencesIndex := os.Getenv(envSuggestionsMetaEsIndex)
	if oldSuggestionsPreferencesIndex == "" {
		oldSuggestionsPreferencesIndex = defaultSuggestionsMetaEsIndex
	}
	// Get preferences from meta index
	response, err := util.GetClient7().Get().
		Index(oldSuggestionsPreferencesIndex).
		Id(preferenceDocID).
		FetchSource(true).
		Do(context.Background())
	if err != nil {
		// handle 404 error
		if es7.IsNotFound(err) {
			return nil
		}
		log.Errorln(logTag, ":", err)
		return &util.Error{
			Message: errorMsg,
			Err:     err,
		}
	}
	var preferences suggestionsMetaResponse
	err2 := json.Unmarshal(response.Source, &preferences)
	if err2 != nil {
		log.Errorln(logTag, ":", err2)
		return &util.Error{
			Message: errorMsg,
			Err:     err2,
		}
	}
	record := PopularPreferences{
		MinCount:            preferences.MinCount,
		MinChars:            preferences.MinChars,
		MinHits:             preferences.MinHits,
		NumberOfDays:        preferences.NumberOfDays,
		Blacklist:           preferences.Blacklist,
		Indices:             preferences.Indices,
		ExternalSuggestions: preferences.ExternalSuggestions,
		LastSyncTime:        preferences.LastSyncTime,
	}
	_, err3 := m.es.savePopularSuggestionsPreferences(context.Background(), record)
	if err3 != nil {
		log.Errorln(logTag, ":", err3)
		return &util.Error{
			Message: errorMsg,
			Err:     err3,
		}
	}
	// update local cache
	SetPopularPreferences(record)
	// delete suggestions meta index
	_, err4 := util.GetClient7().DeleteIndex(oldSuggestionsPreferencesIndex).Do(context.Background())
	if err4 != nil {
		log.Errorln(logTag, ":", err4)
		return &util.Error{
			Message: errorMsg,
			Err:     err4,
		}
	}
	return nil
}

func (m SuggestionsPreferencesMigration) IsAsync() bool {
	return false
}
