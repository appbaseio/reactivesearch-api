package util

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
)

const (
	MBToBytes = 1000000
)

var planToLimit = make(map[Plan]PlanLimit)

// LimitValue will contain the limit value
type LimitValue struct {
	Value   int    `json:"value"`
	Unit    string `json:"unit"`
	NoLimit bool   `json:"no_limit"`
}

// PlanLimit will indicate the limit for every plan
type PlanLimit struct {
	DataUsage           LimitValue `json:"data_usage"`
	AnalyticsAndLogsTTL LimitValue `json:"analytics_and_logs_ttl"`
	Indexes             LimitValue `json:"indexes"`
	Pipelines           LimitValue `json:"pipelines"`
	QueryRules          LimitValue `json:"query_rules"`
	Storage             LimitValue `json:"storage"`
	Requests            LimitValue `json:"requests"`
}

// IsLimitExceeded will check if the passed limit exceeds the
// allowed limit for the plan
func (l LimitValue) IsLimitExceeded(value int) bool {
	// If plan doesn't have a limit, always return false
	if l.NoLimit || l.Value == -1 {
		return false
	}

	return value >= l.Value
}

// FetchLimitsPerPlan will fetch the limits on a per-plan basis
// from AccAPI
func FetchLimitsPerPlan() error {
	urlToHit := ACCAPI + "sls/plan_limits"

	req, err := http.NewRequest(http.MethodGet, urlToHit, nil)
	if err != nil {
		// Handle the error
		return err
	}

	req.Header.Add("X-REACTIVESEARCH-TOKEN", os.Getenv("REACTIVESEARCH_AUTH_TOKEN"))

	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("cache-control", "no-cache")
	res, err := HTTPClient().Do(req)
	if err != nil {
		// Handle error
		return err
	}

	// Read the body
	resBody, readErr := ioutil.ReadAll(res.Body)
	defer res.Body.Close()

	if readErr != nil {
		// Handle error
		return readErr
	}

	limitResponse := make(map[string]interface{})
	unmarshalErr := json.Unmarshal(resBody, &limitResponse)

	if unmarshalErr != nil {
		// Handle error
		return unmarshalErr
	}

	limitsAsMap, asMapOk := limitResponse["limits"].(map[string]interface{})
	if !asMapOk {
		return fmt.Errorf("error while extracting `limits` from AccAPI response!")
	}

	planToLimitTemp := make(map[Plan]PlanLimit)

	for planName, limitAsInterface := range limitsAsMap {
		// Extract the limitAsInterface into a custom limit type
		marshalledLimits, marshalErr := json.Marshal(limitAsInterface)
		if marshalErr != nil {
			continue
		}

		var planLimit PlanLimit
		unmarshalErr := json.Unmarshal(marshalledLimits, &planLimit)
		if unmarshalErr != nil {
			continue
		}

		plan := PlanFromString(planName)
		if plan == InvalidValueEncountered {
			continue
		}

		// For `data_usage`, we need to set the unit as bytes
		// and convert the values
		if planLimit.DataUsage.Unit == "megabytes" {
			planLimit.DataUsage.Value = planLimit.DataUsage.Value * MBToBytes
			planLimit.DataUsage.Unit = "bytes"
		}

		if planLimit.Storage.Unit == "megabytes" {
			planLimit.Storage.Value = planLimit.Storage.Value * MBToBytes
			planLimit.Storage.Unit = "bytes"
		}

		planToLimitTemp[plan] = planLimit
	}

	planToLimit = planToLimitTemp
	return nil
}

// LimitForPlan will return the limit object for the passed
// plan.
//
// If limit doesn't exist for plan, we need to return dummy values
// though that's a very unlikely situation
func (p Plan) LimitForPlan() PlanLimit {
	planLimit, exists := planToLimit[p]
	if !exists {
		defaultValue := LimitValue{
			Value:   0,
			Unit:    "na",
			NoLimit: false,
		}
		return PlanLimit{
			DataUsage:           defaultValue,
			AnalyticsAndLogsTTL: defaultValue,
			Indexes:             defaultValue,
			Pipelines:           defaultValue,
			QueryRules:          defaultValue,
			Storage:             defaultValue,
			Requests:            defaultValue,
		}
	}

	return planLimit
}
