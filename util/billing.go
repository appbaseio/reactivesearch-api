package util

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/appbaseio/reactivesearch-api/model/domain"
	log "github.com/sirupsen/logrus"
)

const ArcIDEnvName = "ARC_ID"
const ClusterIDEnvName = "CLUSTER_ID"
const AppbaseIDEnvName = "APPBASE_ID"

// ACCAPI URL
var ACCAPI = "https://accapi.appbase.io/"

// var ACCAPI = "http://localhost:3000/"

var planDetailsHook *func([]byte)

func SetPlanDetailsHook(fn *func([]byte)) {
	planDetailsHook = fn
}

// planDetails represents the plan endpoint response
var planDetails *[]byte

// setPlanDetails sets the plan details
func setPlanDetails(planInfo []byte) {
	planDetails = &planInfo
	if planDetailsHook != nil {
		hook := *planDetailsHook
		hook(planInfo)
	}
}

// GetPlanDetails returns the plan details
func GetPlanDetails() *[]byte {
	return planDetails
}

// Tier is the value of the user's plan
var tier *Plan

// SetTier sets the tier value
func SetTier(plan *Plan) {
	tier = plan
}

// GetTier returns the current tier
func GetTier(ctx context.Context) *Plan {
	if MultiTenant {
		if ctx == nil {
			return nil
		}
		// Fetch the domain from context
		domainUsed, domainFetchErr := domain.FromContext(ctx)
		if domainFetchErr != nil {
			return nil
		}
		planInfo := GetSLSInstanceByDomain(domainUsed.Raw)
		if planInfo != nil {
			return planInfo.Tier
		}
		return nil
	}
	return tier
}

// Number of ReactiveSearch machines
var numberOfMachines int64

func setNumberOfMachines(machines int64) {
	numberOfMachines = machines
}

// To retrieve the total number of machines
func GetNumberOfMachines() int64 {
	return numberOfMachines
}

// TimeValidity to be obtained from ACCAPI (in secs)
var timeValidity int64

// GetTimeValidity returns the time validity
func GetTimeValidity() int64 {
	return timeValidity
}

// SetTimeValidity returns the time validity
func SetTimeValidity(time int64) {
	timeValidity = time
}

// maxErrorTime before showing errors if invalid trial / plan in hours
var maxErrorTime int64 = 168 // in hrs

// NodeCount is the current node count, defaults to 1
var NodeCount = 1

// isInvalidArcIDUsed flag is being used to determine that if the ReactiveSearch/Appbase.io id is invalid.
// If it is `true` then `Arc` will start throwing errors immediately with a status code of `400` instead of `402`
var isInvalidArcIDUsed = false

// ArcUsage struct is used to report time usage
type ArcUsage struct {
	ArcID          string `json:"arc_id"`
	SubscriptionID string `json:"subscription_id"`
	Quantity       int    `json:"quantity"`
	ClusterID      string `json:"cluster_id"`
	MachineID      string `json:"machine_id"`
}

type ClusterPlan struct {
	Tier                    *Plan    `json:"tier"`
	FeatureCustomEvents     bool     `json:"feature_custom_events"`
	FeatureSuggestions      bool     `json:"feature_suggestions"`
	FeatureRules            bool     `json:"feature_rules"`
	FeatureSearchRelevancy  bool     `json:"feature_search_relevancy"`
	FeatureSearchGrader     bool     `json:"feature_search_grader"`
	FeatureEcommerce        bool     `json:"feature_ecommerce"`
	FeatureCache            bool     `json:"feature_cache"`
	FeaturePipelines        bool     `json:"feature_pipelines"`
	FeatureUIBuilderPremium bool     `json:"feature_uibuilder_premium"`
	Trial                   bool     `json:"trial"`
	TrialValidity           int64    `json:"trial_validity"`
	TierValidity            int64    `json:"tier_validity"`
	TimeValidity            int64    `json:"time_validity"`
	SubscriptionID          string   `json:"subscription_id"`
	ClusterID               string   `json:"cluster_id"`
	NumberOfMachines        int64    `json:"number_of_machines"`
	SubscriptionCanceled    bool     `json:"subscription_canceled"`
	CreatedAt               int64    `json:"created_at"`
	Backend                 *Backend `json:"backend"`
}

