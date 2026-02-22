package pipelines

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/appbaseio/reactivesearch-api/model/category"
	"github.com/appbaseio/reactivesearch-api/plugins/querytranslate"
	"github.com/appbaseio/reactivesearch-api/plugins/rules"
	"github.com/appbaseio/reactivesearch-api/util"
	"github.com/gdexlab/go-render/render"
	"github.com/invopop/jsonschema"
	log "github.com/sirupsen/logrus"
)

type BoostType int

const (
	Score BoostType = iota
	Promote
)

// String is the implementation of Stringer interface that returns the string representation of BoostType type.
func (o BoostType) String() string {
	return [...]string{
		"score",
		"promote",
	}[o]
}

// UnmarshalJSON is the implementation of the Unmarshaler interface for unmarshaling BoostType type.
func (o *BoostType) UnmarshalJSON(bytes []byte) error {
	var boostType string
	err := json.Unmarshal(bytes, &boostType)
	if err != nil {
		return err
	}
	switch boostType {
	case Score.String():
		*o = Score
	case Promote.String():
		*o = Promote
	default:
		return fmt.Errorf("invalid boostType encountered: %v", boostType)
	}
	return nil
}

// MarshalJSON is the implementation of the Marshaler interface for marshaling BoostType type.
func (o BoostType) MarshalJSON() ([]byte, error) {
	var boostType string
	switch o {
	case Score:
		boostType = Score.String()
	case Promote:
		boostType = Promote.String()
	default:
		return nil, fmt.Errorf("invalid boostType encountered: %v", o)
	}
	return json.Marshal(boostType)
}

func (o BoostType) JSONSchema() *jsonschema.Schema {
	return &jsonschema.Schema{
		Type: "string",
		Enum: []interface{}{
			Score.String(),
			Promote.String(),
		},
	}
}

type BoostOperation int

const (
	Add BoostOperation = iota
	Multiply
)

// String is the implementation of Stringer interface that returns the string representation of BoostOperation type.
func (o BoostOperation) String() string {
	return [...]string{
		"add",
		"multiply",
	}[o]
}

// UnmarshalJSON is the implementation of the Unmarshaler interface for unmarshaling BoostOperation type.
func (o *BoostOperation) UnmarshalJSON(bytes []byte) error {
	var boostOperation string
	err := json.Unmarshal(bytes, &boostOperation)
	if err != nil {
		return err
	}
	switch boostOperation {
	case Add.String():
		*o = Add
	case Multiply.String():
		*o = Multiply
	default:
		return fmt.Errorf("invalid boostOp encountered: %v", boostOperation)
	}
	return nil
}

// MarshalJSON is the implementation of the Marshaler interface for marshaling BoostOperation type.
func (o BoostOperation) MarshalJSON() ([]byte, error) {
	var boostOp string
	switch o {
	case Add:
		boostOp = Add.String()
	case Multiply:
		boostOp = Multiply.String()
	default:
		return nil, fmt.Errorf("invalid boostOp encountered: %v", o)
	}
	return json.Marshal(boostOp)
}

func (o BoostOperation) JSONSchema() *jsonschema.Schema {
	return &jsonschema.Schema{
		Type: "string",
		Enum: []interface{}{
			Add.String(),
			Multiply.String(),
		},
	}
}

