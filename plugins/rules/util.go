package rules

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	_ "embed"

	"github.com/antonmedv/expr"
	"github.com/appbaseio/reactivesearch-api/model/acl"
	"github.com/appbaseio/reactivesearch-api/model/category"
	"github.com/appbaseio/reactivesearch-api/model/index"
	"github.com/appbaseio/reactivesearch-api/plugins/openai"
	"github.com/appbaseio/reactivesearch-api/plugins/querytranslate"
	"github.com/appbaseio/reactivesearch-api/util"
	"github.com/appbaseio/reactivesearch-api/util/iplookup"
	"github.com/invopop/jsonschema"
	"github.com/kr/pretty"
	"github.com/robfig/cron"
	log "github.com/sirupsen/logrus"
	base64Polyfill "go.kuoruan.net/v8go-polyfills/base64"
	"go.kuoruan.net/v8go-polyfills/console"
	"rogchap.com/v8go"
)

//go:embed packages/compromise.txt
var compromiseScript string

//go:embed packages/compromise-dates.txt
var compromiseDatesScript string

//go:embed packages/compromise-numbers.txt
var compromiseNumbersScript string

//go:embed packages/lodash.txt
var lodashScript string

//go:embed packages/crypto.txt
var cryptoScript string

//go:embed packages/mongo-db.txt
var mongoDBQueryTranslateScript string

type ActionType int

const (
	PromoteResult      ActionType = iota // after
	HideResult                           // after
	CustomData                           // after
	ReplaceSearchTerm                    // before
	AddFilter                            // before
	SearchSettings                       // before
	RemoveWords                          // before
	ReplaceWords                         // before
	ReplaceSearchQuery                   // before
	Script
)

// String is the implementation of Stringer interface that returns the string representation of ActionType type.
func (o ActionType) String() string {
	return [...]string{
		"promote_result",
		"hide_result",
		"custom_data",
		"replace_search_term",
		"add_filter",
		"search_settings",
		"remove_words",
		"replace_words",
		"replace_search_query",
		"script",
	}[o]
}

// UnmarshalJSON is the implementation of the Unmarshaler interface for unmarshaling ActionType type.
func (o *ActionType) UnmarshalJSON(bytes []byte) error {
	var actionType string
	err := json.Unmarshal(bytes, &actionType)
	if err != nil {
		return err
	}
	switch actionType {
	case PromoteResult.String():
		*o = PromoteResult
	case HideResult.String():
		*o = HideResult
	case CustomData.String():
		*o = CustomData
	case ReplaceSearchTerm.String():
		*o = ReplaceSearchTerm
	case RemoveWords.String():
		*o = RemoveWords
	case ReplaceWords.String():
		*o = ReplaceWords
	case AddFilter.String():
		*o = AddFilter
	case SearchSettings.String():
		*o = SearchSettings
	case ReplaceSearchQuery.String():
		*o = ReplaceSearchQuery
	case Script.String():
		*o = Script
	default:
		return fmt.Errorf("invalid actionType encountered: %v", actionType)
	}
	return nil
}

// MarshalJSON is the implementation of the Marshaler interface for marshaling ActionType type.
func (o ActionType) MarshalJSON() ([]byte, error) {
	var actionType string
	switch o {
	case PromoteResult:
		actionType = PromoteResult.String()
	case HideResult:
		actionType = HideResult.String()
	case ReplaceSearchTerm:
		actionType = ReplaceSearchTerm.String()
	case RemoveWords:
		actionType = RemoveWords.String()
	case ReplaceWords:
		actionType = ReplaceWords.String()
	case CustomData:
		actionType = CustomData.String()
	case AddFilter:
		actionType = AddFilter.String()
	case SearchSettings:
		actionType = SearchSettings.String()
	case ReplaceSearchQuery:
		actionType = ReplaceSearchQuery.String()
	case Script:
		actionType = Script.String()
	default:
		return nil, fmt.Errorf("invalid actionType encountered: %v", o)
	}
	return json.Marshal(actionType)
}

type Action struct {
	Type         *ActionType `json:"type,omitempty"`
	Data         *string     `json:"data,omitempty"`   // we store the stringified data in the ES
	Script       *string     `json:"script,omitempty"` // script has different data type (binary)
	DecodeScript *string
	Request      *ScriptRequest         `json:"request,omitempty"`
	Response     *ScriptResponse        `json:"response,omitempty"`
	Environments map[string]interface{} `json:"envs,omitempty"`
}

type RequestAction struct {
	Type         *ActionType            `json:"type,omitempty"`
	Data         *interface{}           `json:"data,omitempty"` // we store the stringified data in the ES
	Script       *string                `json:"script,omitempty"`
	Request      *ScriptRequest         `json:"request,omitempty"`
	Response     *ScriptResponse        `json:"response,omitempty"`
	Environments map[string]interface{} `json:"envs,omitempty"`
}

type TimeFrame struct {
	StartTime *int64 `json:"start_time,omitempty"`
	EndTime   *int64 `json:"end_time,omitempty"` // EndTime could be optional when user wants to start this trigger after specific date forever
}

type TriggerType int

const (
	Always TriggerType = iota
	Filter
	Index
	Query
	Cron
)

// String is the implementation of Stringer interface that returns the string representation of TriggerType type.
func (o TriggerType) String() string {
	return [...]string{
		"always",
		"filter",
		"index",
		"query",
		"cron",
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
	case Index.String():
		*o = Index
	case Query.String():
		*o = Filter
	case Cron.String():
		*o = Cron
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
	case Index:
		triggerType = Index.String()
	case Query:
		triggerType = Filter.String()
	case Cron:
		triggerType = Cron.String()
	default:
		return nil, fmt.Errorf("invalid triggerType encountered: %v", o)
	}
	return json.Marshal(triggerType)
}

func (o TriggerType) JSONSchema() *jsonschema.Schema {
	triggerTypes := []interface{}{
		Always.String(),
		Filter.String(),
		Index.String(),
		Query.String(),
		Cron.String(),
	}
	return &jsonschema.Schema{
		Type: "string",
		Enum: triggerTypes,
	}
}

// Get the timeout in seconds for the passed trigger
func (o TriggerType) Timeout() time.Duration {
	var timeoutInSeconds time.Duration
	switch o {
	case Cron:
		// Should be 10 mins
		timeoutInSeconds = 10 * 60 * time.Second
	default:
		// Should be 5 mins
		timeoutInSeconds = 5 * time.Second
	}
	return timeoutInSeconds
}

type Trigger struct {
	Type       *TriggerType `json:"type,omitempty" jsonschema:"title=Trigger Type" jsonschema_description:"Type of trigger expression. You can read more at [here](https://docs.reactivesearch.io/docs/search/rules/#configure-if-condition)."`
	Expression string       `json:"expression,omitempty" jsonschema:"title=Trigger Expression" jsonschema_description:"Custom trigger expression. You can read more at [here](https://docs.reactivesearch.io/docs/search/rules/#advanced-editor)."`
	TimeFrame  *TimeFrame   `json:"timeframe,omitempty" jsonschema:"title=Timeframe" jsonschema_description:"To define the valid timeframe for trigger expression."`
}

type ScriptRequest struct {
	Body    string            `json:"body"`
	Headers map[string]string `json:"headers"`
	URL     string            `json:"url"`
	Method  string            `json:"method"`
}

type ScriptResponse struct {
	Code    int               `json:"code"`
	Body    string            `json:"body"`
	Headers map[string]string `json:"headers"`
}

type ValidateScriptResponse struct {
	Request    *ScriptRequest  `json:"request"`
	Response   *ScriptResponse `json:"response"`
	ScriptTook int             `json:"script_took"`
	Console    []string        `json:"console_logs"`
}

// ValidateScriptError is similar to what telemetry writes
// back when an error occurs but also contains a console
// field to indicate console logs
//
// This struct should be used as value for the `error` field
type ValidateScriptError struct {
	Code    int    `json:"code"`
	Status  string `json:"status"`
	Message string `json:"message"`
}

type ValidateScriptResponseWrapper struct {
	Request    *ScriptRequestIn  `json:"request"`
	Response   *ScriptResponseIn `json:"response"`
	ScriptTook int               `json:"script_took"`
	Console    []string          `json:"console_logs"`
}

