package function

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"

	"github.com/prometheus/common/log"
)

const (
	LogTag             = "[functions]"
	EnvOpenFaasGateway = "OPENFAAS_GATEWAY"
	XSearchQuery       = "X-Search-Query"
	XSearchFilters     = "X-Search-Filters"
)

type TriggerType int

const (
	Always TriggerType = iota
	Filter
)

// String is the implementation of Stringer interface that returns the string representation of TriggerType type.
func (o TriggerType) String() string {
	return [...]string{
		"always",
		"filter",
	}[o]
}

// UnmarshalJSON is the implementation of the Unmarshaler interface for unmarshaling TiggerType type.
func (o *TriggerType) UnmarshalJSON(bytes []byte) error {
	var triggerType string
	err := json.Unmarshal(bytes, &triggerType)
	if err != nil {
		return err
	}
	switch triggerType {
	case Always.String():
		*o = Always
	case Filter.String():
		*o = Filter
	default:
		return fmt.Errorf("invalid triggerType encountered: %v", triggerType)
	}
	return nil
}

// MarshalJSON is the implementation of the Marshaler interface for marshaling TiggerType type.
func (o TriggerType) MarshalJSON() ([]byte, error) {
	var triggerType string
	switch o {
	case Always:
		triggerType = Always.String()
	case Filter:
		triggerType = Filter.String()
	default:
		return nil, fmt.Errorf("invalid triggerType encountered: %v", o)
	}
	return json.Marshal(triggerType)
}

type Trigger struct {
	Type          *TriggerType `json:"type"`
	Expression    string       `json:"expression"`
	ExecuteBefore bool         `json:"executeBefore"`
}

// FunctionOpenFaas represents the Open faas function struct
type FunctionOpenFaas struct {
	Service                string                 `json:"service"`
	Network                string                 `json:"network"`
	Image                  string                 `json:"image"`
	EnvProcess             string                 `json:"envProcess"`
	EnvVars                map[string]interface{} `json:"envVars"`
	Constraints            []string               `json:"constraints"`
	Labels                 map[string]interface{} `json:"labels"`
	Annotations            map[string]interface{} `json:"annotations"`
	Secrets                []string               `json:"secrets"`
	RegistryAuth           string                 `json:"registryAuth"`
	Limits                 map[string]interface{} `json:"limits"`
	Requests               map[string]interface{} `json:"requests"`
	ReadOnlyRootFilesystem bool                   `json:""readOnlyRootFilesystem"`
}

type ESFunctionDoc struct {
	Enabled             bool                    `json:"enabled"`
	Trigger             *Trigger                `json:"trigger,omitempty"`
	ExtraRequestPayload *map[string]interface{} `json:"extraRequestPayload,omitempty"`
	Function            FunctionOpenFaas        `json:"function,omitempty"`
	InvocationCount     *int                    `json:"invocationCount,omitempty"`
}

type InvokeRequest struct {
	URL     string                  `json:"url"`
	Method  string                  `json:"method"`
	Headers *map[string]string      `json:"headers"`
	Body    *map[string]interface{} `json:"body"`
}

type InvokeResponse struct {
	Body    *map[string]interface{} `json:"body"`
	Headers *map[string]string      `json:"headers"`
	Status  string                  `json:"status"`
}

type TriggerEnvironments struct {
	ACL      string              `json:"acl"`
	Category string              `json:"category"`
	Index    []string            `json:"index"`
	Filter   []map[string]string `json:"filter"`
	Query    string              `json:"query"`
	Now      int64               `json:"now"`
}

type InvokeFunctionBody struct {
	ExtraRequestPayload map[string]interface{} `json:"extraRequestPayload"`
	Request             *InvokeRequest         `json:"request,omitempty"`
	Response            *InvokeResponse        `json:"response,omitempty"`
	Environments        TriggerEnvironments    `json:"env"`
}

// cachedFunctions represents the struct of a list of saved functions in the .functions index
var cachedFunctions []ESFunctionDoc

// SetFunctionsToCache sets the functions
func SetFunctionsToCache(functions []ESFunctionDoc) {
	cachedFunctions = functions
}

// GetFunctionsFromCache returns a list of cached functions
func GetFunctionsFromCache() []ESFunctionDoc {
	return cachedFunctions
}

// AddFunctionToCache adds a function
func AddFunctionToCache(function ESFunctionDoc) {
	cachedFunctions = append(cachedFunctions, function)
}

// UpdateFunctionToCache updates a function
func UpdateFunctionToCache(function ESFunctionDoc) bool {
	_, loc := IsFunctionExistsInCache(function.Function.Service)
	if loc != nil {
		cachedFunctions[*loc] = function
		return true
	}
	return false
}

// DeleteFunctionToCache deletes a function
func DeleteFunctionToCache(functionID string) bool {
	_, loc := IsFunctionExistsInCache(functionID)
	if loc != nil {
		cachedFunctions = append(cachedFunctions[:*loc], cachedFunctions[*loc+1:]...)
		return true
	}
	return false
}

// GetFunctionFromCache returns a function by ID
func GetFunctionFromCache(functionID string) *ESFunctionDoc {
	function, _ := IsFunctionExistsInCache(functionID)
	return function
}

// IsFunctionExistsInCache checks if a function in present in cache
func IsFunctionExistsInCache(functionID string) (*ESFunctionDoc, *int) {
	for loc, function := range cachedFunctions {
		if function.Function.Service == functionID {
			return &function, &loc
		}
	}
	return nil, nil
}

func GetOpenFaasURL() string {
	return os.Getenv(EnvOpenFaasGateway)
}

func MakeOpenFaasRequest(url, method string, reqBody []byte) ([]byte, *http.Response, error) {
	finalURL := GetOpenFaasURL() + url
	request, err := http.NewRequest(method, finalURL, bytes.NewReader(reqBody))
	if err != nil {
		log.Errorln(LogTag, ":", err)
		return nil, nil, err
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		log.Errorln(LogTag, ":", err)
		return nil, nil, err
	}

	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		log.Errorln(LogTag, ":", err)
		return nil, nil, err
	}
	return body, response, nil
}
