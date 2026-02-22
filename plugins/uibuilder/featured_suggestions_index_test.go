package uibuilder

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/appbaseio/reactivesearch-api/plugins/querytranslate"
	"github.com/appbaseio/reactivesearch-api/util"
	log "github.com/sirupsen/logrus"
	. "github.com/smartystreets/goconvey/convey"
)

func createTestIndexES(indexName string) error {
	ctx := context.Background()
	client := util.GetClient7()

	// Check if the index already exists
	exists, err := client.IndexExists(indexName).Do(ctx)
	if err != nil {
		return fmt.Errorf("error while checking if index already exists: %v", err)
	}
	if exists {
		log.Infoln(logTag, ": index named", indexName, "already exists, skipping...")
		return nil
	}

	// Create the index
	_, err = client.CreateIndex(indexName).Do(ctx)
	if err != nil {
		return fmt.Errorf("error while creating index named: %s, %v", indexName, err)
	}

	log.Println(logTag, ": successfully created index named", indexName)
	return nil
}

func TestFeaturedSuggestions(t *testing.T) {
	searchboxTestingIndex := ".searchbox_test"
	err := createTestIndexES(searchboxTestingIndex)
	if err != nil {
		t.Fatal(err)
	}
	featuredSuggestionsConfig := FeaturedSuggestionsConfig{
		esIndex: searchboxTestingIndex,
	}
	searchboxId := "document"

	desc1 := "React: Step by Step guide"
	id1 := "test-react"
	action1 := querytranslate.Navigate
	subAction1 := `"{ \"link\" : \"/react\" }"`
	sectionId := "document"
	icon1 := "https://icons.com/react.png"
	order1 := 1
	featuredSuggestions := []ESFeaturedSuggestionDoc{
		{
			Label:       "Learn React",
			Value:       "react",
			Description: &desc1,
			Id:          &id1,
			Action:      &action1,
			SubAction:   &subAction1,
			SectionId:   &sectionId,
			IconURL:     &icon1,
			Order:       &order1,
			SearchboxId: &searchboxId,
		},
		{
			Label:       "Learn Vue",
			Value:       "vue",
			Description: &desc1,
			Id:          &id1,
			Action:      &action1,
			SubAction:   &subAction1,
			SectionId:   &sectionId,
			IconURL:     &icon1,
			Order:       &order1,
			SearchboxId: &searchboxId,
		},
	}
	Convey("index featured suggestions", t, func() {
		err2 := featuredSuggestionsConfig.UpdateFeaturedSuggestions(searchboxId, featuredSuggestions)
		if err2 != nil {
			t.Fatal(err2)
		}
		So(true, ShouldResemble, true)
	})
	Convey("search featured suggestions: empty query", t, func() {
		suggestions, err := featuredSuggestionsConfig.SearchFeaturedSuggestions(searchboxId, "")
		if err != nil {
			t.Fatal(err)
		}
		var parsedSuggestions []ESFeaturedSuggestionDoc
		for _, v := range suggestions {
			parsedSuggestions = append(parsedSuggestions, ESFeaturedSuggestionDoc{
				Label:       v.Label,
				Value:       v.Value,
				Description: v.Description,
				Id:          v.Id,
				Action:      v.Action,
				SubAction:   v.SubAction,
				SectionId:   v.SectionId,
				IconURL:     v.IconURL,
				Order:       v.Order,
				SearchboxId: v.SearchboxId,
			})
		}
		parsedFetched, _ := json.Marshal(parsedSuggestions)
		actual, _ := json.Marshal(featuredSuggestions)
		So(string(parsedFetched), ShouldResemble, string(actual))
	})
	Convey("search featured suggestions: query value", t, func() {
		suggestions, err := featuredSuggestionsConfig.SearchFeaturedSuggestions(searchboxId, "learn react")
		if err != nil {
			t.Fatal(err)
		}
		var parsedSuggestions []ESFeaturedSuggestionDoc
		for _, v := range suggestions {
			parsedSuggestions = append(parsedSuggestions, ESFeaturedSuggestionDoc{
				Label:       v.Label,
				Value:       v.Value,
				Description: v.Description,
				Id:          v.Id,
				Action:      v.Action,
				SubAction:   v.SubAction,
				SectionId:   v.SectionId,
				IconURL:     v.IconURL,
				Order:       v.Order,
				SearchboxId: v.SearchboxId,
			})
		}
		parsedFetched, _ := json.Marshal(parsedSuggestions)
		actual, _ := json.Marshal([]ESFeaturedSuggestionDoc{
			{
				Label:       "Learn React",
				Value:       "react",
				Description: &desc1,
				Id:          &id1,
				Action:      &action1,
				SubAction:   &subAction1,
				SectionId:   &sectionId,
				IconURL:     &icon1,
				Order:       &order1,
				SearchboxId: &searchboxId,
			},
		})
		So(string(parsedFetched), ShouldResemble, string(actual))
	})
	Convey("delete featured suggestions", t, func() {
		err := featuredSuggestionsConfig.DeleteFeaturedSuggestions(searchboxId, nil)
		if err != nil {
			t.Fatal(err)
		}
		So(true, ShouldResemble, true)
	})
}
