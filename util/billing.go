package util

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
)

const ArcIDEnvName = "ARC_ID"
const ClusterIDEnvName = "CLUSTER_ID"
const AppbaseIDEnvName = "APPBASE_ID"

// ACCAPI URL
var ACCAPI = "https://accapi.appbase.io/"

// var ACCAPI = "http://localhost:3000/"

// Tier is the value of the user's plan
var tier *Plan

// SetTier sets the tier value
func SetTier(plan *Plan) {
	tier = plan
}

// GetTier returns the current tier
func GetTier() *Plan {
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
	Tier                   *Plan  `json:"tier"`
	FeatureCustomEvents    bool   `json:"feature_custom_events"`
	FeatureSuggestions     bool   `json:"feature_suggestions"`
	FeatureRules           bool   `json:"feature_rules"`
	FeatureSearchRelevancy bool   `json:"feature_search_relevancy"`
	FeatureSearchGrader    bool   `json:"feature_search_grader"`
	FeatureEcommerce       bool   `json:"feature_ecommerce"`
	FeatureCache           bool   `json:"feature_cache"`
	FeaturePipelines       bool   `json:"feature_pipelines"`
	Trial                  bool   `json:"trial"`
	TrialValidity          int64  `json:"trial_validity"`
	TierValidity           int64  `json:"tier_validity"`
	TimeValidity           int64  `json:"time_validity"`
	SubscriptionID         string `json:"subscription_id"`
	ClusterID              string `json:"cluster_id"`
	NumberOfMachines       int64  `json:"number_of_machines"`
	SubscriptionCanceled   bool   `json:"subscription_canceled"`
	CreatedAt              int64  `json:"created_at"`
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
	NodeCount              int                    `json:"node_count"`
	Description            string                 `json:"description"`
	SubscriptionID         string                 `json:"subscription_id"`
	SubscriptionCanceled   bool                   `json:"subscription_canceled"`
	Trial                  bool                   `json:"trial"`
	TrialValidity          int64                  `json:"trial_validity"`
	CreatedAt              int64                  `json:"created_at"`
	Tier                   *Plan                  `json:"tier"`
	TierValidity           int64                  `json:"tier_validity"`
	TimeValidity           int64                  `json:"time_validity"`
	Metadata               map[string]interface{} `json:"metadata"`
	FeatureCustomEvents    bool                   `json:"feature_custom_events"`
	FeatureSuggestions     bool                   `json:"feature_suggestions"`
	FeatureRules           bool                   `json:"feature_rules"`
	FeatureSearchRelevancy bool                   `json:"feature_search_relevancy"`
	FeatureSearchGrader    bool                   `json:"feature_search_grader"`
	FeatureEcommerce       bool                   `json:"feature_ecommerce"`
	FeatureCache           bool                   `json:"feature_cache"`
	FeaturePipelines       bool                   `json:"feature_pipelines"`
	ClusterID              string                 `json:"cluster_id"`
	NumberOfMachines       int64                  `json:"number_of_machines"`
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

func validateTimeValidity() bool {
	if GetTimeValidity() > 0 { // Valid plan
		return true
	} else if GetTimeValidity() <= 0 && -GetTimeValidity() < 3600*maxErrorTime { // Negative validity, plan has been expired
		// Print warning message if remaining time is less than max allowed time
		log.Println("Warning: Payment is required. Arc will start sending out error messages in next", maxErrorTime, "hours")
		return true
	}
	return false
}

// BillingMiddleware function to be called for each request
func BillingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Println("current time validity value: ", GetTimeValidity())

		if isInvalidArcIDUsed {
			// throw invalid APPBASE_ID usage error
			WriteBackError(w, "Please make sure that you're using a valid APPBASE_ID. If the issue persists please contact support@appbase.io with your APPBASE_ID or registered e-mail address.", http.StatusBadRequest)
			return
		}
		// Blacklist subscription routes
		if strings.HasPrefix(r.RequestURI, "/arc/subscription") || strings.HasPrefix(r.RequestURI, "/arc/plan") {
			next.ServeHTTP(w, r)
		} else if validateTimeValidity() {
			next.ServeHTTP(w, r)
		} else {
			// Write an error and stop the handler chain
			WriteBackError(w, "Payment required", http.StatusPaymentRequired)
			return
		}
	})
}

