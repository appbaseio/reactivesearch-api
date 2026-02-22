package uibuilder

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/appbaseio/reactivesearch-api/util"
	es7 "github.com/olivere/elastic/v7"
	log "github.com/sirupsen/logrus"
)

func initPlugin(indexName, mapping string) (*elasticsearch, error) {
	es := &elasticsearch{indexName}

	ctx := context.Background()

	// Check if the preferences index already exists
	exists, err := util.GetClient7().IndexExists(indexName).Do(ctx)
	if err != nil {
		return es, fmt.Errorf("error while checking if index already exists: %v", err)
	}
	if exists {
		log.Printf("%s: index named '%s' already exists, skipping...", logTag, indexName)
		return es, nil
	}

	replicas := util.GetReplicas()
	settings := fmt.Sprintf(mapping, util.HiddenIndexSettings(), replicas)

	// Meta index does not exists, create a new one
	_, err = util.GetClient7().CreateIndex(indexName).Body(settings).Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("error while creating index named %s: %v", indexName, err)
	}

	log.Printf("%s successfully created index named '%s'", logTag, indexName)
	return es, nil
}

// createSearchboxIndex creates an index to store custom suggestions
func createSearchBoxIndex(indexName string, indexConfig string) (*elasticsearch, bool, error) {
	ctx := context.Background()
	es := elasticsearch{indexName}
	// Check if the index already exists
	exists, err := util.GetClient7().IndexExists(indexName).Do(ctx)
	if err != nil {
		return nil, false, fmt.Errorf("error while checking if index already exists: %v", err)
	}
	if exists {
		log.Println(logTag, ": index named", indexName, "already exists, skipping...")
		return &es, true, nil
	}

	replicas := util.GetReplicas()

	settings := fmt.Sprintf(indexConfig, util.HiddenIndexSettings(), replicas)

	// index does not exists, create a new one
	_, err = util.GetClient7().CreateIndex(indexName).Body(settings).Do(ctx)
	if err != nil {
		return &es, false, fmt.Errorf("error while creating index named %s: %v", indexName, err)
	}

	log.Println(logTag, ": successfully created index named", indexName)
	return &es, false, nil
}

// To save the preference for a given ID
func (es *elasticsearch) savePreference(ctx context.Context, id string, record interface{}) error {
	_, err := util.GetClient7().Index().
		Refresh("wait_for").
		Index(es.indexName).
		Id(id).
		BodyJson(record).
		Do(ctx)
	return err
}

// To get the search preference for a given ID
func (es *elasticsearch) getSearchPreference(ctx context.Context, id string) (SearchPreference, error) {
	var preferences SearchPreference
	response, err := util.GetClient7().Get().
		Index(es.indexName).
		Id(id).
		Do(ctx)
	if err != nil {
		log.Errorln(logTag, ": error retrieving preferences", err)
		return preferences, err
	}
	err2 := json.Unmarshal(response.Source, &preferences)
	if err2 != nil {
		log.Errorln(logTag, ": error while un-marshalling the preferences", err2)
		return preferences, err2
	}
	return parseSearchPreference(preferences), nil
}