// ArcUsageResponse stores the response from ACCAPI
type ArcUsageResponse struct {
	Accepted      bool   `json:"accepted"`
	FailureReason string `json:"failure_reason"`
	ErrorMsg      string `json:"error_msg"`
	WarningMsg    string `json:"warning_msg"`
	StatusCode    int    `json:"status_code"`
	TimeValidity  int64  `json:"time_validity"`
}

// ArcInstance TBD: remove struct
type ArcInstance struct {
	SubscriptionID string `json:"subscription_id"`
}

// ArcInstanceResponse TBD: Remove struct
type ArcInstanceResponse struct {
	ArcInstances []ArcInstanceDetails `json:"instances"`
}

// Cluster plan response type
type ClusterPlanResponse struct {
	Plan ClusterPlan `json:"plan"`
}

// ArcInstanceDetails contains the info about a ReactiveSearch Instance
type ArcInstanceDetails struct {
	NodeCount               int                    `json:"node_count"`
	Description             string                 `json:"description"`
	SubscriptionID          string                 `json:"subscription_id"`
	SubscriptionCanceled    bool                   `json:"subscription_canceled"`
	Trial                   bool                   `json:"trial"`
	TrialValidity           int64                  `json:"trial_validity"`
	CreatedAt               int64                  `json:"created_at"`
	Tier                    *Plan                  `json:"tier"`
	TierValidity            int64                  `json:"tier_validity"`
	TimeValidity            int64                  `json:"time_validity"`
	Metadata                map[string]interface{} `json:"metadata"`
	FeatureCustomEvents     bool                   `json:"feature_custom_events"`
	FeatureSuggestions      bool                   `json:"feature_suggestions"`
	FeatureRules            bool                   `json:"feature_rules"`
	FeatureSearchRelevancy  bool                   `json:"feature_search_relevancy"`
	FeatureSearchGrader     bool                   `json:"feature_search_grader"`
	FeatureEcommerce        bool                   `json:"feature_ecommerce"`
	FeatureUIBuilderPremium bool                   `json:"feature_uibuilder_premium"`
	FeatureCache            bool                   `json:"feature_cache"`
	FeaturePipelines        bool                   `json:"feature_pipelines"`
	ClusterID               string                 `json:"cluster_id"`
	NumberOfMachines        int64                  `json:"number_of_machines"`
	Backend                 *Backend               `json:"backend"`
}

// SetDefaultTier sets the default tier when billing is disabled
func SetDefaultTier() {
	var plan = ArcEnterprise
	SetTier(&plan)
}

// ValidateArcID validates the APPBASE_ID by checking the response returned from the ACCAPI
func ValidateArcID(statusCode int) {
	if statusCode == http.StatusBadRequest {
		// Set the flag to `true` so `Arc` can start throwing errors immediately
		isInvalidArcIDUsed = true
	} else {
		isInvalidArcIDUsed = false
	}
}

func validateTimeValidity(timeValidity int64) bool {
	if timeValidity > 0 { // Valid plan
		return true
	} else if timeValidity <= 0 && -timeValidity < 3600*maxErrorTime { // Negative validity, plan has been expired
		// Print warning message if remaining time is less than max allowed time
		log.Println("Warning: Payment is required. Arc will start sending out error messages in next", maxErrorTime, "hours")
		return true
	}
	return false
}

