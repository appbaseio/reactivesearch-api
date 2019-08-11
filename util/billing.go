package util

import (
	"bytes"
	"context"
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/olivere/elastic/v7"
)

var TimeValidity int64
var MAX_ALLOWED_TIME int64 = 24 // in hrs
var ACC_API = "http://localhost:3000/"

type ArcUsage struct {
	ArcID          string `json:"arc_id"`
	Timestamp      int64  `json:"timestamp"`
	SubscriptionID string `json:"subscription_id"`
	Quantity       int    `json:"quantity"`
}

type ArcUsageResponse struct {
	Accepted      bool   `json:"accepted"`
	FailureReason string `json:"failure_reason"`
	ErrorMsg      string `json:"error_msg"`
	WarningMsg    string `json:"warning_msg"`
	StatusCode    int    `json:"status_code"`
	TimeValidity  int64  `json:"time_validity"`
}

type ArcInstance struct {
	SubscriptionID string `json:"subscription_id"`
}

type ArcInstanceResponse struct {
	ArcInstances []arcInstanceDetails `json:"instances"`
}

type arcInstanceDetails struct {
	NodeCount            int64                  `json:"node_count"`
	Description          string                 `json:"description"`
	SubscriptionID       string                 `json:"subscription_id"`
	SubscriptionCanceled bool                   `json:"subscription_canceled"`
	Trial                bool                   `json:"trial"`
	TrialValidity        int64                  `json:"trial_validity"`
	ArcID                string                 `json:"arc_id"`
	CreatedAt            int64                  `json:"created_at"`
	Tier                 string                 `json:"tier"`
	TierValidity         int64                  `json:"tier_validity"`
	Metadata             map[string]interface{} `json:"metadata"`
}

const (
	envEsURL      = "ES_CLUSTER_URL"
	arcIdentifier = "ARC_ID"
)

// Middleware function, which will be called for each request
func BillingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Println("current time validity value: ", TimeValidity)
		if TimeValidity >= 0 { // Valid plan
			next.ServeHTTP(w, r)
		} else if TimeValidity < 0 && TimeValidity < -3600*MAX_ALLOWED_TIME { // Negative validity, plan has been expired
			// Print warning message if remaining time is less than max allowed time
			log.Println("warning: payment required. arc will start sending out error messages in next", MAX_ALLOWED_TIME, "hours")
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
	url := ACC_API + "arc/instances?arcid=" + arcID
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
	log.Println("response body: ", body)
	err = json.Unmarshal(body, &response)
	if len(response.ArcInstances) != 0 {
		arcInstance.SubscriptionID = response.ArcInstances[0].SubscriptionID
	}

	if err != nil {
		log.Println("error while unmarshalling res body: ", err)
		return arcInstance, err
	}
	return arcInstance, nil
}

func ReportUsageRequest(arcUsage ArcUsage) (ArcUsageResponse, error) {
	response := ArcUsageResponse{}
	url := ACC_API + "arc/" + arcUsage.ArcID + "/report_usage"
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

func ReportUsage() {
	url := os.Getenv(envEsURL)
	if url == "" {
		log.Fatalln("ES_CLUSTER_URL not found")
		return
	}
	arcID := os.Getenv(arcIdentifier)
	if arcID == "" {
		log.Fatalln("ARC_ID not found")
		return
	}

	result, err := getArcInstance(arcID)
	if err != nil {
		log.Println("Unable to fetch the arc instance. Please make sure that you're using a valid ARC_ID.")
		return
	}

	subID := result.SubscriptionID
	if subID == "" {
		log.Println("SUBSCRIPTION_ID not found. Initializing in trial mode")
		return
	}
	nodeCount, err := FetchNodeCount(url)
	if err != nil || nodeCount == -1 {
		log.Println("unable to fetch node count: ", err)
	}
	usageBody := ArcUsage{}
	usageBody.ArcID = arcID
	usageBody.SubscriptionID = subID
	usageBody.Timestamp = time.Now().Unix()
	usageBody.Quantity = nodeCount
	response, err1 := ReportUsageRequest(usageBody)
	if err1 != nil {
		log.Println("please contact support. Usage not getting reported: ", err1)
	}

	TimeValidity = response.TimeValidity
	if response.WarningMsg != "" {
		log.Println("warning:", response.WarningMsg)
	}
	if response.ErrorMsg != "" {
		log.Println("error:", response.ErrorMsg)
	}
}

func FetchNodeCount(url string) (int, error) {
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