func parseSearchPreference(preferences SearchPreference) SearchPreference {
	if preferences.IsUsingStringFields {
		// transform new fields (stored as string) back to map
		if preferences.FusionSettings != nil {
			var fusionSettings FusionSettings
			err := json.Unmarshal([]byte(*preferences.FusionSettings), &fusionSettings)
			if err == nil {
				preferences.FusionSettingsStruct = &fusionSettings
				preferences.FusionSettings = nil
			} else {
				log.Errorln(logTag, ":", err)
			}
		}
		if preferences.DeploySettings != nil {
			var deploySettings map[string]interface{}
			err := json.Unmarshal([]byte(*preferences.DeploySettings), &deploySettings)
			if err == nil {
				preferences.DeploySettingsStruct = &deploySettings
				preferences.DeploySettings = nil
			} else {
				log.Errorln(logTag, ":", err)
			}
		}
		if preferences.AuthenticationSettings != nil {
			var authSettings map[string]interface{}
			err := json.Unmarshal([]byte(*preferences.AuthenticationSettings), &authSettings)
			if err == nil {
				preferences.AuthenticationSettingsStruct = &authSettings
				preferences.AuthenticationSettings = nil
			} else {
				log.Errorln(logTag, ":", err)
			}
		}
		if preferences.PageSettings != nil {
			var pageSettings map[string]interface{}
			err := json.Unmarshal([]byte(*preferences.PageSettings), &pageSettings)
			if err == nil {
				preferences.PageSettingsStruct = &pageSettings
				preferences.PageSettings = nil
			} else {
				log.Errorln(logTag, ":", err)
			}
		}
		if preferences.ComponentSettings != nil {
			var componentSettings map[string]interface{}
			err := json.Unmarshal([]byte(*preferences.ComponentSettings), &componentSettings)
			if err == nil {
				preferences.ComponentSettingsStruct = &componentSettings
				preferences.ComponentSettings = nil
			} else {
				log.Errorln(logTag, ":", err)
			}
		}
		if preferences.SyncSettings != nil {
			var syncSettings map[string]interface{}
			err := json.Unmarshal([]byte(*preferences.SyncSettings), &syncSettings)
			if err == nil {
				preferences.SyncSettingsStruct = &syncSettings
				preferences.SyncSettings = nil
			} else {
				log.Errorln(logTag, ":", err)
			}
		}
		if preferences.ThemeSettings != nil {
			var settings ThemeSettings
			err := json.Unmarshal([]byte(*preferences.ThemeSettings), &settings)
			if err == nil {
				preferences.ThemeSettingsStruct = &settings
				preferences.ThemeSettings = nil
			} else {
				log.Errorln(logTag, ":", err)
			}
		}
		if preferences.GlobalSettings != nil {
			var settings GlobalSettings
			err := json.Unmarshal([]byte(*preferences.GlobalSettings), &settings)
			if err == nil {
				preferences.GlobalSettingsStruct = &settings
				preferences.GlobalSettings = nil
			} else {
				log.Errorln(logTag, ":", err)
			}
		}
		if preferences.ResultSettings != nil {
			var settings ResultSettings
			err := json.Unmarshal([]byte(*preferences.ResultSettings), &settings)
			if err == nil {
				preferences.ResultSettingsStruct = &settings
				preferences.ResultSettings = nil
			} else {
				log.Errorln(logTag, ":", err)
			}
		}
		if preferences.ExportSettings != nil {
			var settings ExportSettings
			err := json.Unmarshal([]byte(*preferences.ExportSettings), &settings)
			if err == nil {
				preferences.ExportSettingsStruct = &settings
				preferences.ExportSettings = nil
			} else {
				log.Errorln(logTag, ":", err)
			}
		}
		if preferences.SearchSettings != nil {
			var settings SearchSettings
			err := json.Unmarshal([]byte(*preferences.SearchSettings), &settings)
			if err == nil {
				preferences.SearchSettingsStruct = &settings
				preferences.SearchSettings = nil
			} else {
				log.Errorln(logTag, ":", err)
			}
		}
		if preferences.FacetSettings != nil {
			var settings FacetSettings
			err := json.Unmarshal([]byte(*preferences.FacetSettings), &settings)
			if err == nil {
				preferences.FacetSettingsStruct = &settings
				preferences.FacetSettings = nil
			} else {
				log.Errorln(logTag, ":", err)
			}
		}
	}
	return preferences
}

// To get the recommendation preference for a given ID
func (es *elasticsearch) getRecommendationPreference(ctx context.Context, id string) (RecommendationPreference, error) {
	var preferences RecommendationPreference
	response, err := util.GetClient7().Get().
		Index(es.indexName).
		Id(id).
		Do(ctx)
	if err != nil {
		log.Errorln(logTag, ": error retrieving preferences", err)
		return preferences, err
	}
	err2 := json.Unmarshal(response.Source, &preferences)
	if err2 != nil {
		log.Errorln(logTag, ": error while un-marshalling the preferences", err2)
		return preferences, err2
	}
	return preferences, nil
}

