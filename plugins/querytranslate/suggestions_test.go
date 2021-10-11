package querytranslate

import (
	"sort"
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

func TestIndexSuggestions(t *testing.T) {
	// when highlight is `true` then suggestion value shouldn't contain html tags
	Convey("index suggestions: highlight", t, func() {
		index := "test"
		rawHits := []ESDoc{
			{
				Id: "1",
				Source: map[string]interface{}{
					"title": "Rabindranath Tagore Hall",
				},
				Highlight: map[string]interface{}{
					"title": []interface{}{"<mark>Rabindranath Tagore Hall</mark>"},
				},
				Index: index,
			},
		}
		suggestions := getFinalSuggestions(SuggestionsConfig{
			Value:      "tagore",
			DataFields: []string{"title"},
		}, rawHits)
		score := float64(0)
		id := "1"
		So(suggestions, ShouldResemble, []SuggestionHIT{
			{
				Label: "<mark>Rabindranath Tagore Hall</mark>",
				Value: "Rabindranath Tagore Hall",
				Id:    &id,
				Index: &index,
				Score: &score,
				Source: map[string]interface{}{
					"title": "Rabindranath Tagore Hall",
				},
			},
		})
	})
}

func TestExtractFieldsFromSource(t *testing.T) {
	Convey("basic", t, func() {
		So(extractFieldsFromSource(map[string]interface{}{
			"title": "ded",
		}), ShouldResemble, []string{"title"})
	})
	Convey("advanced: nested", t, func() {
		output := extractFieldsFromSource(map[string]interface{}{
			"title": "ded",
			"person": map[string]interface{}{
				"name": "John",
				"work": "Painter",
			},
		})
		sort.Strings(output)
		So(output, ShouldResemble, []string{"person.name", "person.work", "title"})
	})
	Convey("advanced: nested with array of objects", t, func() {
		expectedOutput := []string{
			"person.education.degree",
			"person.education.university",
			"person.name",
			"title",
			"person.work",
		}
		sort.Strings(expectedOutput)
		actualOutput := extractFieldsFromSource(map[string]interface{}{
			"title": "ded",
			"person": map[string]interface{}{
				"name": "John",
				"work": "Painter",
				"education": []interface{}{
					map[string]interface{}{
						"degree":     "graduation",
						"university": "harvard",
					},
					map[string]interface{}{
						"degree":     "post graduation",
						"university": "harvard",
					},
				},
			},
		})
		sort.Strings(actualOutput)
		So(actualOutput, ShouldResemble, expectedOutput)
	})
}
