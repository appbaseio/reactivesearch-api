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
				{
					ID:        &id,
					DataField: []interface{}{"data_field_1", "data_field_2"},
					Type:      Geo,
				},
			},
		}
		_, _, err := translateQuery(rsQuery, "127.0.0.1", nil, nil)
		So(err, ShouldBeError)
	})
	Convey("with single dataField for geo", t, func() {
		id := "test"
		rsQuery := RSQuery{
			Query: []Query{
				{
					ID:        &id,
					DataField: "data_field_1",
					Type:      Geo,
				},
			},
		}
		_, _, err := translateQuery(rsQuery, "127.0.0.1", nil, nil)
		So(err, ShouldBeNil, nil)
	})
	Convey("with single dataField for term", t, func() {
		id := "test"
		rsQuery := RSQuery{
			Query: []Query{
				{
					ID:        &id,
					DataField: "data_field_1",
					Type:      Term,
				},
			},
		}
		_, _, err := translateQuery(rsQuery, "127.0.0.1", nil, nil)
		So(err, ShouldBeNil, nil)
	})
	Convey("with multiple dataFields for search", t, func() {
		id := "test"
		rsQuery := RSQuery{
			Query: []Query{
				{
					ID:        &id,
					DataField: []interface{}{"data_field_1", "data_field_2"},
				},
			},
		}
		_, _, err := translateQuery(rsQuery, "127.0.0.1", nil, nil)
		So(err, ShouldBeNil, nil)
	})
	Convey("without dataField", t, func() {
		id := "test"
		var value interface{} = "data_field"
		rsQuery := RSQuery{
			Query: []Query{
				{
					ID:    &id,
					Type:  Term,
					Value: &value,
				},
			},
		}
		_, _, err := translateQuery(rsQuery, "127.0.0.1", nil, nil)
		So(err, ShouldNotBeNil, nil)
	})
}