// BillingMiddleware function to be called for each request
func BillingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if MultiTenant {
			// Check if routes are blacklisted
			requestURI := r.RequestURI
			for _, route := range BillingBlacklistedPaths() {
				if strings.HasPrefix(requestURI, route) {
					if ShouldRecordUsage(route) {
						RecordUsageMiddleware(next).ServeHTTP(w, r)
					} else {
						next.ServeHTTP(w, r)
					}
					return
				}
			}

			domainInfo, err := domain.FromContext(r.Context())
			if err != nil {
				log.Errorln("error while reading domain from context")
				WriteBackError(w, "Please make sure that you're using a valid domain. If the issue persists please contact support@appbase.io with your domain or registered e-mail address.", http.StatusBadRequest)
				return
			}

			// Fetch tenantID from the domain read
			tenantID := GetTenantForDomain(domainInfo.Raw)

			// Check the rate limit and throw errors accordingly
			if GetRequestCounterForTenant(tenantID).IsExceeded() {
				log.Errorln("request limit exceeded for the current minute!")
				w.Header().Set("Retry-After", "60")
				WriteBackError(w, "Too many requests, please try after a while!", http.StatusTooManyRequests)
				return
			}

			// Check if data usage has exceeded the allowed limit
			if IsDataUsageExceeded(domainInfo.Raw) {
				log.Errorln("data-usage limit exceeded for the current day")
				w.Header().Set("Retry-After", fmt.Sprintf("%d", 3600*24))
				WriteBackError(w, "Data usage limit exceeded for the plan", http.StatusTooManyRequests)
				return
			}

			// Get instance details for domain
			slsInstanceInfo := GetSLSInstanceByDomain(domainInfo.Raw)
			if slsInstanceInfo == nil {
				// Check if payment is required for this domain
				if IsPaymentNeeded(domainInfo.Raw) {
					WriteBackError(w, "Payment required to use the domain!", http.StatusPaymentRequired)
					return
				}

				WriteBackError(w, "Please make sure that you're using a valid domain. If the issue persists please contact support@appbase.io with your domain or registered e-mail address.", http.StatusBadRequest)
				return
			}
			log.Infoln("current time validity value: ", slsInstanceInfo.TimeValidity)
			// Routes are not blacklisted, verify the payment
			if validateTimeValidity(slsInstanceInfo.TimeValidity) {
				RecordUsageMiddleware(next).ServeHTTP(w, r)
			} else {
				// Write an error and stop the handler chain
				WriteBackError(w, "Payment required", http.StatusPaymentRequired)
				return
			}
		} else {
			log.Infoln("current time validity value: ", GetTimeValidity())

			if isInvalidArcIDUsed {
				// throw invalid APPBASE_ID usage error
				WriteBackError(w, "Please make sure that you're using a valid APPBASE_ID. If the issue persists please contact support@appbase.io with your APPBASE_ID or registered e-mail address.", http.StatusBadRequest)
				return
			}

			// Check if routes are blacklisted
			requestURI := r.RequestURI
			for _, route := range BillingBlacklistedPaths() {
				if strings.HasPrefix(requestURI, route) {
					RecordUsageMiddleware(next).ServeHTTP(w, r)
					return
				}
			}

			// Routes are not blacklisted, verify the payment
			if validateTimeValidity(GetTimeValidity()) {
				RecordUsageMiddleware(next).ServeHTTP(w, r)
			} else {
				// Write an error and stop the handler chain
				WriteBackError(w, "Payment required", http.StatusPaymentRequired)
				return
			}
		}
	})
}

func getMasterCredentials() string {
	username, password := os.Getenv("USERNAME"), os.Getenv("PASSWORD")
	if username == "" {
		username, password = "foo", "bar"
	}
	return username + ":" + password
}

func GetCachedPlanDetails() ([]byte, error) {
	url := "http://" + getMasterCredentials() + "@localhost:" + strconv.Itoa(Port) + "/arc/plan/fs"
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("cache-control", "no-cache")
	res, err := HTTPClient().Do(req)
	if err != nil {
		log.Errorln("error while requesting /arc/plan/fs")
		return nil, err
	}
	defer res.Body.Close()
	return ioutil.ReadAll(res.Body)
}

