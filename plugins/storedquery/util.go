package storedquery

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/appbaseio-confidential/reactivesearch/plugins/querytranslate"
	log "github.com/sirupsen/logrus"
)

// ESStoredQueryRequest represents the request body for PUT stored query route
type ESStoredQueryRequest struct {
	ID          *string                 `json:"id,omitempty"`
	Index       *string                 `json:"index,omitempty"`
	Description *string                 `json:"description,omitempty"`
	Query       *string                 `json:"query,omitempty"`
	Params      *map[string]interface{} `json:"params,omitempty"`
}

// ESStoredQueryDOC represents the shape of a stored query doc stored in ES
type ESStoredQueryDOC struct {
	ID          *string                 `json:"id,omitempty"`
	Index       *string                 `json:"index,omitempty"`
	Description *string                 `json:"description,omitempty"`
	Query       *string                 `json:"query,omitempty"`
	Params      *map[string]interface{} `json:"params,omitempty"`
	CreatedAt   *int64                  `json:"created_at,omitempty"`
	UpdatedAt   *int64                  `json:"updated_at,omitempty"`
}

type ESStoredQueryCached struct {
	ID           *string                 `json:"id,omitempty"`
	Index        *string                 `json:"index,omitempty"`
	Description  *string                 `json:"description,omitempty"`
	Query        *interface{}            `json:"query,omitempty"`
	QueryInBytes []byte                  `json:"queryInBytes,omitempty"`
	Params       *map[string]interface{} `json:"params,omitempty"`
	CreatedAt    *int64                  `json:"created_at,omitempty"`
	UpdatedAt    *int64                  `json:"updated_at,omitempty"`
}

type ESStoredQueryGET struct {
	ID          *string                 `json:"id,omitempty"`
	Index       *string                 `json:"index,omitempty"`
	Description *string                 `json:"description,omitempty"`
	Query       *interface{}            `json:"query,omitempty"`
	Params      *map[string]interface{} `json:"params,omitempty"`
	CreatedAt   *int64                  `json:"created_at,omitempty"`
	UpdatedAt   *int64                  `json:"updated_at,omitempty"`
}

// ESStoredQueryRequestBody represents the shape of the request body for stored query
type ESStoredQueryRequestBody struct {
	Index       *string                 `json:"index,omitempty"`
	Description *string                 `json:"description,omitempty"`
	Query       *interface{}            `json:"query,omitempty"`
	Params      *map[string]interface{} `json:"params,omitempty"`
}

type ValidateQueryRequestBody struct {
	Query  *interface{}            `json:"query,omitempty"`
	Params *map[string]interface{} `json:"params,omitempty"`
}

type ValidateQueryRequestBodyID struct {
	Params *map[string]interface{} `json:"params,omitempty"`
}

type ExecuteQueryRequestBody struct {
	Query  *interface{}            `json:"query,omitempty"`
	Params *map[string]interface{} `json:"params,omitempty"`
	Index  *string                 `json:"index,omitempty"`
}
type ExecuteQueryRequestBodyID struct {
	Params *map[string]interface{} `json:"params,omitempty"`
}

type ValidateResponse struct {
	Valid bool `json:"valid"`
}

func validateStoredQuery(requestBody ESStoredQueryDOC) error {
	if requestBody.ID == nil || *requestBody.ID == "" {
		return errors.New("id can not be empty")
	}
	if requestBody.Index == nil || *requestBody.Index == "" {
		return errors.New("index can not be empty")
	}
	// Validate actions
	if requestBody.Query == nil {
		return errors.New("query can not be empty")
	}
	return nil
}

func getParams(defaultParams *map[string]interface{}, requestParams *map[string]interface{}) map[string]interface{} {
	mergedParams := make(map[string]interface{})
	if defaultParams != nil && requestParams != nil {
		// clone map
		for k, v := range *defaultParams {
			mergedParams[k] = v
		}
		// clone map and override existing params
		for k, v := range *requestParams {
			mergedParams[k] = v
		}
		return mergedParams
	}
	if requestParams != nil {
		// clone map
		for k, v := range *requestParams {
			mergedParams[k] = v
		}
		return mergedParams
	}
	if defaultParams != nil {
		// clone map
		for k, v := range *defaultParams {
			mergedParams[k] = v
		}
		return mergedParams
	}
	return make(map[string]interface{})
}

func flattenReact(react interface{}) []string {
	nestedReact, isNestedReact := react.(map[string]interface{})
	if isNestedReact {
		react := make([]string, 0)
		// handle react prop as struct
		if nestedReact["and"] != nil {
			react = append(react, flattenReact(nestedReact["and"])...)
		}
		if nestedReact["or"] != nil {
			react = append(react, flattenReact(nestedReact["or"])...)
		}
		if nestedReact["not"] != nil {
			react = append(react, flattenReact(nestedReact["not"])...)
		}
		return react
	} else {
		// handle react prop as an array
		reactAsArray, isArray := react.([]interface{})
		if isArray {
			react := make([]string, 0)
			for _, comp := range reactAsArray {
				componentID, isString := comp.(string)
				if isString {
					react = append(react, componentID)
				} else {
					react = append(react, flattenReact(comp)...)
				}
			}
			return react

		} else {
			// handle react prop as string
			reactAsString, isString := react.(string)
			if isString {
				return []string{reactAsString}
			}
		}
	}
	return make([]string, 0)
}

func renderQuery(queryInBytes []byte, params map[string]interface{}) (string, error) {
	queryAsString := string(queryInBytes)
	for name, value := range params {
		byteVal, err := json.Marshal(value)
		if err != nil {
			log.Errorln(logTag, ":", err)
			return "", nil
		}
		tag := fmt.Sprintf(`"{{%s}}"`, name)
		queryAsString = strings.Replace(queryAsString, tag, string(byteVal), -1)
	}
	return queryAsString, nil
}

// extracts the stored queries from the RS API request body
func getStoredQueries(query querytranslate.RSQuery) []string {
	var storedqueries = make(map[string]interface{})
	for _, q := range query.Query {
		if q.DefaultQuery != nil {
			query := *q.DefaultQuery
			id := query["id"]
			if id != nil {
				idAsString, ok := id.(string)
				if ok && idAsString != "" {
					storedqueries[idAsString] = true
				}
			}
		}
		if q.CustomQuery != nil {
			query := *q.CustomQuery
			id := query["id"]
			if id != nil {
				idAsString, ok := id.(string)
				if ok && idAsString != "" {
					storedqueries[idAsString] = true
				}
			}
		}
	}
	var storedqueriesArray = make([]string, 0)
	for k := range storedqueries {
		storedqueriesArray = append(storedqueriesArray, k)
	}
	return storedqueriesArray
}