type ValidateScriptRequest struct {
	Script   string                 `json:"script"` // required
	Envs     map[string]interface{} `json:"envs"`
	Request  *ScriptRequest         `json:"request"`
	Response *ScriptResponse        `json:"response"`
	IsCron   *bool                  `json:"isCron"`
}

type ScriptRequestIn struct {
	Body    interface{}       `json:"body"`
	Headers map[string]string `json:"headers"`
}

type ScriptResponseIn struct {
	Code    int               `json:"code"`
	Body    interface{}       `json:"body"`
	Headers map[string]string `json:"headers"`
	Console []string          `json:"console"`
}

type ValidateScriptRequestWrapper struct {
	Script   string                 `json:"script"` // required
	Envs     map[string]interface{} `json:"envs"`
	Request  *ScriptRequestIn       `json:"request"`
	Response *ScriptResponseIn      `json:"response"`
	IsCron   *bool                  `json:"isCron"`
}

type ScriptContext struct {
	Request      ScriptRequest          `json:"request"`
	Response     ScriptResponse         `json:"response"`
	Environments map[string]interface{} `json:"envs"`
}

type RuleContext struct {
	Envs map[string]interface{} `json:"envs"`
}

type ScriptData struct {
	Script string `json:"script"`
}

// ESRuleDoc represents the shape of a rule doc stored in ES
type ESRuleDoc struct {
	ID                *string   `json:"id,omitempty"`
	Name              *string   `json:"name,omitempty"`
	Description       *string   `json:"description,omitempty"`
	Enabled           *bool     `json:"enabled,omitempty"`
	Order             *int      `json:"order,omitempty"`
	UpdatedAt         *int64    `json:"updated_at,omitempty"`
	CreatedAt         *int64    `json:"created_at,omitempty"`
	Trigger           *Trigger  `json:"trigger,omitempty"`
	Actions           *[]Action `json:"actions,omitempty"`
	ShowAdvanceEditor *bool     `json:"show_advance_editor,omitempty"`
}

type ESRuleRequestBody struct {
	ID                *string          `json:"id,omitempty"`
	Name              *string          `json:"name,omitempty"`
	Description       *string          `json:"description,omitempty"`
	Enabled           *bool            `json:"enabled,omitempty"`
	Order             *int             `json:"order,omitempty"`
	UpdatedAt         *int64           `json:"updated_at,omitempty"`
	CreatedAt         *int64           `json:"created_at,omitempty"`
	Trigger           *Trigger         `json:"trigger,omitempty"`
	Actions           *[]RequestAction `json:"actions,omitempty"`
	ShowAdvanceEditor *bool            `json:"show_advance_editor,omitempty"`
}

type PromotedResult struct {
	Doc      map[string]interface{} `json:"doc"`
	Position int                    `json:"position"`
}
type PromotedResultSuggestionDoc struct {
	Id     string                 `json:"_id"`
	Source map[string]interface{} `json:"_source"`
	Index  string                 `json:"_index"`
	Label  string                 `json:"_suggestion_display_value"`
	URL    string                 `json:"_suggestion_url"`
}
type PromotedResultSuggestion struct {
	Doc PromotedResultSuggestionDoc `json:"doc"`
}

// TODO: Change the struct for filters to support multi value filters
type TriggerEnvironmentsToEvaluate struct {
	Index        []string            `json:"index,omitempty"`
	Filter       map[string]string   `json:"filters,omitempty"`
	Query        string              `json:"query,omitempty"`
	Type         string              `json:"type,omitempty"`         // search|term|suggestion|geo|range
	Origin       string              `json:"origin,omitempty"`       // https://my-search.domain.com
	Referer      string              `json:"referer,omitempty"`      // https://my-search.domain.com/path?q=hello
	Path         string              `json:"path,omitempty"`         // /_doc/nested_something
	Method       []string            `json:"method,omitempty"`       // GET/POST
	URLValues    map[string]string   `json:"urlValues,omitempty"`    // {'q': 'hello'}
	Category     string              `json:"category,omitempty"`     // docs
	ACL          string              `json:"acl,omitempty"`          // bulk
	IPv4         string              `json:"ipv4,omitempty"`         // 29.120.12.12
	IPv6         string              `json:"ipv6,omitempty"`         // 2001:db8:3333:4444:5555:6666:7777:8888
	CustomEvents map[string]string   `json:"customEvents,omitempty"` // { 'platform': 'mac' }
	OpenAIConfig openai.OpenAIConfig `json:"openAIConfig,omitempty"`
}

var MapToParseExpressionToStructKeys = map[string]string{
	"$index":        "Index",
	"$filter":       "Filter",
	"$query":        "Query",
	"$type":         "Type",
	"$origin":       "Origin",
	"$referer":      "Referer",
	"$path":         "Path",
	"$category":     "Category",
	"$acl":          "ACL",
	"$ipv4":         "IPv4",
	"$ipv6":         "IPv6",
	"$customEvents": "CustomEvents",
}

var MapToParseIndexExpressionToStructKeys = map[string]string{
	"$index":        "Index",
	"$type":         "Type",
	"$origin":       "Origin",
	"$referer":      "Referer",
	"$path":         "Path",
	"$category":     "Category",
	"$acl":          "ACL",
	"$ipv4":         "IPv4",
	"$ipv6":         "IPv6",
	"$customEvents": "CustomEvents",
}

func getMasterCredentials() string {
	username, password := os.Getenv("USERNAME"), os.Getenv("PASSWORD")
	if username == "" {
		username, password = "foo", "bar"
	}
	return username + ":" + password
}

// returns the size limit by plan
func getSizeLimitByPlan() int64 {
	if util.GetTier() != nil {
		switch util.GetTier().String() {
		case util.ProductionFirst2019.String(),
			util.ProductionFirst2021.String(),
			util.ProductionFirst2023.String():
			return 100
		case util.ArcEnterprise.String(),
			util.HostedArcEnterprise.String(),
			util.HostedArcEnterprise2021.String(),
			util.ProductionSecond2019.String(),
			util.ProductionThird2019.String(),
			util.ProductionSecond2021.String(),
			util.ProductionThird2021.String(),
			util.ProductionFourth2019.String():
			return 1000
		default:
			return 10
		}
	}
	return 10
}

const filterReplacer = "__"

func dotReplacer(str string) string {
	return strings.Replace(str, ".", filterReplacer, -1)
}

func parseTriggerExpression(expression string, triggerType TriggerType) string {
	/**
	Parse filter, replace '.' to '__' so we can compare nested fields.
	For e.g `$filter.user.name.keyword matches 2011` will be converted to `$filter.user__name__keyword matches 2011`
	*/
	regexToFindFilterEnv := regexp.MustCompile(`\$filter\["[\w()./\- ]*"\]|\$filter\['[\w()./\- ]*'\]|\$filter[.\w()\-_]*`)
	expression = regexToFindFilterEnv.ReplaceAllStringFunc(expression, dotReplacer)
	var parsedExpression = strings.Replace(expression, "$filter"+filterReplacer, "$filter.", -1)

	var mapToUse map[string]string

	switch triggerType {
	case Index:
		mapToUse = MapToParseIndexExpressionToStructKeys
	default:
		mapToUse = MapToParseExpressionToStructKeys
	}

	for key, val := range mapToUse {
		parsedExpression = strings.Replace(parsedExpression, key, val, -1)
	}
	return parsedExpression
}

// ParseTriggerExpression parses the trigger expression and returns it
// as a string.
func ParseTriggerExpression(expression string, triggerType TriggerType) string {
	return parseTriggerExpression(expression, triggerType)
}

// Normalizes the filter keys i.e replaces the '.' with '__'
func ParseFilters(filters []querytranslate.TermFilter) map[string]string {
	var parsedFilters = make(map[string]string)
	for _, filter := range filters {
		parsedFilters[strings.Replace(filter.Key, ".", filterReplacer, -1)] = filter.Value
	}
	return parsedFilters
}

func validateTimeframe(rule ESRuleDoc) bool {
	if rule.Trigger != nil && rule.Trigger.TimeFrame != nil {
		currentTime := time.Now().Unix()
		// validate start time
		if rule.Trigger.TimeFrame.StartTime != nil {
			if currentTime < *rule.Trigger.TimeFrame.StartTime {
				return false
			}
		}
		// validate end time
		if rule.Trigger.TimeFrame.EndTime != nil {
			if currentTime > *rule.Trigger.TimeFrame.EndTime {
				return false
			}
		}
	}
	return true
}