func getArcInstance(arcID string) (ArcInstance, error) {
	arcInstance := ArcInstance{}
	url := ACCAPI + "arc/instances?arcid=" + arcID
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("cache-control", "no-cache")

	res, err := HTTPClient().Do(req)
	// If ACCAPI is down then set the plan
	if (res != nil && res.StatusCode >= 500) || err != nil {
		if planDetails != nil {
			return setBillingVarsArcInstance(*planDetails)
		} else {
			// fetch plan from arc/fs/plan endpoint
			planDetails, err := GetCachedPlanDetails()
			if err != nil {
				log.Errorln("error while refreshing plan, please contact at support@appbase.io")
				// If plan is not set already (that would be the case at the time of initialization)
				// then set the highest appbase.io plan
				plan := GetTier(nil)
				if plan == nil {
					highestPlan := ArcEnterprise
					plan = &highestPlan
				}
				SetTier(plan)
			} else {
				return setBillingVarsArcInstance(planDetails)
			}
		}
		return arcInstance, err
	}
	defer res.Body.Close()
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		log.Errorln("error reading res body:", err)
		return arcInstance, err
	}
	// Validate the ACCAPI response
	ValidateArcID(res.StatusCode)

	return setBillingVarsArcInstance(body)
}

func setBillingVarsArcInstance(body []byte) (ArcInstance, error) {
	response := ArcInstanceResponse{}
	arcInstance := ArcInstance{}
	err := json.Unmarshal(body, &response)
	if err != nil {
		log.Errorln("error while unmarshalling res body:", err)
		return arcInstance, err
	}
	if len(response.ArcInstances) != 0 {
		arcInstanceByID := response.ArcInstances[0]
		arcInstance.SubscriptionID = arcInstanceByID.SubscriptionID
		SetTimeValidity(arcInstanceByID.TimeValidity)
		setMaxErrorTime(Subscripton{
			SubscriptionID:      arcInstanceByID.SubscriptionID,
			SubscriptonCanceled: arcInstanceByID.SubscriptionCanceled,
			CreatedAt:           arcInstanceByID.CreatedAt,
		})
		// Set plan details to local variable
		setPlanDetails(body)
		SetTier(arcInstanceByID.Tier)
		SetBackend(arcInstanceByID.Backend)
		SetFeatureSuggestions(arcInstanceByID.FeatureSuggestions)
		SetFeatureCustomEvents(arcInstanceByID.FeatureCustomEvents)
		SetFeatureRules(arcInstanceByID.FeatureRules)
		SetFeatureSearchRelevancy(arcInstanceByID.FeatureSearchRelevancy)
		SetFeatureSearchGrader(arcInstanceByID.FeatureSearchGrader)
		SetFeatureEcommerce(arcInstanceByID.FeatureEcommerce)
		SetFeatureUIBuilderPremium(arcInstanceByID.FeatureUIBuilderPremium)
		SetFeatureCache(arcInstanceByID.FeatureCache)
		SetFeaturePipelines(arcInstanceByID.FeaturePipelines)
		setNumberOfMachines(arcInstanceByID.NumberOfMachines)
		ClusterID = arcInstanceByID.ClusterID
	} else {
		return arcInstance, errors.New("no valid instance found for the provided APPBASE_ID")
	}
	return arcInstance, nil
}

func getArcClusterInstance(clusterID string) (ArcInstance, error) {
	arcInstance := ArcInstance{}
	url := ACCAPI + "byoc/" + clusterID
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("cache-control", "no-cache")

	res, err := HTTPClient().Do(req)
	// If ACCAPI is down then set the plan
	if (res != nil && res.StatusCode >= 500) || err != nil {
		if planDetails != nil {
			return setBillingVarsArcInstance(*planDetails)
		} else {
			// fetch plan from arc/fs/plan endpoint
			planDetails, err := GetCachedPlanDetails()
			if err != nil {
				log.Errorln("error while refreshing plan, please contact at support@appbase.io")
				// If plan is not set already (that would be the case at the time of initialization)
				// then set the highest appbase.io plan
				plan := GetTier(nil)
				if plan == nil {
					highestPlan := HostedArcEnterprise2021
					plan = &highestPlan
				}
				SetTier(plan)
			} else {
				return setBillingVarsArcInstance(planDetails)
			}
		}
		return arcInstance, err
	}
	defer res.Body.Close()
	// Validate the ACCAPI response
	ValidateArcID(res.StatusCode)

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		log.Errorln("error reading res body:", err)
		return arcInstance, err
	}
	return setBillingVarsArcInstance(body)
}

