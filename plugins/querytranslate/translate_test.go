package querytranslate

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestTranslateQuery(t *testing.T) {
	Convey("with multiple dataFields for geo", t, func() {
		id := "test"
		rsQuery := RSQuery{
			Query: []Query{
				Query{
					ID:        &id,
					DataField: []string{"data_field_1", "data_field_2"},
					Type:      Geo,
				},
			},
		}
		_, err := translateQuery(rsQuery)
		So(err, ShouldBeError)
	})
	Convey("with single dataField for geo", t, func() {
		id := "test"
		rsQuery := RSQuery{
			Query: []Query{
				Query{
					ID:        &id,
					DataField: []string{"data_field_1"},
					Type:      Geo,
				},
			},
		}
		_, err := translateQuery(rsQuery)
		So(err, ShouldBeNil)
	})
	Convey("with multiple dataFields for term", t, func() {
		id := "test"
		rsQuery := RSQuery{
			Query: []Query{
				Query{
					ID:        &id,
					DataField: []string{"data_field_1", "data_field_2"},
					Type:      Term,
				},
			},
		}
		_, err := translateQuery(rsQuery)
		So(err, ShouldBeError)
	})
	Convey("with single dataField for term", t, func() {
		id := "test"
		rsQuery := RSQuery{
			Query: []Query{
				Query{
					ID:        &id,
					DataField: []string{"data_field_1"},
					Type:      Term,
				},
			},
		}
		_, err := translateQuery(rsQuery)
		So(err, ShouldBeNil)
	})
	Convey("with multiple dataFields for search", t, func() {
		id := "test"
		rsQuery := RSQuery{
			Query: []Query{
				Query{
					ID:        &id,
					DataField: []string{"data_field_1", "data_field_2"},
				},
			},
		}
		_, err := translateQuery(rsQuery)
		So(err, ShouldBeNil)
	})
}