// ValidateTriggerExpression will validate the expression for query type
func ValidateTriggerExpression(expression string) error {
	return validateTriggerExpression(expression)
}

// ValidateIndexTriggerExpression will validate the index trigger
// expression.
func ValidateIndexTriggerExpression(expression string) error {
	return validateIndexTriggerExpression(expression)
}

func validateTriggerExpression(expression string) error {
	// dummy env variables to validate an expression
	parsedEnvironments := TriggerEnvironmentsToEvaluate{
		Index: []string{"test1", "test2"},
		Filter: map[string]string{
			"filter1": "2011",
			"filter2": "product",
		},
		Query:        "harry",
		Type:         "search",
		Origin:       "https://my-search.domain.com",
		Referer:      "https://my-search.domain.com/path?q=hello",
		Path:         "/_doc/test",
		Category:     "docs",
		ACL:          "bulk",
		IPv4:         "29.120.12.12",
		IPv6:         "2001:db8:3333:4444:5555:6666:7777:8888",
		CustomEvents: map[string]string{"platform": "mac"},
	}
	_, err := expr.Compile(parseTriggerExpression(parseTriggerExprToLowerCase(expression), Filter), expr.Env(parsedEnvironments), expr.AsBool())
	return err
}

// Validate the expression for Index type of trigger
func validateIndexTriggerExpression(expression string) error {
	// dummy env variables to validate an expression
	parsedEnvironments := TriggerEnvironmentsToEvaluate{
		Index:        []string{"test1", "test2"},
		Type:         "update",
		Origin:       "https://my-search.domain.com",
		Referer:      "https://my-search.domain.com/path?q=hello",
		Path:         "/_doc/test",
		Category:     "docs",
		ACL:          "bulk",
		IPv4:         "29.120.12.12",
		IPv6:         "2001:db8:3333:4444:5555:6666:7777:8888",
		CustomEvents: map[string]string{"platform": "mac"},
	}
	_, err := expr.Compile(parseTriggerExpression(parseTriggerExprToLowerCase(expression), Index), expr.Env(parsedEnvironments), expr.AsBool())
	return err
}

// Validate the cron expression to make sure it's valid.
func validateCronTriggerExpression(expression string) error {
	// We just need to use the cron package to parse the expression.
	// If the expression is not valid, an error will be thrown and
	// we will know that the expression is invalid.
	_, err := cron.Parse(expression)
	return err
}

// validateFilter validates the trigger expression syntax
func validateFilter(ctx context.Context, r *http.Request, rule ESRuleDoc) (bool, error) {
	if rule.Trigger != nil && rule.Trigger.Type != nil {
		triggerType := rule.Trigger.Type
		if *triggerType == Filter {
			// Since it is a filter request, the request must be RSQuery, extract it from the
			// context
			req, err := querytranslate.FromContext(ctx)
			if err != nil {
				log.Error(logTag, ": Couldn't extract RSQuery from request")
				return false, err
			}

			environments := querytranslate.ExtractEnvsFromRequest(*req)

			parsedEnvironments, err := getTriggerEnvs(ctx, r, environments, *rule.Trigger.Type)
			if err != nil {
				return false, err
			}

			// Run for every query in the array
			for _, v := range req.Query {
				parsedEnvironments.Type = v.Type.String()
				program, err := expr.Compile(parseTriggerExpression(parseTriggerExprToLowerCase(rule.Trigger.Expression), *triggerType), expr.Env(parsedEnvironments), expr.AsBool())
				if err != nil {
					return false, err
				}
				output, err := expr.Run(program, *parsedEnvironments)
				if err != nil {
					return false, err
				}
				if output.(bool) {
					return true, nil
				}
			}
			return false, err
		} else if *triggerType == Index && rule.Trigger.Expression != "" {
			// This is for indexing requests
			var environments querytranslate.QueryEnvs

			parsedEnvironments, err := getTriggerEnvs(ctx, r, environments, *rule.Trigger.Type)
			if err != nil {
				return false, err
			}

			program, err := expr.Compile(parseTriggerExpression(parseTriggerExprToLowerCase(rule.Trigger.Expression), *triggerType), expr.Env(parsedEnvironments), expr.AsBool())
			if err != nil {
				return false, err
			}
			output, err := expr.Run(program, *parsedEnvironments)
			if err != nil {
				return false, err
			}
			if output.(bool) {
				return true, nil
			}
		}
	}

	return true, nil
}

// Returns an ip address (1 byte)
func GetClientIP4(ip string) string {
	parsedIP := net.ParseIP(ip)
	// Remove last byte
	ipv4 := parsedIP.To4()
	if ipv4 != nil {
		return ipv4.String()
	}
	return ""
}

// Returns an ip address without last 8 bits (1 byte)
func GetClientIP6(ip string) string {
	parsedIP := net.ParseIP(ip)
	// Remove last byte
	ipv6 := parsedIP.To16()
	ipv4 := parsedIP.To4()
	if ipv4 == nil && ipv6 != nil {
		return ipv6.String()
	}
	return ""
}

// To check if an item is present in a slice
func contains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}

func requestToESDoc(requestBody ESRuleRequestBody) (ESRuleDoc, error) {
	var actions []Action
	esRuleDoc := ESRuleDoc{
		ID:                requestBody.ID,
		Name:              requestBody.Name,
		Description:       requestBody.Description,
		Enabled:           requestBody.Enabled,
		Order:             requestBody.Order,
		UpdatedAt:         requestBody.UpdatedAt,
		CreatedAt:         requestBody.CreatedAt,
		Trigger:           requestBody.Trigger,
		ShowAdvanceEditor: requestBody.ShowAdvanceEditor,
	}
	if requestBody.Actions != nil {
		for _, action := range *requestBody.Actions {
			if action.Data != nil {
				byteValue, err := json.Marshal(*action.Data)
				if err != nil {
					log.Errorln(logTag, ":", err)
					return esRuleDoc, err
				}
				if action.Type != nil && *action.Type == PromoteResult {
					var promotedResults []PromotedResult
					err := json.Unmarshal(byteValue, &promotedResults)
					if err != nil {
						log.Errorln(logTag, ":", err)
						return esRuleDoc, err
					}
					// sort promoted results by position
					sort.Slice(promotedResults, func(i, j int) bool {
						return promotedResults[i].Position < promotedResults[j].Position
					})
					promotedResultsAsBytes, err := json.Marshal(promotedResults)
					if err != nil {
						log.Errorln(logTag, ":", err)
						return esRuleDoc, err
					}
					byteValue = promotedResultsAsBytes
				}
				actionDataAsString := string(byteValue)
				var script *string
				if action.Script != nil {
					// if script is equal to id then don't encode the script and set it from cache
					// it is because user when user updates a different action then script should not get changed
					if *action.Script == *requestBody.ID {
						rule := GetRuleFromCache(*requestBody.ID)
						if rule != nil && rule.Actions != nil {
							for _, action := range *rule.Actions {
								if action.Type != nil && *action.Type == Script {
									script = action.Script
								}
							}
						}
					} else {
						s := base64.StdEncoding.EncodeToString([]byte(*action.Script))
						script = &s
					}
				}
				actions = append(actions, Action{
					Type:         action.Type,
					Data:         &actionDataAsString,
					Script:       script,
					Request:      action.Request,
					Response:     action.Response,
					Environments: action.Environments,
				})
			} else {
				var script *string
				if action.Script != nil {
					// if script is equal to id then don't encode the script and set from cache
					// it is because user when user updates a different action then script should not get changed
					if *action.Script == *requestBody.ID {
						rule := GetRuleFromCache(*requestBody.ID)
						if rule != nil && rule.Actions != nil {
							for _, action := range *rule.Actions {
								if action.Type != nil && *action.Type == Script {
									script = action.Script
								}
							}
						}
					} else {
						s := base64.StdEncoding.EncodeToString([]byte(*action.Script))
						script = &s
					}
				}
				actions = append(actions, Action{
					Type:         action.Type,
					Script:       script,
					Request:      action.Request,
					Response:     action.Response,
					Environments: action.Environments,
				})
			}
		}
		esRuleDoc.Actions = &actions
	}

	return esRuleDoc, nil
}

