package uibuilder

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/appbaseio/reactivesearch-api/util"
	es7 "github.com/olivere/elastic/v7"
	log "github.com/sirupsen/logrus"
)

type FeaturedSuggestionsConfig struct {
	esIndex string
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

// Update the featured suggestions in ES index
func (featuredSuggestionsConfig *FeaturedSuggestionsConfig) UpdateFeaturedSuggestions(searchboxId string, featuredSuggestions []ESFeaturedSuggestionDoc) error {
	ctx := context.Background()
	client := util.GetClient7()

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

	bulkRequest := client.Bulk().Index(featuredSuggestionsConfig.esIndex)

	// Add items to delete
	for _, id := range suggestionsIdsToDelete {
		bulkRequest.Add(es7.NewBulkDeleteRequest().Id(id))
	}
	// Add items to index
	for _, featuredSuggestion := range featuredSuggestions {
		if featuredSuggestion.Id != nil {
			doc := getIndexDocument(featuredSuggestion)
			bulkRequest.Add(es7.NewBulkIndexRequest().Id(*featuredSuggestion.Id).Doc(doc))
		}
	}

	if bulkRequest.NumberOfActions() == 0 {
		return nil
	}

	bulkResponse, bulkErr := bulkRequest.Refresh("wait_for").Do(ctx)
	if bulkErr != nil {
		log.Errorln(logTag, ": error while sending searchbox bulk request to ES, ", bulkErr)
		return bulkErr
	}

	if bulkResponse.Errors {
		errMsg := fmt.Sprint("bulk request to ES had errors")
		log.Errorln(logTag, ": ", errMsg)
		for _, item := range bulkResponse.Failed() {
			log.Errorln(logTag, ": bulk item error: ", item.Error)
		}
	}

	return nil
}

func (featuredSuggestionsConfig *FeaturedSuggestionsConfig) DeleteFeaturedSuggestions(searchboxId string, idsToDelete *[]string) error {
	ctx := context.Background()
	client := util.GetClient7()

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

	if len(suggestionsIds) == 0 {
		return nil
	}

	bulkRequest := client.Bulk().Index(featuredSuggestionsConfig.esIndex)
	for _, id := range suggestionsIds {
		bulkRequest.Add(es7.NewBulkDeleteRequest().Id(id))
	}

	bulkResponse, bulkErr := bulkRequest.Refresh("wait_for").Do(ctx)
	if bulkErr != nil {
		log.Errorln(logTag, ": error while sending searchbox bulk delete request to ES, ", bulkErr)
		return bulkErr
	}

	if bulkResponse.Errors {
		errMsg := fmt.Sprint("bulk delete request to ES had errors")
		log.Errorln(logTag, ": ", errMsg)
		for _, item := range bulkResponse.Failed() {
			log.Errorln(logTag, ": bulk item error: ", item.Error)
		}
	}

	return nil
}

// Returns the matched featured suggestions by value
func (featuredSuggestionsConfig *FeaturedSuggestionsConfig) SearchFeaturedSuggestions(groupId string, value string) ([]ESFeaturedSuggestionDoc, error) {
	ctx := context.Background()
	client := util.GetClient7()
	featuredSuggestions := make([]ESFeaturedSuggestionDoc, 0)

	// If value is present ======>
	// Filter by group id
	// Search on following fields: value,label,description & section label
	// If value is NOT present ======>
	// Filter by group id
	// Match All

	query := es7.NewBoolQuery()

	// filter by search box id
	query.Must(es7.NewTermQuery("searchboxId", groupId))
	if value != "" {
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

	searchResult, err := client.Search().
		Index(featuredSuggestionsConfig.esIndex).
		Query(query).
		Size(10000).
		Do(ctx)
	if err != nil {
		errMsg := fmt.Sprint("error while hitting ES to get featured suggestions, ", err)
		log.Warnln(logTag, ": ", errMsg)
		return featuredSuggestions, fmt.Errorf(errMsg)
	}

	for _, hit := range searchResult.Hits.Hits {
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
	ctx := context.Background()
	client := util.GetClient7()
	featuredSuggestions := make([]ESFeaturedSuggestionDoc, 0)

	// Filter by group id
	query := es7.NewBoolQuery()
	query.Must(es7.NewTermQuery("searchboxId", groupId))

	searchResult, err := client.Search().
		Index(featuredSuggestionsConfig.esIndex).
		Query(query).
		Size(10000).
		Do(ctx)
	if err != nil {
		errMsg := fmt.Sprint("error while hitting ES to get featured suggestions, ", err)
		log.Warnln(logTag, ": ", errMsg)
		return featuredSuggestions, fmt.Errorf(errMsg)
	}

	for _, hit := range searchResult.Hits.Hits {
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

func (featuredSuggestionsConfig *FeaturedSuggestionsConfig) setFeaturedSuggestionsFromESResponse(response *es7.SearchResult, searchboxIndex string, syncToES bool) error {
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
		}
	}

	// Only sync to ES if requested (i.e., when index was just created)
	if syncToES {
		ctx := context.Background()
		client := util.GetClient7()

		// Collect all featured suggestions from all searchboxes
		allFeaturedSuggestions := make([]ESFeaturedSuggestionDoc, 0)
		for _, searchbox := range searchboxes {
			if searchbox.Id != nil {
				featuredSuggestions := getFeaturedSuggestionsFromSearchBox(searchbox)
				allFeaturedSuggestions = append(allFeaturedSuggestions, featuredSuggestions...)
			}
		}

		// Batch index all featured suggestions in a single bulk request
		if len(allFeaturedSuggestions) > 0 {
			bulkRequest := client.Bulk().Index(featuredSuggestionsConfig.esIndex)
			for _, featuredSuggestion := range allFeaturedSuggestions {
				if featuredSuggestion.Id != nil {
					doc := getIndexDocument(featuredSuggestion)
					bulkRequest.Add(es7.NewBulkIndexRequest().Id(*featuredSuggestion.Id).Doc(doc))
				}
			}

			if bulkRequest.NumberOfActions() > 0 {
				bulkResponse, bulkErr := bulkRequest.Do(ctx)
				if bulkErr != nil {
					log.Errorln(logTag, ": error while sending featured suggestions bulk request to ES, ", bulkErr)
					return bulkErr
				}
				if bulkResponse.Errors {
					log.Warnln(logTag, ": bulk request to ES had some errors")
					for _, item := range bulkResponse.Failed() {
						log.Errorln(logTag, ": bulk item error: ", item.Error)
					}
				}
				log.Infoln(logTag, ": indexed", len(allFeaturedSuggestions), "featured suggestions")
			}
		}
	} else {
		log.Infoln(logTag, ": skipping featured suggestions sync (index already exists)")
	}

	// Update local cache
	SetCachedSearchBoxes(searchboxes)
	return nil
}

// Transforms the featured suggestions from a single searchbox to denormalized documents
func getFeaturedSuggestionsFromSearchBox(searchbox SearchBoxESModel) []ESFeaturedSuggestionDoc {
	featuredSuggestionsToIndex := make([]ESFeaturedSuggestionDoc, 0)
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
	return featuredSuggestionsToIndex
}

// Transforms the featured suggestions from multiple searchboxes (kept for backwards compatibility)
func getFeaturedSuggestionsFromSearchBoxes(searchboxes []SearchBoxESModel) []ESFeaturedSuggestionDoc {
	featuredSuggestionsToIndex := make([]ESFeaturedSuggestionDoc, 0)
	for _, searchbox := range searchboxes {
		featuredSuggestionsToIndex = append(featuredSuggestionsToIndex, getFeaturedSuggestionsFromSearchBox(searchbox)...)
	}
	return featuredSuggestionsToIndex
}
