package util

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io/ioutil"
	"log"
	"net/http"
	"os"

	"github.com/olivere/elastic/v7"
)

// ACCAPI URL
var ACCAPI = "https://accapi.appbase.io/"

// var ACCAPI = "http://localhost:3000/"

// TimeValidity to be obtained from ACCAPI
var TimeValidity int64

// MaxErrorTime before showing errors if invalid trial / plan in hours
var MaxErrorTime int64 = 24 // in hrs

// NodeCount is the current node count, defaults to 1
var NodeCount = 1

// ArcUsage struct is used to report time usage
type ArcUsage struct {
	ArcID          string `json:"arc_id"`
	SubscriptionID string `json:"subscription_id"`
	Quantity       int    `json:"quantity"`
	ClusterID      string `json:"cluster_id"`
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

// ArcInstanceDetails contains the info about an Arc Instance
type ArcInstanceDetails struct {
	NodeCount            int                    `json:"node_count"`
	Description          string                 `json:"description"`
	SubscriptionID       string                 `json:"subscription_id"`
	SubscriptionCanceled bool                   `json:"subscription_canceled"`
	Trial                bool                   `json:"trial"`
	TrialValidity        int64                  `json:"trial_validity"`
	ArcID                string                 `json:"arc_id"`
	CreatedAt            int64                  `json:"created_at"`
	Tier                 string                 `json:"tier"`
	TierValidity         int64                  `json:"tier_validity"`
	TimeValidity         int64                  `json:"time_validity"`
	Metadata             map[string]interface{} `json:"metadata"`
}

// BillingMiddleware function to be called for each request
func BillingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Println("current time validity value: ", TimeValidity)
		if TimeValidity > 0 { // Valid plan
			next.ServeHTTP(w, r)
		} else if TimeValidity <= 0 && TimeValidity < -3600*MaxErrorTime { // Negative validity, plan has been expired
			// Print warning message if remaining time is less than max allowed time
			log.Println("Warning: Payment is required. Arc will start sending out error messages in next", MaxErrorTime, "hours")
			next.ServeHTTP(w, r)
		} else {
			// Write an error and stop the handler chain
			http.Error(w, "payment required", http.StatusPaymentRequired)
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
	if err != nil {
		log.Println("error while sending request: ", err)
		return arcInstance, err
	}
	defer res.Body.Close()
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		log.Println("error reading res body: ", err)
		return arcInstance, err
	}
	err = json.Unmarshal(body, &response)
	if len(response.ArcInstances) != 0 {
		arcInstance.SubscriptionID = response.ArcInstances[0].SubscriptionID
		TimeValidity = response.ArcInstances[0].TimeValidity
	} else {
		return arcInstance, errors.New("No valid instance found for the provided ARC_ID")
	}

	if err != nil {
		log.Println("error while unmarshalling res body: ", err)
		return arcInstance, err
	}
	return arcInstance, nil
}

func getArcClusterInstance(clusterID string) (ArcInstance, error) {
	arcInstance := ArcInstance{}
	var response ArcInstanceResponse
	url := ACCAPI + "arc_cluster/" + clusterID
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("cache-control", "no-cache")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Println("error while sending request: ", err)
		return arcInstance, err
	}
	defer res.Body.Close()
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		log.Println("error reading res body: ", err)
		return arcInstance, err
	}
	err = json.Unmarshal(body, &response)

	if err != nil {
		log.Println("error while unmarshalling res body: ", err)
		return arcInstance, err
	}
	if len(response.ArcInstances) != 0 {
		arcInstance.SubscriptionID = response.ArcInstances[0].SubscriptionID
	} else {
		return arcInstance, errors.New("No valid instance found for the provided CLUSTER_ID")
	}
	return arcInstance, nil
}

func reportUsageRequest(arcUsage ArcUsage) (ArcUsageResponse, error) {
	response := ArcUsageResponse{}
	url := ACCAPI + "arc/" + arcUsage.ArcID + "/report_usage"
	marshalledRequest, err := json.Marshal(arcUsage)
	log.Println("Arc usage for Arc ID: ", arcUsage)
	if err != nil {
		log.Println("error while marshalling req body: ", err)
		return response, err
	}
	req, _ := http.NewRequest("POST", url, bytes.NewBuffer(marshalledRequest))
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("cache-control", "no-cache")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Println("error while sending request: ", err)
		return response, err
	}
	defer res.Body.Close()
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		log.Println("error reading res body: ", err)
		return response, err
	}
	err = json.Unmarshal(body, &response)

	if err != nil {
		log.Println("error while unmarshalling res body: ", err)
		return response, err
	}
	return response, nil
}