func esDocToRequest(requestBody ESRuleDoc) (ESRuleRequestBody, error) {
	var actions []RequestAction
	esDoc := ESRuleRequestBody{
		ID:                requestBody.ID,
		Name:              requestBody.Name,
		Description:       requestBody.Description,
		Enabled:           requestBody.Enabled,
		Order:             requestBody.Order,
		UpdatedAt:         requestBody.UpdatedAt,
		CreatedAt:         requestBody.CreatedAt,
		Trigger:           requestBody.Trigger,
		ShowAdvanceEditor: requestBody.ShowAdvanceEditor,
	}
	if requestBody.Actions != nil {
		for _, action := range *requestBody.Actions {
			if action.Data != nil {
				dataInBytes := []byte(*action.Data)
				var data interface{}
				err := json.Unmarshal(dataInBytes, &data)
				if err != nil {
					log.Errorln(logTag, ":", err)
					return esDoc, err
				}
				var ruleScript *string
				if action.Script != nil {
					script, err := base64.StdEncoding.DecodeString(*action.Script)
					if err != nil {
						log.Errorln(logTag, ":", err)
						return esDoc, err
					}
					scriptAsString := string(script)
					ruleScript = &scriptAsString
				}
				actions = append(actions, RequestAction{
					Type:         action.Type,
					Data:         &data,
					Script:       ruleScript,
					Request:      action.Request,
					Response:     action.Response,
					Environments: action.Environments,
				})
			} else if action.Script != nil {
				script, err := base64.StdEncoding.DecodeString(*action.Script)
				if err != nil {
					log.Errorln(logTag, ":", err)
					return esDoc, err
				}
				scriptAsString := string(script)
				actions = append(actions, RequestAction{
					Type:         action.Type,
					Script:       &scriptAsString,
					Request:      action.Request,
					Response:     action.Response,
					Environments: action.Environments,
				})
			}
		}
		esDoc.Actions = &actions
	}

	return esDoc, nil
}

func getDefaultRuleOrder(rules []ESRuleDoc) (order int) {
	// initialize order
	order = len(rules) + 1
	for _, rule := range rules {
		if rule.Order != nil {
			// if any rule has the same order or has greater value than update the order
			if *rule.Order >= order {
				order = *rule.Order + 1
			}
		}
	}
	return
}

// Extracts the regexp from the trigger expression
func getRegexpFromTriggerExp(triggerExpression string) string {
	regexToFindQuerySyntax, err := regexp.Compile(`\$query\s*matches\s*"(.*?)"|\$query\s*matches\s*'(.*?)'`)
	if err != nil {
		log.Errorln(logTag, ":", err)
		return ""
	}
	queryMatches := regexToFindQuerySyntax.FindAllString(triggerExpression, -1)
	if len(queryMatches) <= 0 {
		return ""
	}
	regexToFindRegExp := regexp.MustCompile(`"(.*?)"|'(.*?)'`)
	regExp := regexToFindRegExp.FindString(queryMatches[0])
	log.Println("Query with matches clause", string(queryMatches[0]))
	log.Println("Regexp from trigger expression", regExp)
	return strings.Trim(strings.Trim(regExp, `'`), `"`)
}

// Finds all the single/double quoted strings having space as prefix (or suffix) and converts them to lower case
// Check the TestParseTriggerExprToLowerCase test cases for more info
func parseTriggerExprToLowerCase(expression string) string {
	re := regexp.MustCompile(` "(?:[^"\\]|\\.)*"| '(?:[^'\\]|\\.)*'|"(?:[^"\\]|\\.)*" |'(?:[^'\\]|\\.)*' `)
	return re.ReplaceAllStringFunc(expression, strings.ToLower)
}

// ParseTriggerExprToLowerCase finds all the single/double quoted strings having space as prefix
// (or suffix) and converts them to lower case
// Check the TestParseTriggerExprToLowerCase test cases for more info
func ParseTriggerExprToLowerCase(expression string) string {
	return parseTriggerExprToLowerCase(expression)
}

var mockRequest = `{
	"query":[
	   {
		  "id":"search",
		  "value":"vinyl",
		  "dataField":[
			 "name"
		  ],
		  "size":5,
		  "index":"best-buy-dataset",
		  "includeFields":[
			 "name",
			 "color"
		  ],
		  "react":{
			 "and":"color"
		  }
	   },
	   {
		  "id":"color",
		  "type":"term",
		  "dataField":"color.keyword",
		  "execute":false,
		  "value":"Black"
	   }
	]
 }`

var mockResponse = `{
	"settings":{
	   "took":6,
	   "searchRelevancy":"best-buy-dataset"
	},
	"search":{
	   "took":6,
	   "timed_out":false,
	   "_shards":{
		  "total":3,
		  "successful":3,
		  "skipped":0,
		  "failed":0
	   },
	   "hits":{
		  "total":{
			 "value":22,
			 "relation":"eq"
		  },
		  "max_score":5.7865844,
		  "hits":[
			 {
				"_index":"best-buy-dataset",
				"_type":"_doc",
				"_id":"UXGArnYBpOdhck8TDcFv",
				"_score":5.7865844,
				"_source":{
				   "color":"Black",
				   "name":"Cricut - Premium Removable Vinyl - Black"
				}
			 },
			 {
				"_index":"best-buy-dataset",
				"_type":"_doc",
				"_id":"4Od7rnYBR5qBrhW_U3tj",
				"_score":5.5054154,
				"_source":{
				   "color":"Black",
				   "name":"Uncaged Ergonomics - Vinyl Wobble Stool - Black"
				}
			 },
			 {
				"_index":"best-buy-dataset",
				"_type":"_doc",
				"_id":"cOd_rnYBR5qBrhW_gZB7",
				"_score":5.476978,
				"_source":{
				   "color":"Black",
				   "name":"Crosley - Vinyl Record Cleaning Set - Black"
				}
			 },
			 {
				"_index":"best-buy-dataset",
				"_type":"_doc",
				"_id":"XedzrnYBR5qBrhW_KlLb",
				"_score":5.2471347,
				"_source":{
				   "color":"Black",
				   "name":"Office Star Products - Vinyl Drafting Stool - Black"
				}
			 },
			 {
				"_index":"best-buy-dataset",
				"_type":"_doc",
				"_id":"z3F0rnYBpOdhck8T-4lM",
				"_score":5.237612,
				"_source":{
				   "color":"Black",
				   "name":"Pro-Ject - Vinyl Cleaner VC-S - Black"
				}
			 }
		  ]
	   },
	   "status":200
	}
 }`