func getArcInstance(arcID string) (ArcInstance, error) {
	arcInstance := ArcInstance{}
	response := ArcInstanceResponse{}
	url := ACCAPI + "arc/instances?arcid=" + arcID
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("cache-control", "no-cache")

	res, err := http.DefaultClient.Do(req)
	// If ACCAPI is down then set the plan
	if (res != nil && res.StatusCode >= 500) || err != nil {
		plan := GetTier()
		// If plan is not set already (that would be the case at the time of initialization)
		// then set the highest appbase.io plan
		if plan == nil {
			highestPlan := ArcEnterprise
			plan = &highestPlan
		}
		SetTier(plan)
		log.Errorln("error while sending request:", err)
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

	err = json.Unmarshal(body, &response)
	if len(response.ArcInstances) != 0 {
		arcInstanceByID := response.ArcInstances[0]
		arcInstance.SubscriptionID = arcInstanceByID.SubscriptionID
		SetTimeValidity(arcInstanceByID.TimeValidity)
		setMaxErrorTime(Subscripton{
			SubscriptionID:      arcInstanceByID.SubscriptionID,
			SubscriptonCanceled: arcInstanceByID.SubscriptionCanceled,
			CreatedAt:           arcInstanceByID.CreatedAt,
		})
		SetTier(arcInstanceByID.Tier)
		SetFeatureSuggestions(arcInstanceByID.FeatureSuggestions)
		SetFeatureCustomEvents(arcInstanceByID.FeatureCustomEvents)
		SetFeatureRules(arcInstanceByID.FeatureRules)
		SetFeatureSearchRelevancy(arcInstanceByID.FeatureSearchRelevancy)
		SetFeatureSearchGrader(arcInstanceByID.FeatureSearchGrader)
		SetFeatureEcommerce(arcInstanceByID.FeatureEcommerce)
		SetFeatureCache(arcInstanceByID.FeatureCache)
		SetFeaturePipelines(arcInstanceByID.FeaturePipelines)
		setNumberOfMachines(arcInstanceByID.NumberOfMachines)
		ClusterID = arcInstanceByID.ClusterID
	} else {
		return arcInstance, errors.New("no valid instance found for the provided APPBASE_ID")
	}

	if err != nil {
		log.Errorln("error while unmarshalling res body:", err)
		return arcInstance, err
	}
	return arcInstance, nil
}

func getArcClusterInstance(clusterID string) (ArcInstance, error) {
	arcInstance := ArcInstance{}
	var response ArcInstanceResponse
	url := ACCAPI + "byoc/" + clusterID
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("cache-control", "no-cache")

	res, err := http.DefaultClient.Do(req)
	// If ACCAPI is down then set the plan
	if (res != nil && res.StatusCode >= 500) || err != nil {
		plan := GetTier()
		// If plan is not set already (that would be the case at the time of initialization)
		// then set the highest appbase.io plan
		if plan == nil {
			highestPlan := HostedArcEnterprise2021
			plan = &highestPlan
		}
		SetTier(plan)
		log.Errorln("error while sending request:", err)
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
	err = json.Unmarshal(body, &response)

	if err != nil {
		log.Errorln("error while unmarshalling res body:", err)
		return arcInstance, err
	}
	if len(response.ArcInstances) != 0 {
		arcInstanceDetails := response.ArcInstances[0]
		arcInstance.SubscriptionID = arcInstanceDetails.SubscriptionID
		SetTimeValidity(arcInstanceDetails.TimeValidity)
		setMaxErrorTime(Subscripton{
			SubscriptionID:      arcInstanceDetails.SubscriptionID,
			SubscriptonCanceled: arcInstanceDetails.SubscriptionCanceled,
			CreatedAt:           arcInstanceDetails.CreatedAt,
		})
		SetTier(arcInstanceDetails.Tier)
		SetFeatureSuggestions(arcInstanceDetails.FeatureSuggestions)
		SetFeatureCustomEvents(arcInstanceDetails.FeatureCustomEvents)
		SetFeatureRules(arcInstanceDetails.FeatureRules)
		SetFeatureSearchRelevancy(arcInstanceDetails.FeatureSearchRelevancy)
		SetFeatureSearchGrader(arcInstanceDetails.FeatureSearchGrader)
		SetFeatureEcommerce(arcInstanceDetails.FeatureEcommerce)
		SetFeatureCache(arcInstanceDetails.FeatureCache)
		SetFeaturePipelines(arcInstanceDetails.FeaturePipelines)
		setNumberOfMachines(arcInstanceDetails.NumberOfMachines)
		ClusterID = arcInstanceDetails.ClusterID
	} else {
		return arcInstance, errors.New("no valid instance found for the provided CLUSTER_ID")
	}
	return arcInstance, nil
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
	var response ClusterPlanResponse
	url := ACCAPI + "v1/plan/" + clusterID
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("cache-control", "no-cache")

	res, err := http.DefaultClient.Do(req)
	// If ACCAPI is down then set the plan
	if (res != nil && res.StatusCode >= 500) || err != nil {
		plan := GetTier()
		// If plan is not set already (that would be the case at the time of initialization)
		// then set the highest cluster plan
		if plan == nil {
			highestPlan := ProductionThird2021
			plan = &highestPlan
		}
		SetTier(plan)
		log.Errorln("error while sending request:", err)
		return clusterPlan, err
	}
	defer res.Body.Close()
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		log.Errorln("error reading res body:", err)
		return clusterPlan, err
	}
	// Validate the ACCAPI response
	ValidateArcID(res.StatusCode)

	err = json.Unmarshal(body, &response)

	if err != nil {
		log.Errorln("error while unmarshalling res body:", err)
		return clusterPlan, err
	}

	if response.Plan.Tier == nil {
		return clusterPlan, fmt.Errorf("error while getting the cluster plan")
	}
	// Set the plan for clusters
	SetTier(response.Plan.Tier)
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

	res, err := http.DefaultClient.Do(req)
	// If ACCAPI is down then set the plan
	if (res != nil && res.StatusCode >= 500) || err != nil {
		plan := GetTier()
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

	res, err := http.DefaultClient.Do(req)
	// If ACCAPI is down then set the plan
	if (res != nil && res.StatusCode >= 500) || err != nil {
		plan := GetTier()
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
	url := os.Getenv("ES_CLUSTER_URL")
	if url == "" {
		log.Fatalln("ES_CLUSTER_URL env required but not present")
		return
	}

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
	url := os.Getenv("ES_CLUSTER_URL")
	if url == "" {
		log.Fatalln("ES_CLUSTER_URL env required but not present")
		return
	}
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
