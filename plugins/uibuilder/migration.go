package uibuilder

import (
	"context"
	"encoding/json"
	"os"
	"strings"

	"github.com/appbaseio/reactivesearch-api/util"
	"github.com/google/uuid"
	es7 "github.com/olivere/elastic/v7"
	log "github.com/sirupsen/logrus"
)

type UIBuilderPreferencesMigration struct {
	newIndex string
	oldIndex string
}

func (m UIBuilderPreferencesMigration) ConditionCheck() (bool, *util.Error) {
	errorMsg := `Error occurred while checking condition for uibuilder preferences. 
	Try restarting once if it doesn't fix the issue then please contact us by opening an issue on the GitHub repository.`
	// Only run migration script when `uibuilder-preferences` index exists (old index)
	preferenceIndex := os.Getenv(envEcommPreferencesIndex)
	if preferenceIndex == "" {
		preferenceIndex = defaultEcommPreferencesIndex
	}
	// check if index exists
	exists, err := util.GetClient7().IndexExists(preferenceIndex).Do(context.Background())
	if err != nil {
		log.Errorln(logTag, ":", err)
		return false, &util.Error{
			Message: errorMsg,
			Err:     err,
		}
	}
	// if index exists then sync the preferences to the uibuilder_preferences index
	// Delete the meta index after populating the uibuilder preferences
	if exists {
		return true, nil
	}
	return false, nil
}

func (m UIBuilderPreferencesMigration) Script() *util.Error {
	log.Println(logTag, "Running migration script for uibuilder preferences....This process may take some time.")
	errorMsg := `Error occurred while updating the uibuilder preferences.
	Try restarting once if it doesn't fix the issue then please contact us by opening an issue on the GitHub repository.`
	// Get preferences from old index
	response, err := util.GetClient7().Search(m.oldIndex).Size(10000).
		Do(context.Background())
	if err != nil {
		log.Errorln(logTag, ":", err)
		return &util.Error{
			Message: errorMsg,
			Err:     err,
		}
	}

	bulkRequest := util.GetClient7().Bulk()

	hasPreferences := false

	for _, hit := range response.Hits.Hits {
		if strings.HasPrefix(hit.Id, "recommendations_") {
			// recommendation preference
			var preference RecommendationsPreferences
			err := json.Unmarshal(hit.Source, &preference)
			if err != nil {
				log.Errorln(logTag, ":", err)
				return &util.Error{
					Message: errorMsg,
					Err:     err,
				}
			}
			splitedId := strings.Split(hit.Id, "recommendations_")
			if len(splitedId) > 1 {
				index := splitedId[1]
				preferenceToSave := RecommendationPreferenceRequest{
					ID:                     uuid.New().String(),
					Name:                   "Recommendations " + index,
					Pipeline:               index,
					Type:                   Recommendation,
					ThemeSettings:          preference.ThemeSettings,
					GlobalSettings:         preference.GlobalSettings,
					ResultSettings:         preference.ResultSettings,
					ExportSettings:         preference.ExportSettings,
					RecommendationSettings: preference.RecommendationSettings,
				}
				br := es7.NewBulkIndexRequest().
					Index(m.newIndex).
					Id(preferenceToSave.ID).
					Doc(preferenceToSave)
				bulkRequest.Add(br)
				hasPreferences = true
			}
		} else if strings.HasPrefix(hit.Id, "search_") {
			// search preference
			var preference SearchPreferences
			err := json.Unmarshal(hit.Source, &preference)
			if err != nil {
				log.Errorln(logTag, ":", err)
				return &util.Error{
					Message: errorMsg,
					Err:     err,
				}
			}
			splitedId := strings.Split(hit.Id, "search_")
			if len(splitedId) > 1 {
				index := splitedId[1]
				preferenceToSave := SearchPreferenceRequest{
					ID:                uuid.New().String(),
					Name:              "Search " + index,
					Pipeline:          index,
					Type:              Search,
					ThemeSettings:     preference.ThemeSettings,
					FusionSettings:    preference.FusionSettings,
					ComponentSettings: preference.ComponentSettings,
					PageSettings:      preference.PageSettings,
					GlobalSettings:    preference.GlobalSettings,
					ResultSettings:    preference.ResultSettings,
					ExportSettings:    preference.ExportSettings,
					SearchSettings:    preference.SearchSettings,
					FacetSettings:     preference.FacetSettings,
					SyncSettings:      preference.SyncSettings,
				}
				br := es7.NewBulkIndexRequest().
					Index(m.newIndex).
					Id(preferenceToSave.ID).
					Doc(preferenceToSave)
				bulkRequest.Add(br)
				hasPreferences = true
			}
		}
	}

	// call bulk API
	if hasPreferences {
		_, err3 := bulkRequest.Do(context.Background())
		if err3 != nil {
			log.Errorln(logTag, ":", err3)
			return &util.Error{
				Message: errorMsg,
				Err:     err3,
			}
		}
	}

	// delete old preferences index
	_, err4 := util.GetClient7().DeleteIndex(m.oldIndex).Do(context.Background())
	if err4 != nil {
		log.Errorln(logTag, ":", err4)
		return &util.Error{
			Message: errorMsg,
			Err:     err4,
		}
	}
	return nil
}

func (m UIBuilderPreferencesMigration) IsAsync() bool {
	return false
}