func RunScript(scriptContext []byte, script string, timeout time.Duration) ([]byte, *[]string, error) {
	// Inject console.log support into context
	if singleton == nil {
		// covers the unit test
		singleton = &Rules{}
		// instantiate a new JavaScript VM
		singleton.iso = v8go.NewIsolate()
		global := v8go.NewObjectTemplate(singleton.iso)

		// Inject fetch support
		fetchfn := singleton.getFetchFn()
		global.Set("fetch", fetchfn, v8go.ReadOnly)

		// Inject base64 support
		baseErr := base64Polyfill.InjectTo(singleton.iso, global)
		if baseErr != nil {
			log.Warnln(logTag, "error while injecting base64 support, ", baseErr)
		}

		// Instantiate for the first time
		singleton.ReInstantiateV8Context(global)
	}
	var consoleWriter = new(bytes.Buffer)
	consoleInjectErr := console.InjectTo(singleton.context, console.WithOutput(consoleWriter))
	if consoleInjectErr != nil {
		log.Warnln(logTag, "error while injecting console polyfill, ", consoleInjectErr)
	}

	// Inject KV functions
	kvInjectErr := InjectKV(singleton.context)
	if kvInjectErr != nil {
		log.Warnln(logTag, "error while injecting key value support, ", kvInjectErr)
	} else {
		log.Debug(logTag, ": Successfully injected key value support")
	}

	var logs *[]string
	// Unique ID for script handler
	functionId := "script" + strconv.Itoa(int(time.Now().Unix()))
	_, scriptError := singleton.runScriptWithTimeout(fmt.Sprintf(`
	function %s(context) {
		%s;
		if (typeof(handleRequest) === typeof(Function)) {
			return handleRequest();
		} else if (typeof(handleResponse) === typeof(Function)) {
			return handleResponse();
		} else {
			return context;
		}
	}`, functionId, script), functionId+".js", timeout)
	if scriptError != nil {
		log.Errorln(logTag, ":", scriptError)
		return scriptContext, logs, scriptError
	}
	scriptInvokeResponse, scriptInvokeError := singleton.runScriptWithTimeout(
		fmt.Sprintf(`%s(%s)`, functionId, string(scriptContext)),
		"main.js",
		timeout,
	)

	consoleStr := consoleWriter.String()
	log.Debugln("Script Logs", consoleStr)
	consoleLogs := strings.Split(consoleStr, "\n")
	for s, v := range consoleLogs {
		if strings.Trim(v, " ") == "" {
			// If it is the last element than handle that as well
			if s == len(consoleLogs) {
				consoleLogs = consoleLogs[:s]
			} else if s < len(consoleLogs) {
				consoleLogs = append(consoleLogs[:s], consoleLogs[s+1:]...)
			}
		}
	}
	// write logs
	if len(consoleLogs) > 0 {
		logs = &consoleLogs
	}

	if scriptInvokeError != nil {
		log.Errorln(logTag, ":", scriptInvokeError)
		return scriptContext, logs, scriptInvokeError
	}
	if scriptInvokeResponse != nil {
		var resultParseError error
		var contextInBytes []byte
		// if result is promise
		if scriptInvokeResponse.IsPromise() {
			prom1, _ := scriptInvokeResponse.AsPromise()
			for prom1.State() == v8go.Pending {
				continue
			}
			contextInBytes, resultParseError = prom1.Result().MarshalJSON()

			consoleStr := consoleWriter.String()
			log.Debugln("Script Logs after async resolution", consoleStr)
			consoleLogs := strings.Split(consoleStr, "\n")
			for s, v := range consoleLogs {
				if strings.Trim(v, " ") == "" {
					// If it is the last element than handle that as well
					if s == len(consoleLogs) {
						consoleLogs = append(consoleLogs[:s])
					} else {
						consoleLogs = append(consoleLogs[:s], consoleLogs[s+1:]...)
					}
				}
			}
			// write logs
			if len(consoleLogs) > 0 {
				logs = &consoleLogs
			}
		} else {
			contextInBytes, resultParseError = scriptInvokeResponse.MarshalJSON()
		}
		if resultParseError != nil {
			log.Errorln(logTag, ":", resultParseError)
			return nil, logs, resultParseError
		}
		return contextInBytes, logs, nil
	}
	return scriptContext, logs, nil
}

// Runs the script in a new context
func (r *Rules) runScript(scriptContext ScriptContext, script string, executeRSAPI bool, executeES bool, timeout time.Duration, isDefaultResponse bool, onlyReq bool) (*ValidateScriptResponse, error) {
	// Start timer to record script execution time.
	start := time.Now()
	r.lock.Lock()
	defer r.lock.Unlock()

	// Inject console.log support into context
	var consoleWriter = new(bytes.Buffer)
	consoleInjectErr := console.InjectTo(r.context, console.WithOutput(consoleWriter))
	if consoleInjectErr != nil {
		log.Warnln(logTag, "error while injecting console polyfill, ", consoleInjectErr)
	}

	// Convert the script context into JSON so it can be passed to
	// the script function as a `context` variable.
	bytesRes, err := json.Marshal(scriptContext)
	if err != nil {
		log.Errorln(logTag, ":", err)
		return nil, err
	}
	var scriptInvokeRequest *v8go.Value
	var scriptInvokeResult *v8go.Value
	var scriptInvokeError error

	// Parse the script response
	response := ValidateScriptResponse{
		Response: new(ScriptResponse),
	}

	// Invoke script with request context
	//
	// Following code will *actually* invoke the `handleRequest`method with
	// the JSON converted context.
	scriptInvokeRequest, scriptInvokeError = r.execScript(script, "handleRequest", bytesRes, timeout)
	if scriptInvokeError != nil {
		// NOTE: log will be printed by the called function.

		// Extract the logs
		response = extractConsoleLogs(consoleWriter, response)

		return &response, scriptInvokeError
	}

	var requestInBytes []byte
	if scriptInvokeRequest != nil {
		responseParsed, resultParseError := parseScriptOutput(scriptInvokeRequest)

		// Update the response in bytes
		requestInBytes = responseParsed

		// Extract the logs
		response = extractConsoleLogs(consoleWriter, response)

		if resultParseError != nil {
			log.Errorln(logTag, ": error while parsing `handleRequest` response, ", resultParseError)
			return &response, resultParseError
		}
	}

	// If handleRequest was not defined, then update the requestInBytes value to be the
	// default request body passed.
	var modifiedRequest ScriptRequest
	if string(requestInBytes) != "{}" {
		err := json.Unmarshal(requestInBytes, &modifiedRequest)
		if err != nil {
			log.Errorln(logTag, ":", err)

			// Extract the logs
			response = extractConsoleLogs(consoleWriter, response)

			return &response, err
		}

		// Update the scriptContext with the latest request body
		scriptContext.Request = modifiedRequest
	} else {
		modifiedRequest = scriptContext.Request
	}

	// Marshal bytesRes again to get the modified request
	bytesRes, err = json.Marshal(scriptContext)
	if err != nil {
		log.Errorln(logTag, ": error while remarshalling script context after running handleRequest, ", err)
		return nil, err
	}

	// Initialize the script response
	scriptResponse := scriptContext.Response

	log.Debug(logTag, "modified request is: ", pretty.Formatter(modifiedRequest))

	// Set the default value for response as the modified request and the response
	response = ValidateScriptResponse{
		Response: &scriptResponse,
		Request:  &modifiedRequest,
	}

	// if onlyReq is passed, return
	if onlyReq {
		// Extract the logs
		response = extractConsoleLogs(consoleWriter, response)

		// Extract scriptTook
		response.ScriptTook = int(time.Since(start).Milliseconds())

		return &response, nil
	}

	// Hit RS or ES  if default response is passed else don't because we are respecting the
	// user passed response body.
	if isDefaultResponse {
		// Execute RS API to get the response
		var index []string

		// Parse the index field
		// Initially try to parse as an array of strings
		indexAsArray, ok := scriptContext.Environments["index"].([]interface{})
		if ok {
			for _, v := range indexAsArray {
				indexAsString, ok := v.(string)
				if ok {
					index = append(index, indexAsString)
				}
			}
		} else {
			// Try to parse the index field as a string
			indexAsStr, strOk := scriptContext.Environments["index"].(string)
			if strOk {
				index = append(index, indexAsStr)
			}
		}

		if executeRSAPI && len(index) > 0 {
			index := strings.Join(index, ",")
			url := "http://" + getMasterCredentials() + "@localhost:" + strconv.Itoa(util.Port) + "/" + index + "/_reactivesearch"

			// Parse the string body into RSQuery and then marshal it.
			var rsQuery querytranslate.RSQuery
			err := json.Unmarshal([]byte(modifiedRequest.Body), &rsQuery)
			if err != nil {

				// Extract the logs
				response = extractConsoleLogs(consoleWriter, response)

				log.Errorln(logTag, ": error while converting string body to RSQuery", err)
				return &response, err
			}

			// Enable cache, disable analytics, query rules
			enabledStatus := true
			disabledStatus := false
			rsQuery.Settings = &querytranslate.Settings{
				UseCache:         &enabledStatus,
				RecordAnalytics:  &disabledStatus,
				EnableQueryRules: &disabledStatus,
			}

			// Marshal the updated RSQuery back into JSON to pass it
			// in the request to RS API.
			marshalledRequest, err := json.Marshal(rsQuery)
			if err != nil {
				log.Errorln(logTag, ": error encountered while marshalling request body", err)
				return nil, err
			}

			// make the RS API request
			req, reqErr := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(marshalledRequest))
			if reqErr != nil {

				// Extract the logs
				response = extractConsoleLogs(consoleWriter, response)

				log.Errorln(logTag, ": error while creating request to hit RS API, ", reqErr)
				return &response, reqErr
			}

			// Disable cache and set content type
			req.Header.Add("Content-Type", "application/json")
			req.Header.Add("cache-control", "no-cache")
			res, err := util.HTTPClient().Do(req)
			if err != nil {
				log.Errorln(logTag, ": error encountered while marshalling request body", err)
				return nil, err
			}
			headers := make(map[string]string)
			for k := range res.Header {
				headers[k] = res.Header.Get(k)
			}

			var body string
			buffer := new(bytes.Buffer)
			_, err2 := buffer.ReadFrom(res.Body)
			body = buffer.String()
			if err2 != nil {
				log.Errorln(logTag, ": error encountered while marshalling request body", err2)
				return nil, err2
			}
			// write the script response
			if res != nil {
				scriptResponse = ScriptResponse{
					Code:    res.StatusCode,
					Headers: headers,
					Body:    body,
				}
			}
		}

		// Execute ES
		// We need to execute normal index request and bulk request separately
		// since the body type will change.
		//
		// NOTE: If executeES is true the envs.ACL is present
		if executeES && len(index) > 0 {
			// Just do an index request with the body
			var ESUrl string

			// If executeES is true than the acl will be one of the cases
			// so we don't need to worry about a default
			switch scriptContext.Environments["acl"] {
			case "index", "update", "create":
				ESUrl = fmt.Sprint(util.GetESURL(), "/", index[0], "/_doc")
			case "bulk":
				ESUrl = fmt.Sprint(util.GetESURL(), "/", index[0], "/_bulk")
			}

			req, err := http.NewRequest(http.MethodPost, ESUrl, bytes.NewBuffer([]byte(modifiedRequest.Body)))
			if err != nil {

				// Extract the logs
				response = extractConsoleLogs(consoleWriter, response)

				log.Errorln(logTag, "error while creating request for ES, ", err)
				return &response, err
			}

			log.Debug(logTag, " hitting URL: ", ESUrl)

			// Set the content type request header
			// NOTE: Following code can be simplified but kept this way
			// for better readability of the developer.
			switch scriptContext.Environments["acl"] {
			case "index", "update", "create":
				req.Header.Add("Content-Type", "application/json")
			case "bulk":
				req.Header.Add("Content-Type", "application/x-ndjson")
			}

			// Set the headers that the user passed
			if scriptContext.Request.Headers != nil {
				for key, value := range scriptContext.Request.Headers {
					req.Header.Set(key, value)
				}
			}

			res, err := util.HTTPClient().Do(req)
			if err != nil {

				// Extract the logs
				response = extractConsoleLogs(consoleWriter, response)

				log.Error(logTag, "error occurred while making the ES request, ", err)
				return &response, err
			}

			// Parse the response headers
			headers := make(map[string]string)
			for k := range res.Header {
				headers[k] = res.Header.Get(k)
			}

			var body string
			buffer := new(bytes.Buffer)
			_, err2 := buffer.ReadFrom(res.Body)
			body = buffer.String()
			if err2 != nil {

				// Extract the logs
				response = extractConsoleLogs(consoleWriter, response)

				log.Errorln(logTag, ": error encountered while marshalling response body", err2)
				return &response, err2
			}
			// write the script response
			if res != nil {
				scriptResponse = ScriptResponse{
					Code:    res.StatusCode,
					Headers: headers,
					Body:    body,
				}
			}
		}

		response = ValidateScriptResponse{
			Request:  &modifiedRequest,
			Response: &scriptResponse,
		}

		// Update script context response as well
		// since handleResponse will be called after this
		scriptContext.Response = scriptResponse

		// Marshal the bytesRes again since the context was updated
		bytesRes, err = json.Marshal(scriptContext)
		if err != nil {

			// Extract the logs
			response = extractConsoleLogs(consoleWriter, response)

			log.Errorln(logTag, "error while marshaling script context with the udpated response, ", err)
			return &response, err
		}
	}

	// Invoke script with response context
	//
	// Following will *actually* invoke the `handleResponse` function with
	// the passed script context.
	scriptInvokeResult, scriptInvokeError = r.execScript(script, "handleResponse", bytesRes, timeout)

	if scriptInvokeError != nil {
		// NOTE: Log will be handled by the called function

		// Extract the logs
		response = extractConsoleLogs(consoleWriter, response)

		return &response, scriptInvokeError
	}

	// Extract the response from `handleResponse`
	var responseInBytes []byte
	if scriptInvokeResult != nil {
		responseParsed, resultParseError := parseScriptOutput(scriptInvokeResult)

		// Update the response in bytes
		responseInBytes = responseParsed

		// Extract the logs
		response = extractConsoleLogs(consoleWriter, response)

		if resultParseError != nil {
			log.Errorln(logTag, ": error while parsing `handleResponse` response, ", resultParseError)
			return &response, resultParseError
		}
	}

	// Modify Response if the output is different than default output, i.e. when handleRequest() function is defined by the user
	if string(responseInBytes) != "{}" {
		var modifiedResponse ScriptResponse
		err5 := json.Unmarshal(responseInBytes, &modifiedResponse)

		// Extract the logs
		response = extractConsoleLogs(consoleWriter, response)

		if err5 != nil {
			log.Errorln(logTag, ":", err5)
			return &response, err5
		}
		if response.Request == nil {
			response.Request = &scriptContext.Request
		}
		response.Response = &modifiedResponse
		response.ScriptTook = int(time.Since(start).Milliseconds())
	}

	// Extract the console logs
	response = extractConsoleLogs(consoleWriter, response)

	// Extract scriptTook
	response.ScriptTook = int(time.Since(start).Milliseconds())

	return &response, nil
}