func reportClusterUsageRequest(arcUsage ArcUsage) (ArcUsageResponse, error) {
	response := ArcUsageResponse{}
	url := ACCAPI + "arc_cluster/report_usage"
	marshalledRequest, err := json.Marshal(arcUsage)
	log.Println("Arc usage for Cluster ID: ", arcUsage)
	if err != nil {
		log.Println("error while marshalling req body: ", err)
		return response, err
	}
	req, _ := http.NewRequest("PUT", url, bytes.NewBuffer(marshalledRequest))
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("cache-control", "no-cache")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Println("error while sending request: ", err)
		return response, err
	}
	defer res.Body.Close()
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		log.Println("error reading res body: ", err)
		return response, err
	}
	err = json.Unmarshal(body, &response)

	if err != nil {
		log.Println("error while unmarshalling res body: ", err)
		return response, err
	}
	return response, nil
}

// ReportUsage reports Arc usage, intended to be called every hour
func ReportUsage() {
	url := os.Getenv("ES_CLUSTER_URL")
	if url == "" {
		log.Fatalln("ES_CLUSTER_URL env required but not present")
		return
	}
	arcID := os.Getenv("ARC_ID")
	if arcID == "" {
		log.Fatalln("ARC_ID env required but not present")
		return
	}

	result, err := getArcInstance(arcID)
	if err != nil {
		log.Println("Unable to fetch the arc instance. Please make sure that you're using a valid ARC_ID.")
		return
	}

	NodeCount, err = fetchNodeCount(url)
	if err != nil || NodeCount <= 0 {
		log.Println("Unable to fetch a correct node count: ", err)
	}

	subID := result.SubscriptionID
	if subID == "" {
		log.Println("SUBSCRIPTION_ID not found. Initializing in trial mode")
		return
	}

	usageBody := ArcUsage{}
	usageBody.ArcID = arcID
	usageBody.SubscriptionID = subID
	usageBody.Quantity = NodeCount
	response, err1 := reportUsageRequest(usageBody)
	if err1 != nil {
		log.Println("Please contact support@appbase.io with your ARC_ID or registered e-mail address. Usage is not getting reported: ", err1)
	}

	TimeValidity = response.TimeValidity
	if response.WarningMsg != "" {
		log.Println("warning:", response.WarningMsg)
	}
	if response.ErrorMsg != "" {
		log.Println("error:", response.ErrorMsg)
	}
}

// ReportHostedArcUsage reports Arc usage by hosted cluster, intended to be called every hour
func ReportHostedArcUsage() {
	log.Printf("=> Reporting hosted arc usage")
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
		log.Println("Unable to fetch the arc instance. Please make sure that you're using a valid CLUSTER_ID.", err)
		return
	}

	NodeCount, err = fetchNodeCount(url)
	if err != nil || NodeCount <= 0 {
		log.Println("Unable to fetch a correct node count: ", err)
	}

	subID := result.SubscriptionID
	if subID == "" {
		log.Println("SUBSCRIPTION_ID not found. Initializing in trial mode")
		return
	}

	usageBody := ArcUsage{}
	usageBody.ClusterID = clusterID
	usageBody.SubscriptionID = subID
	usageBody.Quantity = NodeCount
	response, err1 := reportClusterUsageRequest(usageBody)
	if err1 != nil {
		log.Println("Please contact support@appbase.io with your ARC_ID or registered e-mail address. Usage is not getting reported: ", err1)
	}

	// TimeValidity = response.TimeValidity
	if response.WarningMsg != "" {
		log.Println("warning:", response.WarningMsg)
	}
	if response.ErrorMsg != "" {
		log.Println("error:", response.ErrorMsg)
	}
}

// fetchNodeCount returns the number of current ElasticSearch nodes
func fetchNodeCount(url string) (int, error) {
	ctx := context.Background()
	// Initialize the client
	client, err := elastic.NewClient(
		elastic.SetURL(url),
		elastic.SetRetrier(NewRetrier()),
		elastic.SetSniff(false),
		elastic.SetHttpClient(HTTPClient()),
	)
	if err != nil {
		log.Fatalln("unable to initialize elastic client: ", err)
	}
	nodes, err := client.NodesInfo().
		Metric("nodes").
		Do(ctx)
	if err != nil {
		return -1, err
	}
	return len(nodes.Nodes), nil
}
