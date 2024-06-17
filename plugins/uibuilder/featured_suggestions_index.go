package uibuilder

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/appbaseio-confidential/reactivesearch/util"
	"github.com/gdexlab/go-render/render"
	"github.com/olivere/elastic/v7"
	es7 "github.com/olivere/elastic/v7"
	log "github.com/sirupsen/logrus"
)

type FeaturedSuggestionsConfig struct {
	zincIndex string
}

func getIndexDocument(featuredSuggestion ESFeaturedSuggestionDoc) map[string]interface{} {
	doc := map[string]interface{}{}
	doc["label"] = featuredSuggestion.Label
	doc["value"] = featuredSuggestion.Value

	if featuredSuggestion.Description != nil {
		doc["description"] = *featuredSuggestion.Description
	}
	if featuredSuggestion.Action != nil {
		doc["action"] = featuredSuggestion.Action.String()
	}
	if featuredSuggestion.SubAction != nil {
		doc["subAction"] = *featuredSuggestion.SubAction
	}
	if featuredSuggestion.SearchboxId != nil {
		doc["searchboxId"] = *featuredSuggestion.SearchboxId
	}
	if featuredSuggestion.Id != nil {
		doc["id"] = *featuredSuggestion.Id
	}
	if featuredSuggestion.SectionId != nil {
		doc["sectionId"] = *featuredSuggestion.SectionId
	}
	if featuredSuggestion.SectionLabel != nil {
		doc["sectionLabel"] = *featuredSuggestion.SectionLabel
	}
	if featuredSuggestion.Icon != nil {
		doc["icon"] = *featuredSuggestion.Icon
	}
	if featuredSuggestion.IconURL != nil {
		doc["iconURL"] = *featuredSuggestion.IconURL
	}
	if featuredSuggestion.Order != nil {
		doc["order"] = *featuredSuggestion.Order
	}
	return doc
}

// Update the featured suggestions in Zinc index
func (featuredSuggestionsConfig *FeaturedSuggestionsConfig) UpdateFeaturedSuggestions(searchboxId string, featuredSuggestions []ESFeaturedSuggestionDoc) error {
	newSuggestionsIds := make([]string, 0)
	for _, suggestion := range featuredSuggestions {
		if suggestion.Id != nil {
			newSuggestionsIds = append(newSuggestionsIds, *suggestion.Id)
		}
	}
	// retrieve saved featured suggestions by id
	savedFeaturedSuggestions, err := featuredSuggestionsConfig.GetFeaturedSuggestions(searchboxId)
	if err != nil {
		log.Errorln(logTag, ":", err)
		return err
	}
	suggestionsIdsToDelete := make([]string, 0)
	for _, suggestion := range savedFeaturedSuggestions {
		if suggestion.Id != nil {
			// Avoid deleting the documents with matching id
			if util.Contains(newSuggestionsIds, *suggestion.Id) {
				continue
			}
			suggestionsIdsToDelete = append(suggestionsIdsToDelete, *suggestion.Id)
		}
	}

	zc := util.GetZincClient()
	bulkRequestEachArr := make([]string, 0)
	// Add items to delete
	for _, id := range suggestionsIdsToDelete {
		marshallledOperation, _ := json.Marshal(map[string]interface{}{
			"delete": map[string]interface{}{"_index": featuredSuggestionsConfig.zincIndex, "_id": id},
		})
		bulkRequestEachArr = append(bulkRequestEachArr, string(marshallledOperation))
	}
	// Add items to index
	for _, featuredSuggestion := range featuredSuggestions {
		if featuredSuggestion.Id != nil {
			doc := getIndexDocument(featuredSuggestion)
			marshalledDoc, err := json.Marshal(doc)
			if err != nil {
				log.Errorln(logTag, ":", err.Error())
				return err
			}
			marshallledOperation, _ := json.Marshal(map[string]interface{}{
				"index": map[string]interface{}{"_index": featuredSuggestionsConfig.zincIndex, "_id": *featuredSuggestion.Id},
			})
			bulkRequestEachArr = append(bulkRequestEachArr, string(marshallledOperation))
			bulkRequestEachArr = append(bulkRequestEachArr, string(marshalledDoc))
		}
	}
	bulkRequestStr := strings.Join(bulkRequestEachArr, "\n")
	bulkRequestStr += "\n"
	bulkReqResponse, bulkRequestErr := zc.MakeRequest("es/_bulk", http.MethodPost, []byte(bulkRequestStr), nil)
	if bulkRequestErr != nil {
		log.Errorln(logTag, ": error while sending searchbox bulk request to Zinc, ", bulkRequestErr)
		return bulkRequestErr
	}

	log.Debugln(logTag, ": bulk request endpoint status code: ", bulkReqResponse.StatusCode)
	if bulkReqResponse.StatusCode != http.StatusOK {
		errMsg := fmt.Sprint("bulk request to Zinc returned a non OK status code: ", bulkReqResponse.StatusCode)
		log.Errorln(logTag, ": ", errMsg)
		return errors.New(errMsg)
	}
	bulkReqResponse.Body.Close()
	return nil
}