// execScript will format the script into proper code and execute it accordingly
// and return the response and error.
func (r *Rules) execScript(script string, functionName string, contextBytes []byte, timeout time.Duration) (*v8go.Value, error) {
	// Make sure functionName is valid
	if functionName != "handleRequest" && functionName != "handleResponse" {
		return nil, errors.New("functionName should be `handleResponse` or `handleRequest`")
	}

	// Unique ID for script handler
	functionId := "script" + strconv.Itoa(int(time.Now().Unix()))
	// We're using javascript function scope to execute a script
	// It helps to limit the script changes to that function only
	// 1. Run the user script
	// 2. Parse the stringified context to JSON object
	// 3. Invoke `handleRequest` or `handleResponse` method based on the execution type

	// Run the request script if defined first
	// Following will not actually invoke the method
	// but just initialize the script with the code passed by the user.
	//
	// The following code is just defining a function that contains the
	// function code.
	_, scriptError := r.runScriptWithTimeout(fmt.Sprintf(`
	function %s(context) {
		%s;
		if (typeof(%s) === typeof(Function)) {
			return %s();
		} else {
			return {};
		}
	}`, functionId, script, functionName, functionName), functionId+".js", timeout)
	if scriptError != nil {
		log.Errorln(logTag, ":", scriptError)
		return nil, scriptError
	}

	// Invoke script with request context
	//
	// Following code will *actually* invoke the method with
	// the JSON converted context.
	scriptInvokeResult, scriptInvokeError := r.runScriptWithTimeout(fmt.Sprintf(`%s(%s)`, functionId, string(contextBytes)), "main.js", timeout)
	if scriptInvokeError != nil {
		log.Errorln(logTag, fmt.Sprintf(": error while executing `%s`, ", functionName), scriptInvokeError)
		return nil, scriptInvokeError
	}

	return scriptInvokeResult, nil
}

// parseScriptOutput will parse the scripts output into bytes and
// handle errors while doing so, if any.
func parseScriptOutput(scriptInvokeResult *v8go.Value) ([]byte, error) {
	var responseInBytes []byte
	var resultParseError error

	// If result is promise, wait till it's resolved
	// or rejected.
	if scriptInvokeResult.IsPromise() {
		prom1, err := scriptInvokeResult.AsPromise()
		if err != nil {
			return nil, err
		}

		// Wait till the promise state is pending, i:e
		// not resolved or rejected.
		for prom1.State() == v8go.Pending {
			continue
		}

		// If the promise is rejected, show an error accordingly
		if prom1.State() == v8go.Rejected {
			errMsg := prom1.Result().String()
			return nil, errors.New(errMsg)
		}

		responseInBytes, resultParseError = prom1.Result().MarshalJSON()
	} else {
		responseInBytes, resultParseError = scriptInvokeResult.MarshalJSON()
	}

	return responseInBytes, resultParseError
}

