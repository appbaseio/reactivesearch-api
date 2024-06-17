package uibuilder

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/appbaseio-confidential/reactivesearch/plugins/querytranslate"
	"github.com/appbaseio-confidential/reactivesearch/util"
	log "github.com/sirupsen/logrus"
	. "github.com/smartystreets/goconvey/convey"
)

func createTestIndexZinc(indexWithSuffix string) (*util.ZincClient, bool, error) {
	zincClient := util.GetZincClient()

	// Check if the index already exists
	// Make a request to the get settings endpoint of Zinc
	// and check if the status code is 200 to know if it exists
	// or not.
	existsEndpointZinc := fmt.Sprintf("api/%s/_settings", indexWithSuffix)
	existsResponse, existsResponseErr := zincClient.MakeRequest(existsEndpointZinc, http.MethodGet, []byte(""), nil)

	if existsResponseErr != nil {
		return nil, false, fmt.Errorf("error while checking if index already exists: %v", existsResponseErr)
	}

	if existsResponse == nil || existsResponse.StatusCode == http.StatusOK {
		log.Infoln(logTag, ": index named", indexWithSuffix, "already exists, skipping...")
		return zincClient, true, nil
	}

	indexCreateBody := fmt.Sprintf(indexConfigZinc, indexWithSuffix)

	// Send a create request for the index
	// with the mapping and name of the index present in the body
	indexCreateResponse, indexCreateErr := zincClient.MakeRequest("api/index", http.MethodPost, []byte(indexCreateBody), nil)

	if indexCreateErr != nil {
		return nil, false, fmt.Errorf("error while creating index named: %s, %v", indexWithSuffix, indexCreateErr)
	}

	// TODO: Check status code and handle errors accordingly, if any
	log.Debugln(logTag, "index create status code returned is: ", indexCreateResponse.StatusCode)

	log.Println(logTag, ": successfully created index named", indexWithSuffix)
	return zincClient, false, nil
}

func TestFeaturedSuggestions(t *testing.T) {
	searchboxTestingIndex := ".searchbox_test"
	_, _, err := createTestIndexZinc(searchboxTestingIndex)
	if err != nil {
		t.Fatal(err)
	}
	featuredSuggestionsConfig := FeaturedSuggestionsConfig{
		zincIndex: searchboxTestingIndex,
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
