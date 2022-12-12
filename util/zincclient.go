package util

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"io/ioutil"
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
	zincTag        string = "[zinc]"
)

// RequestService will be used to make requests to Zinc
type RequestService struct {
	Endpoint        string
	Method          string
	internalHeaders *http.Header
	Body            []byte
	clientToUse     *ZincClient
}

// IndexService will be used to make index requests to Zinc
type IndexService struct {
	RequestService
}

// BulkService will be used to make bulk requests to Zinc
type BulkService struct {
	RequestService
}

// SearchService will be used to make search requests to Zinc
type SearchService struct {
	RequestService
}

// UpdateService will be used to make update requests to Zinc
type UpdateService struct {
	RequestService
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
func (zc *ZincClient) MakeRequest(endpoint string, method string, body []byte, headers *http.Header, ctx context.Context) (*http.Response, error) {
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

	if MultiTenant {
		// Read the body, remove tenant ID and then return it
		responseBody, readErr := io.ReadAll(response.Body)

		if readErr != nil {
			return nil, fmt.Errorf("error while reading response to remove tenant_id: %s", readErr.Error())
		}

		updatedResponseBody, hideErr := HideTenantID(responseBody, ctx)
		if hideErr != nil {
			return nil, fmt.Errorf("error while hiding tenant_id from body: %s", hideErr.Error())
		}

		// TODO: Confirm that body is updated
		response.Body = ioutil.NopCloser(bytes.NewBuffer(updatedResponseBody))
	}

	return response, responseErr
}

// NewRequestService will initialize a new request service with the passed values
func NewRequestService(endpoint string, method string, body []byte, zc *ZincClient) *RequestService {
	return &RequestService{
		Endpoint:        endpoint,
		Method:          method,
		Body:            body,
		internalHeaders: nil,
		clientToUse:     zc,
	}
}

// Headers will allow adding headers to the request
func (rs *RequestService) Headers(headers *http.Header) *RequestService {
	rs.internalHeaders = headers
	return rs
}

// Index will return an IndexService object with the passed details
func (zc *ZincClient) Index(endpoint string, method string, body []byte) *IndexService {
	// Create a new index service object
	newIndexService := IndexService{
		RequestService: *NewRequestService(endpoint, method, body, zc),
	}

	return &newIndexService
}

// Headers will add the headers to the index service
func (is *IndexService) Headers(headers *http.Header) *IndexService {
	is.RequestService = *is.RequestService.Headers(headers)
	return is
}

// Do will make the request to Zinc and return a response accordingly
func (is *IndexService) Do(ctx context.Context) (*http.Response, error) {
	bodyToUse := is.Body
	if MultiTenant {
		// Add the `tenantID` to the request body
		updatedBody, updateErr := AddTenantID(is.Body, ctx)
		if updateErr != nil {
			errMsg := fmt.Sprint("error while adding tenant_id to passed body: ", updateErr.Error())
			log.Warnln(zincTag, ": ", errMsg)
			return nil, updateErr
		}
		bodyToUse = updatedBody
	}

	return is.clientToUse.MakeRequest(is.Endpoint, is.Method, bodyToUse, is.internalHeaders, ctx)
}

// Bulk will return a BulkService object with the passed details
func (zc *ZincClient) Bulk(endpoint string, method string, body []byte) *BulkService {
	return &BulkService{
		RequestService: *NewRequestService(endpoint, method, body, zc),
	}
}

// Headers will add the passed headers to the bulk request
func (bs *BulkService) Headers(headers *http.Header) *BulkService {
	bs.RequestService = *bs.RequestService.Headers(headers)
	return bs
}

// Search will return a SearchService object with the passed details
func (zc *ZincClient) Search(endpoint string, method string, body []byte) *SearchService {
	return &SearchService{
		RequestService: *NewRequestService(endpoint, method, body, zc),
	}
}

// Headers will add the passed headers to the request body
func (ss *SearchService) Headers(headers *http.Header) *SearchService {
	ss.RequestService = *ss.RequestService.Headers(headers)
	return ss
}

// Do will make the search request and return the search response
func (ss *SearchService) Do(ctx context.Context) (*http.Response, error) {
	bodyToUse := ss.Body
	if MultiTenant {
		updatedBody, updateErr := addTenantIdFilterQuery(ss.Body, ctx)
		if updateErr != nil {
			return nil, updateErr
		}
		bodyToUse = updatedBody
	}

	return ss.clientToUse.MakeRequest(ss.Endpoint, ss.Method, bodyToUse, ss.internalHeaders, ctx)
}

// Update will return a UpdateService object with the passed details
func (zc *ZincClient) Update(endpoint string, method string, body []byte) *UpdateService {
	return &UpdateService{
		RequestService: *NewRequestService(endpoint, method, body, zc),
	}
}

// Headers will add the passed headers to the request body
func (us *UpdateService) Headers(headers *http.Header) *UpdateService {
	us.RequestService = *us.RequestService.Headers(headers)
	return us
}

// Do will make the update request and return the response
func (us *UpdateService) Do(ctx context.Context) (*http.Response, error) {
	return us.clientToUse.MakeRequest(us.Endpoint, us.Method, us.Body, us.internalHeaders, ctx)
}

// NewClient instantiates the Zinc Client
func NewZincClient() {
	zincClientInit.Do(func() {
		// Initialize the zinc client
		initZincClient()

		log.Println("zinc client instantiated")
	})
}