type Subscripton struct {
	SubscriptonCanceled bool
	SubscriptionID      string
	CreatedAt           int64
}

func setMaxErrorTime(subscriptionDetails Subscripton) {
	// if subscription id is present and
	// subscription is not cancelled
	// and subscription creation date is at least 2 months (60 days) from now
	// then increase the error reporting time to 4 weeks
	if strings.TrimSpace(subscriptionDetails.SubscriptionID) != "" {
		if !subscriptionDetails.SubscriptonCanceled {
			currentTime := time.Now().Unix()                                    // seconds
			subscriptionDuration := currentTime - subscriptionDetails.CreatedAt // seconds
			if subscriptionDuration > 2*30*24*60*60 {
				maxErrorTime = 720 // in hrs (4 weeks)
			}
		}
	}
}

// Fetches the cluster plan details for the encrypted cluster id
func getClusterPlan(clusterID string) (ClusterPlan, error) {
	clusterPlan := ClusterPlan{}
	url := ACCAPI + "v1/plan/" + clusterID
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("cache-control", "no-cache")

	res, err := HTTPClient().Do(req)
	// If ACCAPI is down then set the plan
	if (res != nil && res.StatusCode >= 500) || err != nil {
		if planDetails != nil {
			return setBillingVarsCluster(*planDetails)
		} else {
			// fetch plan from arc/fs/plan endpoint
			planDetails, err := GetCachedPlanDetails()
			if err != nil {
				log.Errorln("error while refreshing plan, please contact at support@appbase.io")
				plan := GetTier(nil)
				if plan == nil {
					highestPlan := ProductionThird2021
					plan = &highestPlan
				}
				SetTier(plan)
			} else {
				return setBillingVarsCluster(planDetails)
			}
		}
		return clusterPlan, err
	}
	defer res.Body.Close()
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		log.Errorln("error reading res body:", err)
		return clusterPlan, err
	}
	return setBillingVarsCluster(body)
}

func setBillingVarsCluster(body []byte) (ClusterPlan, error) {
	clusterPlan := ClusterPlan{}
	var response ClusterPlanResponse
	err := json.Unmarshal(body, &response)
	if err != nil {
		log.Errorln("error while unmarshalling res body:", err)
		return clusterPlan, err
	}
	err = json.Unmarshal(body, &response)

	if err != nil {
		log.Errorln("error while unmarshalling res body:", err)
		return clusterPlan, err
	}
	if response.Plan.Tier == nil {
		return clusterPlan, fmt.Errorf("error while getting the cluster plan")
	}
	// Set plan details to local variable
	setPlanDetails(body)
	// Set the plan for clusters
	SetTier(response.Plan.Tier)
	SetBackend(response.Plan.Backend)
	SetTimeValidity(response.Plan.TimeValidity)
	setMaxErrorTime(Subscripton{
		SubscriptionID:      response.Plan.SubscriptionID,
		SubscriptonCanceled: response.Plan.SubscriptionCanceled,
		CreatedAt:           response.Plan.CreatedAt,
	})
	SetFeatureSuggestions(response.Plan.FeatureSuggestions)
	SetFeatureCustomEvents(response.Plan.FeatureCustomEvents)
	SetFeatureRules(response.Plan.FeatureRules)
	SetFeatureSearchRelevancy(response.Plan.FeatureSearchRelevancy)
	SetFeatureSearchGrader(response.Plan.FeatureSearchGrader)
	SetFeatureEcommerce(response.Plan.FeatureEcommerce)
	SetFeatureUIBuilderPremium(response.Plan.FeatureUIBuilderPremium)
	SetFeatureCache(response.Plan.FeatureCache)
	SetFeaturePipelines(response.Plan.FeaturePipelines)
	setNumberOfMachines(response.Plan.NumberOfMachines)
	ClusterID = response.Plan.ClusterID
	return clusterPlan, nil
}