func (es *elasticsearch) deletePreference(ctx context.Context, id string) error {
	_, err := util.GetClient7().Delete().
		Refresh("wait_for").
		Index(es.indexName).
		Id(id).
		Do(ctx)
	return err
}

func (es *elasticsearch) getSearchPreferences(ctx context.Context) ([]SearchPreference, error) {
	preferences := make([]SearchPreference, 0)
	query := es7.NewTermQuery("type", Search.String())
	res, err := util.GetClient7().
		Search(es.indexName).
		Query(query).
		SortWithInfo(es7.SortInfo{
			Field:        "updated_at",
			Ascending:    false,
			UnmappedType: "long",
		}).Size(10000).Do(ctx)
	if err != nil {
		return preferences, err
	}
	for _, hit := range res.Hits.Hits {
		var preference SearchPreference
		err := json.Unmarshal(hit.Source, &preference)
		if err != nil {
			return preferences, err
		}
		preferences = append(preferences, parseSearchPreference(preference))
	}
	return preferences, nil
}

func (es *elasticsearch) getRecommendationPreferences(ctx context.Context) ([]RecommendationPreference, error) {
	preferences := make([]RecommendationPreference, 0)
	query := es7.NewTermQuery("type", Recommendation.String())
	res, err := util.GetClient7().
		Search(es.indexName).
		Size(10000).
		SortWithInfo(es7.SortInfo{
			Field:        "updated_at",
			Ascending:    false,
			UnmappedType: "long",
		}).Query(query).Do(ctx)
	if err != nil {
		return preferences, err
	}
	for _, hit := range res.Hits.Hits {
		var preference RecommendationPreference
		err := json.Unmarshal(hit.Source, &preference)
		if err != nil {
			return preferences, err
		}
		preferences = append(preferences, preference)
	}
	return preferences, nil
}

func (es *elasticsearch) saveSearchBox(ctx context.Context, searchboxId string, payload SearchBoxESModel) *Error {
	payloadToProcess := payload
	currentTime := time.Now().Unix()
	payloadToProcess.Id = &searchboxId
	existingRecord, _ := es.getSearchBox(ctx, searchboxId)
	if existingRecord == nil {
		payloadToProcess.CreatedAt = &currentTime
	} else {
		payloadToProcess.CreatedAt = existingRecord.CreatedAt
		payloadToProcess.UpdatedAt = &currentTime
	}
	var record map[string]interface{}
	recordInBytes, err := json.Marshal(payloadToProcess)
	if err != nil {
		return &Error{
			err: err,
		}
	}
	err2 := json.Unmarshal(recordInBytes, &record)
	if err2 != nil {
		return &Error{
			err: err2,
		}
	}
	_, err3 := util.GetClient7().Index().
		Refresh("wait_for").
		Index(es.indexName).
		Id(searchboxId).
		BodyJson(record).
		Do(context.Background())
	if err3 != nil {
		return &Error{
			err: err3,
		}
	}
	return nil
}

func (es *elasticsearch) getSearchBox(ctx context.Context, searchboxId string) (*SearchBoxESModel, *Error) {
	res, err := util.GetClient7().
		Get().
		Id(searchboxId).
		Index(es.indexName).
		Do(ctx)
	if err != nil {
		if es7.IsNotFound(err) {
			return nil, &Error{
				err:  err,
				code: http.StatusNotFound,
			}
		}
		return nil, &Error{
			err: err,
		}
	}
	var searchbox SearchBoxESModel
	err2 := json.Unmarshal(res.Source, &searchbox)
	if err2 != nil {
		return nil, &Error{
			err: err2,
		}
	}
	s := getSearchBoxFromESModel(searchbox)
	return &s, nil
}

