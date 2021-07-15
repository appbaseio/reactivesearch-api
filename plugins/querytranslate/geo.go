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

func (query *Query) getGeoValue() (*GeoValue, error) {
	if query.Value == nil {
		return nil, nil
	}
	value := *query.Value

	mapValue, isValidValue := value.(map[string]interface{})
	if !isValidValue {
		return nil, errors.New("invalid geo value")
	}
	if mapValue["geoBoundingBox"] == nil {
		if mapValue["distance"] == nil {
			return nil, errors.New("invalid geo value, 'distance' field is missing")
		}
		if mapValue["unit"] == nil {
			return nil, errors.New("invalid geo value, 'unit' field is missing")
		}
		if mapValue["location"] == nil {
			return nil, errors.New("invalid geo value, 'location' field is missing")
		}
		geoValue := GeoValue{}

		distance, ok := mapValue["distance"].(float64)
		if ok {
			distanceAsInt := int(distance)
			geoValue.Distance = &distanceAsInt
		} else {
			return nil, errors.New("invalid distance value in geo query")
		}

		unit, ok := mapValue["unit"].(string)
		if ok {
			geoValue.Unit = &unit
		} else {
			return nil, errors.New("invalid unit value in geo query")
		}

		location, ok := mapValue["location"].(string)
		if ok {
			geoValue.Location = &location
		} else {
			return nil, errors.New("invalid location value in geo query")
		}
		return &geoValue, nil
	}
	geoBoundingBox, ok := mapValue["geoBoundingBox"].(map[string]interface{})
	if !ok {
		return nil, errors.New("invalid geo value, geoBoundingBox must be an object")
	}
	if geoBoundingBox["topLeft"] == nil {
		return nil, errors.New("invalid geo value, 'geoBoundingBox.topLeft' field is missing")
	}
	if geoBoundingBox["bottomRight"] == nil {
		return nil, errors.New("invalid geo value, 'geoBoundingBox.bottomRight' field is missing")
	}
	geoValue := GeoValue{}

	topLeft, ok := geoBoundingBox["topLeft"].(string)
	if !ok {
		return nil, errors.New("invalid topLeft value in geo query")
	}
	bottomRight, ok := geoBoundingBox["bottomRight"].(string)
	if !ok {
		return nil, errors.New("invalid bottomRight value in geo query")
	}
	geoValue.BoundingBox = &GeoBoundingBox{
		TopLeft:     topLeft,
		BottomRight: bottomRight,
	}
	return &geoValue, nil
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
		return nil, errors.New("Field 'dataField' cannot be empty")
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