type BoostStageInputStruct struct {
	DataField          *string                    `json:"dataField" jsonschema:"title=Data Field" jsonschema_description:"Field name to match value to find documents to rank."`
	Value              *interface{}               `json:"value" jsonschema:"title=Value" jsonschema_description:""`
	BoostType          BoostType                  `json:"boostType" jsonschema:"title=Boost Type" jsonschema_description:"To define the type of boost stage. Defaults to 'score'."`
	BoostOperation     BoostOperation             `json:"boostOp" jsonschema:"title=Boost Operation" jsonschema_description:"Boost operation, for e.g 'add' or 'multiply'."`
	BoostFactor        int                        `json:"boostFactor" jsonschema:"title=Boost Factor" jsonschema_description:"Boost factor to be used by boost operation, for e.g, boost_factor as 2 & boostType as 'multiply' would multiply the score by 2 and re-ranks the results."`
	BoostMaxDocs       *int                       `json:"boostMaxDocs" jsonschema:"title=Boost Max Docs" jsonschema_description:"Maximum number of documents to boost."`
	QueryFormat        querytranslate.QueryFormat `json:"queryFormat" jsonschema:"title=Query Format" jsonschema_description:"Query operator for boost query, can have values as 'or' and 'and'. Defaults to 'or'."`
	BoostSizeThreshold *int                       `json:"boostSizeThreshold" jsonschema:"title=Boost Size Threshold" jsonschema_description:"Maximum 'from' + 'size' value till the boost stage will get applied"`
}

func GetBoostInputSchema() map[string]interface{} {
	schema := GetReflactor().Reflect(&BoostStageInputStruct{})
	inputsSchema, _ := schema.MarshalJSON()
	var schemaInBytes map[string]interface{}
	json.Unmarshal(inputsSchema, &schemaInBytes)
	return schemaInBytes
}

type BoostStageResponse struct {
	QueryId    string                `json:"queryId"`
	Inputs     BoostStageInputStruct `json:"inputs"`
	Query      string                `json:"query"`
	Index      string                `json:"index"`
	ShouldSkip bool                  `json:"shouldSkip"`
}