// SetClusterPlan fetches the cluster plan & sets the Tier value
func SetClusterPlan() {
	log.Println("=> Getting cluster plan details")
	clusterID := os.Getenv(ClusterIDEnvName)
	if clusterID == "" {
		log.Fatalln("CLUSTER_ID env required but not present")
		return
	}
	_, err := getClusterPlan(clusterID)
	if err != nil {
		log.Errorln("Unable to fetch the cluster plan. Please make sure that you're using a valid CLUSTER_ID. If the issue persists please contact support@appbase.io with your APPBASE_ID or registered e-mail address.", err)
		return
	}
}

func reportUsageRequest(arcUsage ArcUsage) (ArcUsageResponse, error) {
	response := ArcUsageResponse{}
	url := ACCAPI + "arc/report_usage"
	marshalledRequest, err := json.Marshal(arcUsage)
	log.Println("ReactiveSearch usage for APPBASE_ID:", arcUsage)
	if err != nil {
		log.Errorln("error while marshalling req body:", err)
		return response, err
	}
	req, _ := http.NewRequest(http.MethodPut, url, bytes.NewBuffer(marshalledRequest))
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("cache-control", "no-cache")

	res, err := HTTPClient().Do(req)
	// If ACCAPI is down then set the plan
	if (res != nil && res.StatusCode >= 500) || err != nil {
		plan := GetTier(nil)
		// If plan is not set already (that would be the case at the time of initialization)
		// then set the highest appbase.io plan
		if plan == nil {
			highestPlan := ArcEnterprise
			plan = &highestPlan
		}
		SetTier(plan)
		log.Errorln("error while sending request:", err)
		return response, err
	}
	defer res.Body.Close()
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		log.Errorln("error reading res body:", err)
		return response, err
	}
	err = json.Unmarshal(body, &response)
	if err != nil {
		log.Errorln("error while unmarshalling res body:", err)
		return response, err
	}
	return response, nil
}

func reportClusterUsageRequest(arcUsage ArcUsage) (ArcUsageResponse, error) {
	response := ArcUsageResponse{}
	url := ACCAPI + "byoc/report_usage"
	marshalledRequest, err := json.Marshal(arcUsage)
	log.Println("ReactiveSearch usage for Cluster ID:", arcUsage)
	if err != nil {
		log.Errorln("error while marshalling req body:", err)
		return response, err
	}
	req, _ := http.NewRequest(http.MethodPut, url, bytes.NewBuffer(marshalledRequest))
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("cache-control", "no-cache")

	res, err := HTTPClient().Do(req)
	// If ACCAPI is down then set the plan
	if (res != nil && res.StatusCode >= 500) || err != nil {
		plan := GetTier(nil)
		// If plan is not set already (that would be the case at the time of initialization)
		// then set the highest hosted appbase.io plan
		if plan == nil {
			highestPlan := HostedArcEnterprise2021
			plan = &highestPlan
		}
		SetTier(plan)
		log.Errorln("error while sending request:", err)
		return response, err
	}
	defer res.Body.Close()
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		log.Errorln("error reading res body:", err)
		return response, err
	}
	err = json.Unmarshal(body, &response)

	if err != nil {
		log.Errorln("error while unmarshalling res body:", err)
		return response, err
	}
	return response, nil
}

// GetAppbaseID to get appbase id
func GetAppbaseID() (string, error) {
	arcID := os.Getenv(ArcIDEnvName)
	if arcID == "" {
		appbaseID := os.Getenv(AppbaseIDEnvName)
		if appbaseID == "" {
			return "", errors.New("APPBASE_ID env required but not present")
		} else {
			arcID = appbaseID
		}
	}

	return arcID, nil
}