func (featuredSuggestionsConfig *FeaturedSuggestionsConfig) DeleteFeaturedSuggestions(searchboxId string, idsToDelete *[]string) error {
	zc := util.GetZincClient()

	suggestionsIds := make([]string, 0)

	if idsToDelete != nil {
		suggestionsIds = *idsToDelete
	} else {
		// retrieve saved featured suggestions by id
		savedFeaturedSuggestions, err := featuredSuggestionsConfig.GetFeaturedSuggestions(searchboxId)
		if err != nil {
			log.Errorln(logTag, ":", err)
			return err
		}
		if len(savedFeaturedSuggestions) == 0 {
			return nil
		}
		for _, savedSuggestion := range savedFeaturedSuggestions {
			if savedSuggestion.Id != nil {
				suggestionsIds = append(suggestionsIds, *savedSuggestion.Id)
			}
		}
	}

	bulkRequestEachArr := make([]string, 0)
	for _, id := range suggestionsIds {
		marshallledOperation, _ := json.Marshal(map[string]interface{}{
			"delete": map[string]interface{}{"_index": featuredSuggestionsConfig.zincIndex, "_id": id},
		})
		bulkRequestEachArr = append(bulkRequestEachArr, string(marshallledOperation))
	}
	bulkRequestStr := strings.Join(bulkRequestEachArr, "\n")
	bulkRequestStr += "\n"
	bulkReqResponse, bulkRequestErr := zc.MakeRequest("es/_bulk", http.MethodPost, []byte(bulkRequestStr), nil)
	if bulkRequestErr != nil {
		log.Errorln(logTag, ": error while sending searchbox bulk request to Zinc, ", bulkRequestErr)
		return bulkRequestErr
	}

	log.Debugln(logTag, ": bulk request endpoint status code: ", bulkReqResponse.StatusCode)
	if bulkReqResponse.StatusCode != http.StatusOK {
		errMsg := fmt.Sprint("bulk request to Zinc returned a non OK status code: ", bulkReqResponse.StatusCode)
		log.Errorln(logTag, ": ", errMsg)
		return errors.New(errMsg)
	}
	bulkReqResponse.Body.Close()
	return nil
}