func executeBoostStage(
	stage ESPipelineStage,
	parsedInputs *string,
	scriptContextInBytes []byte,
	rsAPIRequest *ReactiveSearchQueryContext,
	scriptEnvs map[string]interface{},
	async bool,
	startTime *time.Time) ([]byte, bool, *Error) {
	if parsedInputs == nil {
		return scriptContextInBytes, false, nil
	}
	stageId := getStageID(stage)

	var parsedBoostInputs BoostStageInputStruct
	err2 := json.Unmarshal([]byte(*parsedInputs), &parsedBoostInputs)
	if err2 != nil {
		return scriptContextInBytes, false, &Error{
			Err: err2,
		}
	}
	if parsedBoostInputs.DataField == nil {
		return scriptContextInBytes, false, &Error{
			Err:  fmt.Errorf("The 'dataField' property must be present for stage: " + *stageId),
			Code: http.StatusBadRequest,
		}
	}
	if parsedBoostInputs.BoostType == Score && parsedBoostInputs.Value == nil {
		return scriptContextInBytes, false, &Error{
			Err:  fmt.Errorf("The 'value' property must be present when boostType is 'score' for stage: " + *stageId),
			Code: http.StatusBadRequest,
		}
	}
	var scriptContext rules.ScriptContext
	err := json.Unmarshal(scriptContextInBytes, &scriptContext)
	if err != nil {
		log.Errorln(logTag, ":", err)
		return nil, false, &Error{
			Err: err,
		}
	}
	reqCategory, ok := scriptEnvs["category"].(string)
	log.Debug(logTag, ": request category: ", reqCategory)
	if ok && reqCategory == category.ReactiveSearch.String() {
		// Perform boost queries
		// read RS API from channel
		rsQuery := rsAPIRequest.Get()
		log.Debug(logTag, ": rsQuery: ", render.AsCode(rsQuery))
		if rsQuery != nil {
			// extract ip from envs
			var ip string
			if ipv4, ok := scriptEnvs["ipv4"]; ok {
				ipAsString, ok := ipv4.(string)
				if ok {
					ip = ipAsString
				}
			}
			if ip == "" {
				if ipv6, ok := scriptEnvs["ipv6"]; ok {
					ipAsString, ok := ipv6.(string)
					if ok {
						ip = ipAsString
					}
				}
			}
			var boostQuery string
			var queryId string
			// Defaults to indices in URL
			var index = strings.Join(getIndicesFromEnvs(scriptEnvs), ",")
			size := 10
			if parsedBoostInputs.BoostMaxDocs != nil {
				size = *parsedBoostInputs.BoostMaxDocs
			}
			boostQueryIndex := getBoostQueryIndex(rsQuery)
			var computedSize *int
			log.Debug(logTag, ": boost query index: ", boostQueryIndex)
			if boostQueryIndex != -1 {
				query := rsQuery.Query[boostQueryIndex]

				// Avoid executing boost query if computed size is greater than threshold
				if parsedBoostInputs.BoostSizeThreshold != nil {
					computedSize = getComputedSize(query)
					if computedSize != nil &&
						*computedSize > *parsedBoostInputs.BoostSizeThreshold {
						// Write empty response so `es_stage` can trim the results > query.size
						boostResponse := BoostStageResponse{
							Inputs:     parsedBoostInputs,
							QueryId:    queryId,
							ShouldSkip: true,
						}
						marshalledResponse, err := json.Marshal(boostResponse)
						if err != nil {
							log.Errorln(logTag, ":", err)
							return nil, false, &Error{
								Err: err,
							}
						}
						// write output to a top-level variable
						output := map[string]interface{}{
							*stageId + "__boost": string(marshalledResponse),
						}
						contextInBytes, err := json.Marshal(output)
						if err != nil {
							log.Errorln(logTag, ":", err)
							return nil, false, &Error{
								Err: err,
							}
						}
						// return response
						return contextInBytes, false, nil
					}
				}

				queryId = *query.ID
				if query.Index != nil {
					index = *query.Index
				}
				// Extract query from translated ES query
				_, esQuery, _, err := querytranslate.TranslateQuery(*rsQuery, ip, query.ID, nil)
				if err != nil {
					log.Errorln(logTag, ":", err)
					return nil, false, &Error{
						Err: err,
					}
				}
				var queryJSON map[string]interface{}
				err2 := json.Unmarshal(esQuery, &queryJSON)
				if err2 != nil {
					log.Errorln(logTag, ":", err2)
					return nil, false, &Error{
						Err: err2,
					}
				}
				queryValue := queryJSON["query"]
				if queryValue == nil {
					queryValue = map[string]interface{}{
						"match_all": map[string]interface{}{},
					}
				}

				log.Debug(logTag, ": query value: ", queryValue)

				// Determine the type of boost query based on the passed value.
				_, boostQueryGenerated, boostQueryGenerateErr := getBoostQuery(&parsedBoostInputs)
				if boostQueryGenerateErr != nil {
					return nil, false, boostQueryGenerateErr
				}

				log.Debug(logTag, ": boost query: ", boostQueryGenerated)

				boostQueryMap := map[string]interface{}{
					"query": map[string]interface{}{
						"bool": map[string]interface{}{
							"must": []interface{}{
								queryValue,
								boostQueryGenerated,
							},
						},
					},
				}
				for k, v := range queryJSON {
					if !util.Contains([]string{"query", "aggs", "size", "from"}, k) {
						boostQueryMap[k] = v
					}
				}
				var boostQuerySize = size
				if computedSize != nil && *computedSize < size {
					boostQuerySize = *computedSize
				}
				boostQueryMap["size"] = boostQuerySize
				marshalledQuery, err3 := json.Marshal(boostQueryMap)
				if err3 != nil {
					log.Errorln(logTag, ":", err3)
					return nil, false, &Error{
						Err: err3,
					}
				}
				boostQuery = string(marshalledQuery)

			}
			if boostQuery != "" {
				// Write the boostQuery so that it can be used

				boostMap := BoostStageResponse{
					Index:   index,
					QueryId: queryId,
					Query:   boostQuery,
					Inputs:  parsedBoostInputs,
				}
				mapAsStr, marshalErr := json.Marshal(boostMap)
				if marshalErr != nil {
					errMsg := fmt.Sprint("error while marshalling boost stage: ", marshalErr.Error())
					log.Errorln(logTag, ": ", errMsg)
					return nil, false, &Error{
						Err: fmt.Errorf(errMsg),
					}
				}

				output := map[string]interface{}{
					*stageId + "__boost": string(mapAsStr),
				}
				contextInBytes, err := json.Marshal(output)
				if err != nil {
					log.Errorln(logTag, ":", err)
					return nil, false, &Error{
						Err: err,
					}
				}
				// return response
				return contextInBytes, false, nil
			}
		}
		return scriptContextInBytes, false, nil
	}
	return scriptContextInBytes, false, nil
}