func (r *Rules) runScriptWithTimeout(script string, origin string, timeout time.Duration) (*v8go.Value, error) {
	vals := make(chan *v8go.Value, 1)
	errs := make(chan error, 1)

	go func() {
		val, err := r.context.RunScript(script, origin)
		if err != nil {
			// Convert the error into JSError
			jsErr := err.(*v8go.JSError)
			errMsg := fmt.Sprintf("%s at char location: +`%d`+", jsErr.Message, parseCharPositionForError(jsErr.Location))
			err = errors.New(errMsg)
			errs <- err
			return
		}
		vals <- val
	}()

	select {
	case val := <-vals:
		// success
		return val, nil
	case err := <-errs:
		// javascript error
		return nil, err
	case <-time.After(timeout):
		vm := r.context.Isolate() // get the Isolate from the context
		vm.TerminateExecution()   // terminate the execution
		err := <-errs             // will get a termination error back from the running script
		return nil, err
	}
}

func getTriggerEnvs(ctx context.Context, r *http.Request, environments querytranslate.QueryEnvs, triggerType TriggerType) (*TriggerEnvironmentsToEvaluate, error) {
	indices, err := index.FromContext(ctx)
	if err != nil {
		return nil, err
	}
	// add the `*` for the default comparison
	indices = append(indices, "*")

	// Use indices in lower case for comparison since we parse the values to lower case in trigger expression before saving
	var lowerCaseIndices []string
	for _, v := range indices {
		lowerCaseIndices = append(lowerCaseIndices, strings.ToLower(v))
	}

	ip := iplookup.FromRequest(r)

	var clientIPv4 string

	var clientIPv6 string

	// Declare default empty strings for passed category
	// and ACL
	var passedCategory = ""
	var passedACL = ""

	// Extract the category from the request contenxt
	reqCategory, err := category.FromContext(ctx)
	if err != nil {
		log.Errorln(logTag, ":", "Couldn't extract category from ctx")
	} else {
		passedCategory = reqCategory.String()
	}

	reqAcl, err := acl.FromContext(ctx)
	if err != nil {
		log.Debugln(logTag, ":", "Couldn't extract ACL from ctx, using category instead", err)
		passedACL = reqCategory.String()
	} else {
		passedACL = reqAcl.String()
	}

	ipv4 := GetClientIP4(ip)
	if ipv4 != "" {
		clientIPv4 = ipv4
	} else {
		ipv6 := GetClientIP6(ip)
		if ipv6 != "" {
			clientIPv6 = ipv6
		}
	}
	customEvents := make(map[string]string)

	// Check if the request is filter type then extract RSQuery from context
	if triggerType == Filter {
		req, err := querytranslate.FromContext(ctx)
		if err != nil {
			log.Error(logTag, ": Error while extracting RSQuery from context")
			return nil, err
		}
		if req.Settings != nil && req.Settings.CustomEvents != nil {
			for key, value := range *req.Settings.CustomEvents {
				valueAsString, ok := value.(string)
				if ok {
					customEvents[key] = valueAsString
				}
			}
		}
	}

	parsedEnvironments := TriggerEnvironmentsToEvaluate{
		Filter:       ParseFilters(environments.TermFilters),
		Index:        lowerCaseIndices,
		Origin:       r.Host,         // https://my-search.domain.com
		Referer:      r.Referer(),    // https://my-search.domain.com/path?q=hello
		Path:         r.URL.Path,     // /_doc/test
		Category:     passedCategory, // docs
		ACL:          passedACL,      // bulk
		IPv4:         clientIPv4,     // 29.120.12.12
		IPv6:         clientIPv6,     // 2001:db8:3333:4444:5555:6666:7777:8888
		CustomEvents: customEvents,
	}
	if environments.Query != nil {
		parsedEnvironments.Query = *environments.Query
	}
	return &parsedEnvironments, nil
}

// GetTriggerEnvs extracts the trigger environments to run
// the trigger.
func GetTriggerEnvs(ctx context.Context, r *http.Request, environments querytranslate.QueryEnvs, triggerType TriggerType) (*TriggerEnvironmentsToEvaluate, error) {
	return getTriggerEnvs(ctx, r, environments, triggerType)
}

// Check if the passed ACL is a valid value from the list
// of the possible ACL's for an indexing request.
//
// We will just check if the value passed is present
// in a predefined list.
func isValidACLForIndexing(reqACL *acl.ACL) bool {
	reqACLValue := *reqACL
	switch reqACLValue {
	case
		acl.Index,
		acl.Update,
		acl.UpdateByQuery,
		acl.Create,
		acl.Bulk,
		acl.Delete,
		acl.DeleteByQuery:
		return true
	}
	return false
}

// Check if the request is of indexing type.
// This is done by checking the category to be of type "docs"
// and ACL's should be one of:
// ['index', 'update', 'update_by_query', 'create', 'bulk', 'delete' 'delete_by_query']
func isIndexingRequest(req *http.Request) bool {
	ctx := req.Context()

	reqCategory, err := category.FromContext(ctx)
	if err != nil {
		log.Errorln(logTag, ":", err)
		return false
	}

	// If the category is not docs, just return
	if *reqCategory != category.Docs {
		return false
	}

	// Check if the ACL matches.
	reqAcl, err := acl.FromContext(ctx)
	if err != nil {
		log.Errorln(logTag, ":", err)
		return false
	}

	// Check if the ACL is valid for indexing
	if !isValidACLForIndexing(reqAcl) {
		return false
	}

	return true
}

// IsIndexingRequest checks if the request is an indexing one
// and accordingly returns it.
func IsIndexingRequest(req *http.Request) bool {
	return isIndexingRequest(req)
}

// Get string from the passed map[string]interface value
func getStringFromInterface(jsonBody interface{}) (string, error) {
	mapJSONBody, ok := jsonBody.(map[string]interface{})
	if !ok {
		// body should be treated as a string and passed
		return fmt.Sprintf("%v", jsonBody), nil
	}
	jsonStr, err := json.Marshal(mapJSONBody)
	if err != nil {
		return "", err
	}
	return string(jsonStr), nil
}

// Extract the request body to be used in the code from the wrapper
// as passed by the user.
// We just need to marshal the JSON passed in the body values and copy the
// rest as usual.
func extractBodyFromWrapper(requestBodyIn ValidateScriptRequestWrapper) (ValidateScriptRequest, error) {
	var requestBody ValidateScriptRequest
	// Convert the interface request body and response body to string and
	// assign to ValidateScriptRequest
	if requestBodyIn.Request != nil {
		requestBodyParsedStr, err := getStringFromInterface(requestBodyIn.Request.Body)
		if err != nil {
			log.Error(logTag, ": error while marshalling request body to string, ", err)
			return requestBody, err
		}
		requestBody.Request = &ScriptRequest{
			Body:    requestBodyParsedStr,
			Headers: requestBodyIn.Request.Headers,
		}
	}

	if requestBodyIn.Response != nil {
		responseBodyParsedStr, err := getStringFromInterface(requestBodyIn.Response.Body)
		if err != nil {
			log.Errorln(logTag, ": error while marshalling response body to string, ", err)
			return requestBody, err
		}
		requestBody.Response = &ScriptResponse{
			Code:    requestBodyIn.Response.Code,
			Body:    responseBodyParsedStr,
			Headers: requestBodyIn.Response.Headers,
		}
	}

	requestBody.Script = requestBodyIn.Script
	requestBody.Envs = requestBodyIn.Envs
	requestBody.IsCron = requestBodyIn.IsCron

	return requestBody, nil
}

// Convert the passed response to the wrapper so that it can be
// returned properly.
func getWrapperFromBody(responseBody ValidateScriptResponse, isBulk bool) (ValidateScriptResponseWrapper, error) {
	var responseBodyOut ValidateScriptResponseWrapper

	if responseBody.Request != nil {
		// Unmarshal the string into an interface
		responseBodyOut.Request = &ScriptRequestIn{
			Headers: responseBody.Request.Headers,
		}

		// If it is a bulk request, don't unmarshal since it'll throw
		// an error
		if isBulk {
			responseBodyOut.Request.Body = responseBody.Request.Body
		} else {
			var responseRequestBody interface{}
			err := json.Unmarshal([]byte(responseBody.Request.Body), &responseRequestBody)
			if err != nil {
				log.Error(logTag, ": error while unmarshalling response request body, ", responseBody.Request.Body, err)
				return responseBodyOut, err
			}
			responseBodyOut.Request.Body = responseRequestBody
		}
	}

	if responseBody.Response != nil {
		// Unmarshal the string into interface
		var responseResponseBody interface{}
		err := json.Unmarshal([]byte(responseBody.Response.Body), &responseResponseBody)
		if err != nil {
			log.Error(logTag, ": error while unmarshalling response's response body, ", responseBody.Response.Body, err)
			return responseBodyOut, err
		}
		responseBodyOut.Response = &ScriptResponseIn{
			Body:    responseResponseBody,
			Headers: responseBody.Response.Headers,
			Code:    responseBody.Response.Code,
		}
	}

	responseBodyOut.ScriptTook = responseBody.ScriptTook
	responseBodyOut.Console = responseBody.Console
	return responseBodyOut, nil
}

