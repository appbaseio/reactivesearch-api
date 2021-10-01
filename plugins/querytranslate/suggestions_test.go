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