// ReportUsage reports ReactiveSearch usage, intended to be called every hour
func ReportUsage() {
	arcID, err := GetAppbaseID()
	if err != nil {
		log.Fatalln(err)
		return
	}

	result, err := getArcInstance(arcID)
	if err != nil {
		log.Errorln("Unable to fetch the appbase.io instance. Please make sure that you're using a valid APPBASE_ID. If the issue persists please contact support@appbase.io with your APPBASE_ID or registered e-mail address.")
		return
	}

	NodeCount, err = fetchNodeCount()
	if err != nil || NodeCount <= 0 {
		log.Errorln("Unable to fetch a correct node count:", err)
	}

	subID := result.SubscriptionID
	if subID == "" {
		log.Println("SUBSCRIPTION_ID not found. Initializing in trial mode")
		return
	}

	usageBody := ArcUsage{
		ArcID:          arcID,
		SubscriptionID: subID,
		Quantity:       NodeCount,
		MachineID:      MachineID,
	}
	response, err1 := reportUsageRequest(usageBody)
	if err1 != nil {
		log.Errorln("Please contact support@appbase.io with your APPBASE_ID or registered e-mail address. Usage is not getting reported:", err1)
	}

	if response.WarningMsg != "" {
		log.Warn("warning:", response.WarningMsg)
	}
	if response.ErrorMsg != "" {
		log.Errorln("error:", response.ErrorMsg)
	}
}

// ReportHostedArcUsage reports ReactiveSearch usage by hosted cluster, intended to be called every hour
func ReportHostedArcUsage() {
	log.Println("=> Reporting hosted ReactiveSearch usage")
	clusterID := os.Getenv("CLUSTER_ID")
	if clusterID == "" {
		log.Fatalln("CLUSTER_ID env required but not present")
		return
	}

	// getArcClusterInstance(clusterId)
	result, err := getArcClusterInstance(clusterID)
	if err != nil {
		log.Errorln("Unable to fetch the ReactiveSearch API server. Please make sure that you're using a valid CLUSTER_ID. If the issue persists please contact support@appbase.io with your APPBASE_ID or registered e-mail address.", err)
		return
	}

	NodeCount, err = fetchNodeCount()
	if err != nil || NodeCount <= 0 {
		log.Errorln("Unable to fetch a correct node count:", err)
	}

	subID := result.SubscriptionID
	if subID == "" {
		log.Println("SUBSCRIPTION_ID not found. Initializing in trial mode")
		return
	}

	usageBody := ArcUsage{
		ClusterID:      clusterID,
		SubscriptionID: subID,
		Quantity:       NodeCount,
		MachineID:      MachineID,
	}
	response, err1 := reportClusterUsageRequest(usageBody)
	if err1 != nil {
		log.Errorln("Please contact support@appbase.io with your CLUSTER_ID or registered e-mail address. Usage is not getting reported:", err1)
	}

	if response.WarningMsg != "" {
		log.Warn("warning:", response.WarningMsg)
	}
	if response.ErrorMsg != "" {
		log.Errorln("error:", response.ErrorMsg)
	}
}

// fetchNodeCount returns the number of current ElasticSearch nodes
func fetchNodeCount() (int, error) {
	nodes, err := GetTotalNodes()
	if err != nil {
		return 0, err
	}
	return nodes, nil
}

func IsBillingEnabled() bool {
	return Billing == "true" || ClusterBilling == "true" || HostedBilling == "true"
}

// BillingBlacklistedPaths will return an array of paths
// that should not be affected if billing is enabled.
func BillingBlacklistedPaths() []string {
	return []string{
		"/arc/subscription",
		"/arc/plan",
		"/arc/health",
		"/arc/_health",
		"/reactivesearch/endpoints",
	}
}

// UsageBlacklistedPaths will return an array of paths
// that should not be considered if recording is enabled.
func UsageBlacklistedPaths() []string {
	return []string{
		"/arc/health",
		"/arc/_health",
	}
}

// ShouldRecordUsage will check if the usage should be
// recorded for the passed path
func ShouldRecordUsage(path string) bool {
	for _, blacklistedPath := range UsageBlacklistedPaths() {
		if strings.HasPrefix(path, blacklistedPath) {
			return false
		}
	}

	return true
}