// Returns the first type search query with execute true
func getBoostQueryIndex(rsAPI *querytranslate.RSQuery) int {
	if rsAPI == nil {
		return -1
	}
	for i, query := range rsAPI.Query {
		if query.Type == querytranslate.Search {
			if query.Execute == nil || *query.Execute {
				return i
			}
		}
	}
	return -1
}

func getComputedSize(query querytranslate.Query) *int {
	if query.From != nil && *query.From > 0 {
		size := *query.From
		if query.Size != nil {
			size += *query.Size
		} else {
			// add default size as 10
			size += 10
		}
		return &size
	}
	return nil
}

// getBoostQuery will determine the type of boost query
// and accordingly return the type, the built query and errors
// if any are found.
func getBoostQuery(queryInputs *BoostStageInputStruct) (*querytranslate.QueryType, interface{}, *Error) {
	// We will determine the type of the query depending on the
	// type of the incoming value.
	//
	// If the value is an array -> `term` or `search`
	// If the value is an object -> `geo` or `range`
	//
	// If the `object` contains the keys `start` or `end`, it
	// will be a range query and otherwise it will be a distance
	// query if the `location` key is present.
	//
	// We do not support other types of values here so we will
	// just throw an error if that happens.

	if queryInputs.Value == nil {
		// Value cannot be nil.
		errMsg := fmt.Errorf("`value` cannot be empty for boost stage inputs")
		return nil, nil, &Error{
			Err:  errMsg,
			Code: http.StatusBadRequest,
		}
	}

	valueAsArr, asArrOk := (*queryInputs.Value).([]interface{})
	if asArrOk {
		// Seems like value is an array so we will need to parse it
		// into an array of strings which is the only type
		// of array we support.
		valueAsStrArr := make([]string, 0)
		for _, valueAsInterface := range valueAsArr {
			valueAsStr, asStrOk := valueAsInterface.(string)
			if !asStrOk {
				continue
			}

			valueAsStrArr = append(valueAsStrArr, valueAsStr)
		}

		// If it's an empty array, we need to throw an error
		if len(valueAsStrArr) == 0 {
			return nil, nil, &Error{
				Err:  fmt.Errorf("`value` should be a non-empty array of strings, found empty"),
				Code: http.StatusBadRequest,
			}
		}

		queryType := querytranslate.Search

		// Generate the query
		boostQuery, generateErr := generateBoostQueryByType(queryInputs, queryType, strings.Join(valueAsStrArr, " "))
		return &queryType, boostQuery, generateErr
	}

	// Value can be both an object or a string
	valueAsMap, asMapOk := (*queryInputs.Value).(map[string]interface{})
	if asMapOk {
		// Determine whether this is for `range` or `geo` and accordingly
		// build and return.

		// Check if it is of type range. If the map contains either of `start`
		// or `end` then the value will be for `range` type of query boosting.
		queryType := querytranslate.Search

		for key, _ := range valueAsMap {
			if key == "start" || key == "end" {
				queryType = querytranslate.Range
				break
			}

			if key == "location" {
				queryType = querytranslate.Geo
				break
			}
		}

		// At this point, if the `value` is valid, then queryType should be
		// one of `range` or `geo`. If it is of type `search` which was the
		// default value, then we need to throw an error.
		if queryType == querytranslate.Search {
			return nil, nil, &Error{
				Err:  fmt.Errorf("`value` should contain proper keys if passed as an object. None Found."),
				Code: http.StatusBadRequest,
			}
		}

		// Generate the query here
		boostQuery, generateErr := generateBoostQueryByType(queryInputs, queryType, valueAsMap)
		return &queryType, boostQuery, generateErr
	}

	// We will not support any other type of value so just throw an error
	// right away.
	errMsg := fmt.Errorf("`value` can be one of array of object, other types are not supported for boost stage inputs")
	return nil, nil, &Error{
		Err:  errMsg,
		Code: http.StatusBadRequest,
	}
}

