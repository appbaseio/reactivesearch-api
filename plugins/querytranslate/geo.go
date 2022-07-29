package querytranslate

import (
	"errors"
	"strconv"
)

type GeoBoundingBox struct {
	TopLeft     string
	BottomRight string
}
type GeoValue struct {
	Distance    *int
	Unit        *string
	Location    *string
	BoundingBox *GeoBoundingBox
}

// SolrGeoValue will contain the GeoValue similar to the
// ES Geo value except that it will have the distance as a
// float.
type SolrGeoValue struct {
	Distance    *float64
	Unit        *string
	Location    *string
	BoundingBox *GeoBoundingBox
}

// getGeoValueWithoutDistance will extract the geo value without
// extracting the distance field and will return the distance as
// a float (parsed from the request) so that it can be parsed further
// according to need
//
// In case of errors or boundingBox type of value, the distance value
// parsed will be returned as 0.
func (query *Query) getGeoValueWithoutDistance() (*GeoValue, float64, error) {
	if query.Value == nil {
		return nil, 0, nil
	}
	value := *query.Value

	mapValue, isValidValue := value.(map[string]interface{})
	if !isValidValue {
		return nil, 0, errors.New("invalid geo value")
	}
	if mapValue["geoBoundingBox"] == nil {
		if mapValue["distance"] == nil {
			return nil, 0, errors.New("invalid geo value, 'distance' field is missing")
		}
		if mapValue["unit"] == nil {
			return nil, 0, errors.New("invalid geo value, 'unit' field is missing")
		}
		if mapValue["location"] == nil {
			return nil, 0, errors.New("invalid geo value, 'location' field is missing")
		}
		geoValue := GeoValue{}

		distanceAsFloat, ok := mapValue["distance"].(float64)
		if !ok {
			return nil, 0, errors.New("error while parsing `value.distance` for geo type of query")
		}

		unit, ok := mapValue["unit"].(string)
		if ok {
			geoValue.Unit = &unit
		} else {
			return nil, 0, errors.New("invalid unit value in geo query")
		}

		location, ok := mapValue["location"].(string)
		if ok {
			geoValue.Location = &location
		} else {
			return nil, 0, errors.New("invalid location value in geo query")
		}
		return &geoValue, distanceAsFloat, nil
	}
	geoBoundingBox, ok := mapValue["geoBoundingBox"].(map[string]interface{})
	if !ok {
		return nil, 0, errors.New("invalid geo value, geoBoundingBox must be an object")
	}
	if geoBoundingBox["topLeft"] == nil {
		return nil, 0, errors.New("invalid geo value, 'geoBoundingBox.topLeft' field is missing")
	}
	if geoBoundingBox["bottomRight"] == nil {
		return nil, 0, errors.New("invalid geo value, 'geoBoundingBox.bottomRight' field is missing")
	}
	geoValue := GeoValue{}

	topLeft, ok := geoBoundingBox["topLeft"].(string)
	if !ok {
		return nil, 0, errors.New("invalid topLeft value in geo query")
	}
	bottomRight, ok := geoBoundingBox["bottomRight"].(string)
	if !ok {
		return nil, 0, errors.New("invalid bottomRight value in geo query")
	}
	geoValue.BoundingBox = &GeoBoundingBox{
		TopLeft:     topLeft,
		BottomRight: bottomRight,
	}
	return &geoValue, 0, nil
}

func (query *Query) getGeoValue() (*GeoValue, error) {
	valueWithoutDistance, distanceParsed, parseErr := query.getGeoValueWithoutDistance()
	if parseErr != nil {
		return nil, parseErr
	}

	// If no error was thrown, parse the distance since it is
	// required in ES as integer.
	distanceAsInt := int(distanceParsed)
	valueWithoutDistance.Distance = &distanceAsInt

	return valueWithoutDistance, nil
}

// GetSolrGeoValue will parse the value passed as a geo value for
// Solr and accordingly return it.
func (query *Query) GetSolrGeoValue() (*SolrGeoValue, error) {
	valueWithoutDistance, distanceParsed, parseErr := query.getGeoValueWithoutDistance()
	if parseErr != nil {
		return nil, parseErr
	}

	// We will need to convert the GeoValue to SolrGeoValue first
	solrGeoValue := &SolrGeoValue{
		Unit:        valueWithoutDistance.Unit,
		Location:    valueWithoutDistance.Location,
		BoundingBox: valueWithoutDistance.BoundingBox,
		Distance:    &distanceParsed,
	}

	return solrGeoValue, nil
}

// GetGeoValue extracts the value into a GeoValue object
func (query *Query) GetGeoValue() (*GeoValue, error) {
	return query.getGeoValue()
}

func (query *Query) generateGeoQuery() (*interface{}, error) {
	geoValue, err := query.getGeoValue()

	if err != nil {
		return nil, err
	}

	if geoValue == nil {
		return nil, nil
	}

	normalizedFields := NormalizedDataFields(query.DataField, query.FieldWeights)
	if len(normalizedFields) < 1 {
		return nil, errors.New("field 'dataField' cannot be empty")
	}
	dataField := normalizedFields[0].Field

	var geoQuery interface{}

	if geoValue.Distance != nil && geoValue.Location != nil && geoValue.Unit != nil {
		geoQuery = map[string]interface{}{
			"geo_distance": map[string]interface{}{
				"distance": strconv.Itoa(*geoValue.Distance) + *geoValue.Unit,
				dataField:  *geoValue.Location,
			},
		}
	} else if geoValue.BoundingBox != nil {
		geoQuery = map[string]interface{}{
			"geo_bounding_box": map[string]interface{}{
				dataField: map[string]interface{}{
					"top_left":     geoValue.BoundingBox.TopLeft,
					"bottom_right": geoValue.BoundingBox.BottomRight,
				},
			},
		}
	}
	// Apply nestedField query
	geoQuery = query.applyNestedFieldQuery(geoQuery)
	return &geoQuery, nil
}
