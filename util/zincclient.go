package util

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"

	log "github.com/sirupsen/logrus"
)

type ZincClient struct {
	URL        string
	Username   string
	Password   string
	AuthHeader string
}

var (
	zincClientInit sync.Once
	zincClient     *ZincClient
)

// IndexService will be used to make index requests to Zinc
type IndexService struct {
	Endpoint        string
	Method          string
	internalHeaders *http.Header
	Body            []byte
}

// GetZincData will return the zinc data from the
// environment.
//
// The return will be three strings:
// - URL
// - username
// - password
func GetZincData() (string, string, string) {
	zincURL := os.Getenv("ZINC_CLUSTER_URL")

	if zincURL == "" {
		log.Warnln("Error encountered: ", fmt.Errorf("ZINC_CLUSTER_URL must be set in the environment variables"))
		zincURL = "http://appbase:zincf0rappbase@localhost:4080"
	}

	username, password := "", ""

	if strings.Contains(zincURL, "@") {
		splitIndex := strings.LastIndex(zincURL, "@")
		protocolWithCredentials := strings.Split(zincURL[0:splitIndex], "://")
		credentials := protocolWithCredentials[1]
		protocol := protocolWithCredentials[0]
		host := zincURL[splitIndex+1:]

		credentialSeparator := strings.Index(credentials, ":")
		username = credentials[0:credentialSeparator]
		password = credentials[credentialSeparator+1:]

		zincURL = fmt.Sprintf("%s://%s", protocol, host)

	}
	return zincURL, username, password
}

// GetZincClient will return the zinc client and only
// init it once.
func GetZincClient() *ZincClient {
	// initialize the client if not present
	if zincClient == nil {
		initZincClient()
	}
	return zincClient
}

// initZincClient will initiate the zinc client
// by extracting the details from the env file.
func initZincClient() {
	zincURL, username, password := GetZincData()
	authHeader := ""

	if username != "" && password != "" {
		authHeader = base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", username, password)))
	}

	zincClient = &ZincClient{
		URL:        zincURL,
		Username:   username,
		Password:   password,
		AuthHeader: authHeader,
	}
}

// MakeRequest will allow making a request to zinc index.
func (zc *ZincClient) MakeRequest(endpoint string, method string, body []byte, headers *http.Header) (*http.Response, error) {
	urlToHit := fmt.Sprintf("%s/%s", zc.URL, endpoint)

	if headers == nil {
		defaultHeader := make(http.Header)
		headers = &defaultHeader
	}

	// Create the request
	request, requestCreateErr := http.NewRequest(method, urlToHit, bytes.NewReader(body))
	if requestCreateErr != nil {
		// Handle the error
		log.Warnln(": error while creating request to send Zinc, ", requestCreateErr)
		return nil, requestCreateErr
	}

	// If authHeader is not empty, set it as basic auth
	if zc.AuthHeader != "" {
		headers.Set("Authorization", fmt.Sprintf("Basic %s", zc.AuthHeader))
	}

	// Set the headers
	for key, value := range *headers {
		request.Header.Set(key, strings.Join(value, ", "))
	}

	// Send the request now
	response, responseErr := HTTPClient().Do(request)

	return response, responseErr
}

// Index will return an IndexService object with the passed details
func (zc *ZincClient) Index(endpoint string, method string, body []byte) *IndexService {
	// Create a new index service object

	newIndexService := IndexService{
		Endpoint:        endpoint,
		Method:          method,
		Body:            body,
		internalHeaders: nil,
	}

	return &newIndexService
}

// Headers will allow adding headers to the request
func (is *IndexService) Headers(headers *http.Header) *IndexService {
	is.internalHeaders = headers
	return is
}

// NewClient instantiates the Zinc Client
func NewZincClient() {
	zincClientInit.Do(func() {
		// Initialize the zinc client
		initZincClient()

		log.Println("zinc client instantiated")
	})
}