// Returns the matched featured suggestions by value
func (featuredSuggestionsConfig *FeaturedSuggestionsConfig) SearchFeaturedSuggestions(groupId string, value string) ([]ESFeaturedSuggestionDoc, error) {
	featuredSuggestions := make([]ESFeaturedSuggestionDoc, 0)

	zincEndpointToHit := fmt.Sprintf("es/%s/_search", featuredSuggestionsConfig.zincIndex)

	zc := util.GetZincClient()

	// If value is present ======>
	// Filter by group id
	// Search on following fields: value,label,description & section label
	// If value is NOT present ======>
	// Filter by group id
	// Match All

	query := es7.NewBoolQuery()

	// filter by search box id
	query.Must(es7.NewTermQuery("searchboxId", groupId))
	if strings.TrimSpace(value) != "" {
		// filter by `value` field
		query.Should(es7.NewMatchQuery("value", value).Operator("or").Boost(3).Fuzziness("2"))
		query.Should(es7.NewMatchPhraseQuery("value", value).Boost(3))
		query.Should(es7.NewPrefixQuery("value", value).Boost(3))

		// filter by `label` field
		query.Should(es7.NewMatchQuery("label", value).Operator("or").Boost(2.5).Fuzziness("2"))
		query.Should(es7.NewMatchPhraseQuery("label", value).Boost(2.5))
		query.Should(es7.NewPrefixQuery("label", value).Boost(2.5))

		// filter by `description` field
		query.Should(es7.NewMatchQuery("description", value).Operator("or").Boost(2).Fuzziness("1"))
		query.Should(es7.NewMatchPhraseQuery("description", value).Boost(2))
		query.Should(es7.NewPrefixQuery("description", value).Boost(2))

		// filter by `sectionLabel` field
		query.Should(es7.NewMatchQuery("sectionLabel", value).Operator("and").Boost(2).Fuzziness("1"))
		query.Should(es7.NewMatchPhraseQuery("sectionLabel", value).Boost(2))
		query.Should(es7.NewPrefixQuery("sectionLabel", value).Boost(2))

		query.MinimumNumberShouldMatch(1)
	} else {
		query.Should(es7.NewMatchAllQuery())
	}

	searchSource := es7.NewSearchSource().Query(query).Size(10000)
	searchString, err := searchSource.Source()
	if err != nil {
		log.Errorln(logTag, ":", err)
		return featuredSuggestions, err
	}

	bodyAsString, marshalErr := json.Marshal(searchString)
	if marshalErr != nil {
		log.Warnln(logTag, ": error while marshaling search source, ", marshalErr)
		return featuredSuggestions, marshalErr
	}

	rawQuery := render.Render(string(bodyAsString))

	zincQuery, err := strconv.Unquote(rawQuery)
	if err != nil {
		log.Warnln(logTag, ": error while parsing query, ", err)
		return featuredSuggestions, err
	}

	searchResponse, searchErr := zc.MakeRequest(zincEndpointToHit, http.MethodPost, []byte(zincQuery), nil)
	if searchErr != nil {
		errMsg := fmt.Sprint("error while hitting zinc to get featured suggestions, ", searchErr)
		log.Warnln(logTag, ": ", errMsg)
		return featuredSuggestions, fmt.Errorf(errMsg)
	}
	if searchResponse.StatusCode != http.StatusOK {
		errMsg := fmt.Sprint("Zinc returned a non OK status code: ", searchResponse.StatusCode)
		log.Warnln(logTag, ": ", errMsg)
		return featuredSuggestions, fmt.Errorf(errMsg)
	}

	var res struct {
		Hits *es7.SearchHits `json:"hits,omitempty"`
	}

	defer searchResponse.Body.Close()
	rawSearchBody, readErr := io.ReadAll(searchResponse.Body)

	if readErr != nil {
		log.Warnln(logTag, ": ", readErr)
		return featuredSuggestions, readErr
	}
	err3 := json.Unmarshal(rawSearchBody, &res)
	if err3 != nil {
		log.Errorln(logTag, ":", err3)
		return featuredSuggestions, err3
	}

	for _, hit := range res.Hits.Hits {
		var featureSuggestion ESFeaturedSuggestionDoc
		err := json.Unmarshal(hit.Source, &featureSuggestion)
		if err != nil {
			log.Errorln(logTag, ":", err)
			return featuredSuggestions, err
		}
		featureSuggestion.Score = hit.Score
		featuredSuggestions = append(featuredSuggestions, featureSuggestion)
	}
	return featuredSuggestions, nil
}