// Create an error value to be reported when internal method
// fails.
func newErrorValue(iso *v8go.Isolate, err string) *v8go.Value {
	errVal, _ := v8go.NewValue(iso, err)
	return errVal
}

// Get the fetch function to be used by JS
func (r *Rules) getFetchFn() *v8go.FunctionTemplate {
	return v8go.NewFunctionTemplate(r.iso, func(info *v8go.FunctionCallbackInfo) *v8go.Value {
		args := info.Args()
		resolver, _ := v8go.NewPromiseResolver(info.Context())

		go func() {
			if len(args) <= 0 {
				err := errors.New("1 argument required, but only 0 present")
				resolver.Reject(newErrorValue(r.iso, err.Error()))
				return
			}

			url := args[0].String()
			var reqBody FetchRequest
			if len(args) > 1 {
				str, err := v8go.JSONStringify(r.context, args[1])
				if err != nil {
					resolver.Reject(newErrorValue(r.iso, err.Error()))
					return
				}

				reader := strings.NewReader(str)
				if err := json.NewDecoder(reader).Decode(&reqBody); err != nil {
					resolver.Reject(newErrorValue(r.iso, err.Error()))
					return
				}
			}

			var res *http.Response
			var err error

			req, err := http.NewRequest(reqBody.Method, url, strings.NewReader(reqBody.Body))
			if err != nil {
				log.Warnln(logTag, " error occurred while creating request body, ", err)
				resolver.Reject(newErrorValue(r.iso, err.Error()))
				return
			}

			// Create the header
			req.Header = make(http.Header)

			// Set headers as well
			for key, value := range reqBody.Headers {
				req.Header.Set(key, value)
			}

			res, err = util.HTTPClient().Do(req)

			if err != nil {
				log.Errorln(logTag, ":", err)

				// Sometimes the error can contain " in the string which might
				// lead to weird side effects by JSON unmarshal. Replace " with
				// backticks (`)
				replacerReg := regexp.MustCompile(`"`)
				errVal, _ := v8go.NewValue(r.iso, replacerReg.ReplaceAllString(err.Error(), "`"))
				resolver.Reject(errVal)
			} else {
				body, err := ioutil.ReadAll(res.Body)
				if err != nil {
					log.Errorln(logTag, ":", err)
					errVal, _ := v8go.NewValue(r.iso, err.Error())
					resolver.Reject(errVal)
				}

				// Extract other details about the request as well

				// Extract the response headers
				headers := make(map[string]string)
				for key, value := range res.Header {
					headers[key] = strings.Join(value, ", ")
				}

				fetchResponse := map[string]interface{}{
					"body":       string(body),
					"status":     res.StatusCode,
					"statusText": res.Status,
					"ok":         res.StatusCode == http.StatusOK,
					"headers":    headers,
					"redirected": res.StatusCode == http.StatusPermanentRedirect || res.StatusCode == http.StatusTemporaryRedirect,
					"url":        url,
				}

				responseTemplate := v8go.NewObjectTemplate(r.iso)

				// Inject a `JSON` and `text` function
				//
				// Following piece of code is picked up from https://github.com/kuoruan/v8go-polyfills
				// All credit goes to the authors of the repo and the following code contains
				// some modifications (wherever was necessary).
				jsonFnTmp := v8go.NewFunctionTemplate(r.iso, func(info *v8go.FunctionCallbackInfo) *v8go.Value {
					ctx := info.Context()

					resolver, _ := v8go.NewPromiseResolver(ctx)

					go func() {
						val, err := v8go.JSONParse(ctx, string(body))
						if err != nil {
							rejectVal, _ := v8go.NewValue(r.iso, err.Error())
							resolver.Reject(rejectVal)
							return
						}

						resolver.Resolve(val)
					}()

					return resolver.GetPromise().Value
				})

				jsonSetErr := responseTemplate.Set("json", jsonFnTmp, v8go.ReadOnly)
				if jsonSetErr != nil {
					errVal, _ := v8go.NewValue(r.iso, fmt.Errorf("error while injecting `json` as a function for the fetch response: %s", jsonSetErr.Error()))
					resolver.Reject(errVal)
				}

				textFnTmp := v8go.NewFunctionTemplate(r.iso, func(info *v8go.FunctionCallbackInfo) *v8go.Value {
					ctx := info.Context()
					resolver, _ := v8go.NewPromiseResolver(ctx)

					go func() {
						v, err := v8go.NewValue(r.iso, body)
						if err != nil {
							rejectVal, _ := v8go.NewValue(r.iso, err.Error())
							resolver.Reject(rejectVal)
							return
						}
						resolver.Resolve(v)
					}()

					return resolver.GetPromise().Value
				})

				textSetErr := responseTemplate.Set("text", textFnTmp, v8go.ReadOnly)
				if textSetErr != nil {
					errVal, _ := v8go.NewValue(r.iso, fmt.Errorf("error while injecting `text` as a function for the fetch response: %s", textSetErr.Error()))
					resolver.Reject(errVal)
				}

				resObj, err := responseTemplate.NewInstance(info.Context())
				if err != nil {
					errVal, _ := v8go.NewValue(r.iso, err.Error())
					resolver.Reject(errVal)
				}

				for key, value := range fetchResponse {
					resObj.Set(key, value)
				}

				resolver.Resolve(resObj)
			}
		}()
		return resolver.GetPromise().Value
	})
}

// Request Body for fetch
type FetchRequest struct {
	Body     string            `json:"body"`
	Headers  map[string]string `json:"headers"`
	Method   string            `json:"method"`
	Redirect string            `json:"redirect"`
}

func TriggerEnvsToMap(envs TriggerEnvironmentsToEvaluate) map[string]interface{} {
	var scriptEnvs map[string]interface{}
	envsBytes, err := json.Marshal(envs)
	if err != nil {
		log.Warnln(logTag, "error occurred while converting trigger envs to map", err)
	}
	err = json.Unmarshal(envsBytes, &scriptEnvs)
	if err != nil {
		log.Warnln(logTag, "error occurred while unmarshalling the combined envs", err)
	}
	return scriptEnvs
}

// parseCharPositionForError will parse the character position
// for the error character in the passed JS string
func parseCharPositionForError(errLocation string) int {
	// The location string will contain the last part of the string
	// in the form of <lineno>:<columnno>$
	// Since, in our case the JS script is one line, we can find the position
	// of the charcter using the colunmno field.
	columnNoMatch := regexp.MustCompile(`:\d+$`)
	cleanupRe := regexp.MustCompile(`:`)

	locationMatches := columnNoMatch.FindAllString(errLocation, 1)
	if len(locationMatches) == 0 {
		return -1
	}

	// Cleanup the regex
	cleanedUpLocation := cleanupRe.ReplaceAllString(locationMatches[0], "")

	// Convert to int64 and subtract 4
	// 4 because we have two \t in the string
	locationInt, err := strconv.Atoi(cleanedUpLocation)
	if err != nil {
		log.Warnln(logTag, "something went wrong while parsing column no to int, ", err)
		return -1
	}

	// Subtract 4 (for \t)
	return locationInt - 4
}

// extractConsoleLogs will try to extract the console logs
// from the writer and update the response pointer with the logs
func extractConsoleLogs(consoleWriter *bytes.Buffer, response ValidateScriptResponse) ValidateScriptResponse {
	// Extract the console logs
	// Even if script didn't run properly, the console string
	// should return an empty string so no need to handle
	// errors here.
	consoleStr := consoleWriter.String()
	consoleLogs := strings.Split(consoleStr, "\n")
	response.Console = consoleLogs

	return response
}