func (es *elasticsearch) getSearchBoxes(ctx context.Context, hidden bool) ([]SearchBoxESModel, *Error) {
	var query es7.Query
	query = es7.NewTermsQuery("hidden", hidden)
	if hidden {
		query = es7.NewMatchAllQuery()
	}
	res, err := util.GetClient7().
		Search().
		Query(query).
		Size(10000).
		Index(es.indexName).
		Do(ctx)
	if err != nil {
		return nil, &Error{
			err: err,
		}
	}
	searchboxes := make([]SearchBoxESModel, 0)
	for _, searchbox := range res.Hits.Hits {
		var parsedSearchBox SearchBoxESModel
		err2 := json.Unmarshal(searchbox.Source, &parsedSearchBox)
		if err2 != nil {
			return nil, &Error{
				err: err2,
			}
		}
		searchboxes = append(searchboxes, getSearchBoxFromESModel(parsedSearchBox))
	}

	return searchboxes, nil
}

func getSearchBoxFromESModel(searchboxPreference SearchBoxESModel) SearchBoxESModel {
	if searchboxPreference.SearchBox != nil && searchboxPreference.SearchBox.Endpoint != nil &&
		searchboxPreference.SearchBox.Endpoint.Endpoint != nil {
		if searchboxPreference.SearchBox.Endpoint.Endpoint.IsUsingStringFormat {
			if searchboxPreference.SearchBox.Endpoint.Endpoint.Body != nil {
				var b interface{}
				err := json.Unmarshal([]byte(*searchboxPreference.SearchBox.Endpoint.Endpoint.Body), &b)
				if err != nil {
					log.Errorln(logTag, ":", err)
				}
				(*searchboxPreference.SearchBox.Endpoint.Endpoint).BodyMap = &b
				(*searchboxPreference.SearchBox.Endpoint.Endpoint).Body = nil
			}
			if searchboxPreference.SearchBox.Endpoint.Endpoint.Headers != nil {
				var headers map[string]string
				err := json.Unmarshal([]byte(*searchboxPreference.SearchBox.Endpoint.Endpoint.Headers), &headers)
				if err != nil {
					log.Errorln(logTag, ":", err)
				}
				(*searchboxPreference.SearchBox.Endpoint.Endpoint).HeadersMap = &headers
				(*searchboxPreference.SearchBox.Endpoint.Endpoint).Headers = nil
			}
		}
	}
	return searchboxPreference
}
func (es *elasticsearch) deleteSearchBox(ctx context.Context, searchboxId string) *Error {
	_, err3 := util.GetClient7().Delete().
		Refresh("wait_for").
		Index(es.indexName).
		Id(searchboxId).
		Do(context.Background())
	if err3 != nil {
		if es7.IsNotFound(err3) {
			return &Error{
				err:  err3,
				code: http.StatusNotFound,
			}
		}
		return &Error{
			err: err3,
		}
	}
	return nil
}

// To save the authentication preference
func (es *elasticsearch) setAuthPreference(ctx context.Context, record interface{}) error {
	_, err := util.GetClient7().Index().
		Refresh("wait_for").
		Index(es.indexName).
		Id("auth_preference").
		BodyJson(record).
		Do(ctx)
	return err
}

// To get the authentication preferences
func (es *elasticsearch) getAuthPreference(ctx context.Context) (map[string]interface{}, error) {
	var preferences = make(map[string]interface{})

	response, err := util.GetClient7().Get().
		Index(es.indexName).
		Id("auth_preference").
		Do(ctx)
	if err != nil {
		log.Errorln(logTag, ": error getting authentication preferences", err)
		return preferences, err
	}
	err2 := json.Unmarshal(response.Source, &preferences)
	if err2 != nil {
		log.Errorln(logTag, ": error while unmarshalling the authentication preferences", err2)
		return preferences, err2
	}
	return preferences, nil
}