// Get saved featured suggestions
func (featuredSuggestionsConfig *FeaturedSuggestionsConfig) GetFeaturedSuggestions(groupId string) ([]ESFeaturedSuggestionDoc, error) {
	featuredSuggestions := make([]ESFeaturedSuggestionDoc, 0)

	zincEndpointToHit := fmt.Sprintf("es/%s/_search", featuredSuggestionsConfig.zincIndex)

	zc := util.GetZincClient()

	// Filter by group id

	query := es7.NewBoolQuery()

	// filter by search box id
	query.Must(es7.NewTermQuery("searchboxId", groupId))

	searchSource := es7.NewSearchSource().Query(query).Size(10000)
	searchString, err := searchSource.Source()
	if err != nil {
		log.Errorln(logTag, ":", err)
		return featuredSuggestions, err
	}

	bodyAsString, marshalErr := json.Marshal(searchString)
	if marshalErr != nil {
		log.Warnln(logTag, ": error while marshaling search source, ", marshalErr)
		return featuredSuggestions, marshalErr
	}

	rawQuery := render.Render(string(bodyAsString))
	zincQuery, err := strconv.Unquote(rawQuery)
	if err != nil {
		log.Warnln(logTag, ": error while parsing query, ", err)
		return featuredSuggestions, err
	}

	searchResponse, searchErr := zc.MakeRequest(zincEndpointToHit, http.MethodPost, []byte(zincQuery), nil)
	if searchErr != nil {
		errMsg := fmt.Sprint("error while hitting zinc to get featured suggestions, ", searchErr, zincQuery)
		log.Warnln(logTag, ": ", errMsg)
		return featuredSuggestions, fmt.Errorf(errMsg)
	}
	if searchResponse.StatusCode != http.StatusOK {
		errMsg := fmt.Sprint("Zinc returned a non OK status code: ", searchResponse.StatusCode)
		log.Warnln(logTag, ": ", errMsg)
		return featuredSuggestions, fmt.Errorf(errMsg)
	}

	var res struct {
		Hits *es7.SearchHits `json:"hits,omitempty"`
	}

	defer searchResponse.Body.Close()
	rawSearchBody, readErr := io.ReadAll(searchResponse.Body)

	if readErr != nil {
		log.Warnln(logTag, ": ", readErr)
		return featuredSuggestions, readErr
	}
	err3 := json.Unmarshal(rawSearchBody, &res)
	if err3 != nil {
		log.Errorln(logTag, ":", err3)
		return featuredSuggestions, err3
	}

	for _, hit := range res.Hits.Hits {
		var featureSuggestion ESFeaturedSuggestionDoc
		err := json.Unmarshal(hit.Source, &featureSuggestion)
		if err != nil {
			log.Errorln(logTag, ":", err)
			return featuredSuggestions, err
		}
		featureSuggestion.Score = hit.Score
		featureSuggestion.Id = &hit.Id
		featuredSuggestions = append(featuredSuggestions, featureSuggestion)
	}
	return featuredSuggestions, nil
}

func (featuredSuggestionsConfig *FeaturedSuggestionsConfig) setFeaturedSuggestionsFromESResponse(response *elastic.SearchResult, searchboxIndex string) error {
	var searchboxes []SearchBoxESModel
	for _, hit := range response.Hits.Hits {
		if hit.Index == searchboxIndex {
			var searchbox SearchBoxESModel
			err := json.Unmarshal(hit.Source, &searchbox)
			if err != nil {
				log.Errorln(logTag, ": ", err)
				return err
			}
			searchboxes = append(searchboxes, searchbox)
			if searchbox.Id != nil {
				featuredSuggestionsConfig.UpdateFeaturedSuggestions(*searchbox.Id, getFeaturedSuggestionsFromSearchBoxes(searchboxes))
			}
		}

	}
	// Update local cache
	SetCachedSearchBoxes(searchboxes)
	return nil
}

// Transforms the featured suggestions from elasticsearch to the featured suggestions stored in zinc index
func getFeaturedSuggestionsFromSearchBoxes(searchboxes []SearchBoxESModel) []ESFeaturedSuggestionDoc {
	featuredSuggestionsToIndex := make([]ESFeaturedSuggestionDoc, 0)
	for _, searchbox := range searchboxes {
		if searchbox.SearchBox != nil &&
			searchbox.SearchBox.Featured != nil &&
			searchbox.SearchBox.Featured.Layout != nil &&
			searchbox.SearchBox.Featured.Layout.Sections != nil {
			for _, section := range searchbox.SearchBox.Featured.Layout.Sections {
				order := 1
				for _, suggestion := range section.Suggestions {
					featuredSuggestionsToIndex = append(featuredSuggestionsToIndex, ESFeaturedSuggestionDoc{
						Label:        suggestion.Label,
						Value:        suggestion.Value,
						Action:       suggestion.Action,
						SubAction:    suggestion.SubAction,
						Id:           suggestion.Id,
						Description:  suggestion.Description,
						Icon:         suggestion.Icon,
						IconURL:      suggestion.IconURL,
						SearchboxId:  searchbox.Id,
						SectionId:    section.Id,
						SectionLabel: section.Label,
						Order:        &order,
					})
					order += 1
				}
			}
		}
	}
	return featuredSuggestionsToIndex
}
