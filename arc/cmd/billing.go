package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/appbaseio-confidential/arc/util"
	"github.com/olivere/elastic"
)

const (
	envEsURL       = "ES_CLUSTER_URL"
	arcIdentifier  = "ARC_ID"
	emailID        = "EMAIL"
	subscriptionID = "SUBSCRIPTION_ID"
)

// Middleware function, which will be called for each request
func BillingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if util.BillingOK {
			next.ServeHTTP(w, r)
		} else {
			// Write an error and stop the handler chain
			http.Error(w, "payment required", http.StatusPaymentRequired)
		}
	})
}

type ArcUsage struct {
	ArcID          string `json:"arc_id"`
	Timestamp      int64  `json:"timestamp"`
	SubscriptionID string `json:"subscription_id"`
	Quantity       int    `json:"quantity"`
	Email          string `json:"email"`
}

type ArcUsageResponse struct {
	Accepted      bool   `json:"accepted"`
	FailureReason string `json:"failure_reason"`
	ErrorMsg      string `json:"error_msg"`
	WarningMsg    string `json:"warning_msg"`
	StatusCode    int    `json:"status_code"`
}

func ReportUsageRequest(arcUsage ArcUsage) (ArcUsageResponse, error) {
	response := ArcUsageResponse{}
	url := "https://accapi.appbase.io/arc/" + arcUsage.ArcID + "/report_usage"
	marshalledRequest, err := json.Marshal(arcUsage)
	if err != nil {
		log.Println("error while marshalling req body: ", err)
		return response, err
	}
	req, _ := http.NewRequest("POST", url, bytes.NewBuffer(marshalledRequest))
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("cache-control", "no-cache")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Println("error while sending reequest: ", err)
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
	}
	arcID := os.Getenv(arcIdentifier)
	if url == "" {
		log.Fatalln("ARC_ID not found")
	}
	email := os.Getenv(emailID)
	if url == "" {
		log.Fatalln("EMAIL not found")
	}
	subID := os.Getenv(subscriptionID)
	if url == "" {
		log.Println("SUBSCRIPTION_ID not found. Initializing in trial mode")
	}
	nodeCount, err := FetchNodeCount()
	if err != nil || nodeCount == -1 {
		log.Println("unable to fetch node count: ", err)
	}
	usageBody := ArcUsage{}
	usageBody.ArcID = arcID
	usageBody.Email = email
	usageBody.SubscriptionID = subscriptionID
	usageBody.Timestamp = time.Now().Unix()
	usageBody.Quantity = nodeCount
	response, err := ReportUsageRequest(usageBody)
	if err != nil {
		log.Println("please contact support. Usage not getting reported: ", err)
	}

	if response.StatusCode != 0 {
		util.BillingOK = response.Accepted
	}
	if response.Accepted {
		util.BillingErrorCount = 0
	}
	if response.ErrorMsg != "" || response.StatusCode == 402 || !response.Accepted {
		util.BillingErrorCount++
	}
	if response.WarningMsg != "" {
		log.Println("warning:", response.WarningMsg)
	}
	if response.ErrorMsg != "" {
		log.Println("error:", response.ErrorMsg)
	}
}

func FetchNodeCount() (int, error) {
	ctx := context.Background()
	// Initialize the client
	client, err := elastic.NewClient(
		elastic.SetURL(url),
		elastic.SetRetrier(util.NewRetrier()),
		elastic.SetSniff(false),
		elastic.SetHttpClient(util.HTTPClient()),
	)
	if err != nil {
		log.Fatalln("unable to initialize elastic client: ", err)
	}
	nodes, err := client.NodesInfo().
		Metric("nodes").
		Do(context.Background())
	if err != nil {
		return -1, err
	}
	return nodes.Nodes, nil
}