// generateBoostQueryByType will generate the query to use for boosting
// based on the passed type
func generateBoostQueryByType(queryInputs *BoostStageInputStruct, boostQueryType querytranslate.QueryType, value interface{}) (interface{}, *Error) {
	switch boostQueryType {
	case querytranslate.Term, querytranslate.Search:
		boostQuery := map[string]interface{}{
			"match": map[string]interface{}{
				*queryInputs.DataField: map[string]interface{}{
					"query":    value,
					"operator": queryInputs.QueryFormat.String(),
				},
			},
		}
		return boostQuery, nil
	case querytranslate.Range:
		// Parse the gte and lte values
		valueAsMap, asMapOk := value.(map[string]interface{})
		if !asMapOk {
			return nil, &Error{
				Err:  fmt.Errorf("`value` passed is not a map, cannot continue"),
				Code: http.StatusInternalServerError,
			}
		}

		gte, isGtePresent := valueAsMap["start"]
		lte, isLtePresent := valueAsMap["end"]

		if !isGtePresent && !isLtePresent {
			return nil, &Error{
				Err:  fmt.Errorf("error parsing `start` and `end` from value, neither is present"),
				Code: http.StatusBadRequest,
			}
		}

		rangeValueMap := map[string]interface{}{}
		if isGtePresent {
			rangeValueMap["gte"] = gte
		}
		if isLtePresent {
			rangeValueMap["lte"] = lte
		}

		boostQuery := map[string]interface{}{
			"range": map[string]interface{}{
				*queryInputs.DataField: rangeValueMap,
			},
		}
		return boostQuery, nil
	case querytranslate.Geo:
		// Parse the gte and lte values
		valueAsMap, asMapOk := value.(map[string]interface{})
		if !asMapOk {
			return nil, &Error{
				Err:  fmt.Errorf("`value` passed is not a map, cannot continue"),
				Code: http.StatusInternalServerError,
			}
		}

		// Make sure all required keys are present which would be
		// `distance`, `unit` and `location`.
		distance, isDistancePresent := valueAsMap["distance"]
		unit, isUnitPresent := valueAsMap["unit"]
		location, isLocationPresent := valueAsMap["location"]

		if !isDistancePresent || !isUnitPresent || !isLocationPresent {
			return nil, &Error{
				Err:  fmt.Errorf("`distance`, `unit` and `location` is a required value"),
				Code: http.StatusBadRequest,
			}
		}

		distanceAsFloat, asFloatOk := distance.(float64)
		if !asFloatOk {
			return nil, &Error{
				Err:  fmt.Errorf("`distance` should be a number, non number value passed"),
				Code: http.StatusBadRequest,
			}
		}

		distanceAsInt := int(distanceAsFloat)
		unitAsStr, asStrOk := unit.(string)
		if !asStrOk {
			return nil, &Error{
				Err:  fmt.Errorf("`unit` should be a string, non string value passed"),
				Code: http.StatusBadRequest,
			}
		}

		boostQuery := map[string]interface{}{
			"geo_distance": map[string]interface{}{
				"distance":             strconv.Itoa(distanceAsInt) + unitAsStr,
				*queryInputs.DataField: location,
			},
		}
		return boostQuery, nil
	default:
		return nil, &Error{
			Err:  fmt.Errorf("Invalid type passed for boosting"),
			Code: http.StatusBadRequest,
		}
	}
}
