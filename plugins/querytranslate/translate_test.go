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
		_, err := translateQuery(rsQuery, "127.0.0.1")
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
		_, err := translateQuery(rsQuery, "127.0.0.1")
		So(err, ShouldBeNil)
	})
	Convey("with multiple dataFields for term", t, func() {
		id := "test"
		rsQuery := RSQuery{
			Query: []Query{
				{
					ID:        &id,
					DataField: []interface{}{"data_field_1", "data_field_2"},
					Type:      Term,
				},
			},
		}
		_, err := translateQuery(rsQuery, "127.0.0.1")
		So(err, ShouldBeError)
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
		_, err := translateQuery(rsQuery, "127.0.0.1")
		So(err, ShouldBeNil)
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
		_, err := translateQuery(rsQuery, "127.0.0.1")
		So(err, ShouldBeNil)
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
		_, err := translateQuery(rsQuery, "127.0.0.1")
		So(err, ShouldNotBeNil)
	})
}
