package querytranslate

import (
	"errors"
)

// RangeValue represents the struct of range value
type RangeValue struct {
	Start *interface{}
	End   *interface{}
	Boost *float64
}

func (query *Query) getRangeValue(value interface{}) (*RangeValue, error) {
	mapValue, isValidValue := value.(map[string]interface{})

	if !isValidValue {
		return nil, errors.New("invalid range value")
	}

	rangeValue := RangeValue{}

	if mapValue["start"] != nil {
		start := mapValue["start"]
		rangeValue.Start = &start
	}

	if mapValue["end"] != nil {
		end := mapValue["end"]
		rangeValue.End = &end
	}

	if mapValue["boost"] != nil {
		boost, ok := mapValue["boost"].(float64)
		if ok {
			rangeValue.Boost = &boost
		}
	}
	return &rangeValue, nil
}

// GetRangeValue is a wrapper on top of getRangeValue to extract
// the value in a rangeValue type.
func (query *Query) GetRangeValue(value interface{}) (*RangeValue, error) {
	return query.getRangeValue(value)
}

func (query *Query) getRangeQuery(value interface{}) (*map[string]interface{}, error) {
	rangeValue, err := query.getRangeValue(value)

	if err != nil {
		return nil, err
	}

	if rangeValue == nil {
		return nil, nil
	}

	if rangeValue.Start == nil && rangeValue.End == nil {
		return nil, nil
	}

	normalizedFields := NormalizedDataFields(query.DataField, query.FieldWeights)
	if len(normalizedFields) < 1 {
		return nil, errors.New("field 'dataField' cannot be empty")
	}
	dataField := normalizedFields[0].Field

	var rangeQuery map[string]interface{}
	tempRangeQuery := make(map[string]interface{})

	if rangeValue.Start != nil {
		tempRangeQuery["gte"] = rangeValue.Start
	}
	if rangeValue.End != nil {
		tempRangeQuery["lte"] = rangeValue.End
	}
	// apply query format for date queries
	if query.QueryFormat != nil &&
		*query.QueryFormat != And.String() &&
		*query.QueryFormat != Or.String() {
		tempRangeQuery["format"] = *query.QueryFormat
	}
	if rangeValue.Boost != nil {
		tempRangeQuery["boost"] = rangeValue.Boost
	}
	rangeQuery = map[string]interface{}{
		"range": map[string]interface{}{
			dataField: tempRangeQuery,
		},
	}

	if query.IncludeNullValues != nil && *query.IncludeNullValues {
		rangeQuery = map[string]interface{}{
			"bool": map[string]interface{}{
				"should": []interface{}{
					rangeQuery,
					map[string]interface{}{
						"bool": map[string]interface{}{
							"must_not": map[string]interface{}{
								"exists": map[string]interface{}{
									"field": dataField,
								},
							},
						},
					},
				},
			},
		}
	}
	return &rangeQuery, nil
}

func (query *Query) generateRangeQuery() (*interface{}, error) {

	if query.Value == nil {
		return nil, nil
	}

	value := *query.Value
	valueAsArray, isMulti := value.([]interface{})

	var rangeQuery interface{}

	if isMulti {
		var multiRangeQuery []interface{}
		for _, value := range valueAsArray {
			rangeQuery, err := query.getRangeQuery(value)
			if err != nil {
				return nil, err
			}
			if rangeQuery == nil {
				return nil, nil
			}
			multiRangeQuery = append(multiRangeQuery, *rangeQuery)
		}
		if len(multiRangeQuery) == 0 {
			return nil, nil
		}
		rangeQuery = map[string]interface{}{
			"bool": map[string]interface{}{
				"should":               multiRangeQuery,
				"minimum_should_match": 1,
				"boost":                1.0,
			},
		}

	} else {
		tempRangeQuery, err := query.getRangeQuery(*query.Value)
		if err != nil {
			return nil, err
		}
		if tempRangeQuery == nil {
			return nil, nil
		}
		rangeQuery = *tempRangeQuery
	}
	// Apply nestedField query
	rangeQuery = query.applyNestedFieldQuery(rangeQuery)

	return &rangeQuery, nil
}
