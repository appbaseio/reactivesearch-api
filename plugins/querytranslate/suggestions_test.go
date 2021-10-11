package querytranslate

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestHighlightResults(t *testing.T) {
	Convey("highlight results: with highlight field", t, func() {
		So(highlightResults(ESDoc{
			Source: map[string]interface{}{
				"title":       "Harry Potter Collection",
				"description": "desc1",
			},
			Highlight: map[string]interface{}{
				"title": []interface{}{
					"<mark>Harry</mark> Potter Collection (<mark>Harry</mark> Potter, #1-6)",
				},
			},
		}), ShouldResemble, ESDoc{
			Source: map[string]interface{}{
				"title":       "<mark>Harry</mark> Potter Collection (<mark>Harry</mark> Potter, #1-6)",
				"description": "desc1",
			},
			Highlight: map[string]interface{}{
				"title": []interface{}{
					"<mark>Harry</mark> Potter Collection (<mark>Harry</mark> Potter, #1-6)",
				},
			},
		})
	})
	Convey("highlight results: without highlight field", t, func() {
		So(highlightResults(ESDoc{
			Source: map[string]interface{}{
				"title":       "Harry Potter Collection",
				"description": "desc1",
			},
		}), ShouldResemble, ESDoc{
			Source: map[string]interface{}{
				"title":       "Harry Potter Collection",
				"description": "desc1",
			},
		})
	})
}

func TestPredictiveSuggestions(t *testing.T) {
	// user searches "tagore"
	// field contains: "Rabindranath Tagore Hall"
	// → predictive suggestion will be: "tagore hall"
	Convey("predictive suggestions: 1", t, func() {
		var suggestions = []SuggestionHIT{
			{
				Label: "Rabindranath Tagore Hall",
				Value: "Rabindranath Tagore Hall",
			},
		}
		enable := true
		maxPredictedWords := 2
		predictiveSuggestions := getPredictiveSuggestions(SuggestionsConfig{
			Value:                       "tagore",
			EnablePredictiveSuggestions: &enable,
			MaxPredictedWords:           &maxPredictedWords,
		}, &suggestions)
		So(predictiveSuggestions, ShouldResemble, []SuggestionHIT{
			{
				Label: "tagore<mark class=\"highlight\"> hall</mark>",
				Value: "tagore hall",
			},
		})
	})
	// user searches "tagore"
	// field contains: "rabindranath tagore"
	// → this will be the suggestion
	Convey("predictive suggestions: 2", t, func() {
		var suggestions = []SuggestionHIT{
			{
				Label: "Rabindranath Tagore",
				Value: "Rabindranath Tagore",
			},
		}
		enable := true
		maxPredictedWords := 2
		predictiveSuggestions := getPredictiveSuggestions(SuggestionsConfig{
			Value:                       "tagore",
			EnablePredictiveSuggestions: &enable,
			MaxPredictedWords:           &maxPredictedWords,
		}, &suggestions)
		So(predictiveSuggestions, ShouldResemble, []SuggestionHIT{
			{
				Label: "<mark class=\"highlight\">rabindranath </mark>tagore",
				Value: "rabindranath tagore",
			},
		})
	})
	// user searches: "there"
	// field contains: "here and there"
	// maxpredictedword is 1
	// suggestion would be empty because `and` is a stopword and max word is 1
	Convey("predictive suggestions: 3", t, func() {
		var suggestions = []SuggestionHIT{
			{
				Label: "here and there",
				Value: "here and there",
			},
		}
		enable := true
		maxPredictedWords := 1
		predictiveSuggestions := getPredictiveSuggestions(SuggestionsConfig{
			Value:                       "there",
			EnablePredictiveSuggestions: &enable,
			MaxPredictedWords:           &maxPredictedWords,
			ApplyStopwords:              &enable,
		}, &suggestions)
		So(predictiveSuggestions, ShouldResemble, []SuggestionHIT{
			// {
			// 	Label: "<mark class=\"highlight\">here and </mark>there",
			// 	Value: "here and there",
			// },
		})
	})
	// user searches: "there"
	// field contains: "here and there"
	// suggestion would be empty because `here` & `and` are stopwords
	// maxpredictedword is 2
	Convey("predictive suggestions: 4", t, func() {
		var suggestions = []SuggestionHIT{
			{
				Label: "here and there",
				Value: "here and there",
			},
		}
		enable := true
		maxPredictedWords := 2
		predictiveSuggestions := getPredictiveSuggestions(SuggestionsConfig{
			Value:                       "there",
			EnablePredictiveSuggestions: &enable,
			MaxPredictedWords:           &maxPredictedWords,
			ApplyStopwords:              &enable,
		}, &suggestions)
		So(predictiveSuggestions, ShouldResemble, []SuggestionHIT{})
	})
	// user searches: "there"
	// field contains: "here and there"
	// suggestion would be "and there" because stopwords are not enabled
	// maxpredictedword is 2
	Convey("predictive suggestions: 5", t, func() {
		var suggestions = []SuggestionHIT{
			{
				Label: "here and there",
				Value: "here and there",
			},
		}
		enable := true
		maxPredictedWords := 2
		predictiveSuggestions := getPredictiveSuggestions(SuggestionsConfig{
			Value:                       "there",
			EnablePredictiveSuggestions: &enable,
			MaxPredictedWords:           &maxPredictedWords,
		}, &suggestions)
		So(predictiveSuggestions, ShouldResemble, []SuggestionHIT{
			{
				Label: "<mark class=\"highlight\">and </mark>there",
				Value: "and there",
			},
		})
	})
	// user searches: "bat"
	// field contains: "batman and sons"
	// suggestion would be "batman" because "and" is a stopword and stopwords are enabled
	// maxpredictedword is 1
	Convey("predictive suggestions: 6", t, func() {
		var suggestions = []SuggestionHIT{
			{
				Label: "batman and sons",
				Value: "batman and sons",
			},
		}
		enable := true
		maxPredictedWords := 1
		predictiveSuggestions := getPredictiveSuggestions(SuggestionsConfig{
			Value:                       "bat",
			EnablePredictiveSuggestions: &enable,
			ApplyStopwords:              &enable,
			MaxPredictedWords:           &maxPredictedWords,
		}, &suggestions)
		So(predictiveSuggestions, ShouldResemble, []SuggestionHIT{
			{
				Value: "batman",
				Label: "bat<mark class=\"highlight\">man</mark>",
			},
		})
	})
}
