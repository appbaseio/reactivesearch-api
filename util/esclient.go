package util

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"

	v "github.com/hashicorp/go-version"
	es7 "github.com/olivere/elastic/v7"
	log "github.com/sirupsen/logrus"
)

var version int
var semanticVersion string

type ClusterType int

const (
	ElasticSearch ClusterType = iota
	OpenSearch
)

func (c ClusterType) String() string {
	switch c {
	case ElasticSearch:
		return "elasticsearch"
	case OpenSearch:
		return "opensearch"
	}

	return ""
}

var clusterType *ClusterType = nil

var (
	clientInit sync.Once
	client7    *es7.Client
)

// GetClient7 returns the es7 client
func GetClient7() *es7.Client {
	// initialize the client if not present
	if client7 == nil {
		initClient7()
	}
	return client7
}

// GetESURL returns elasticsearch url with escaped auth
func GetESURL() string {
	esURL := os.Getenv("ES_CLUSTER_URL")

	if esURL == "" {
		log.Fatal("Error encountered: ", fmt.Errorf("ES_CLUSTER_URL must be set in the environment variables"))
	}

	if strings.Contains(esURL, "@") {
		splitIndex := strings.LastIndex(esURL, "@")
		protocolWithCredentials := strings.Split(esURL[0:splitIndex], "://")
		credentials := protocolWithCredentials[1]
		protocol := protocolWithCredentials[0]
		host := esURL[splitIndex+1:]

		credentialSeparator := strings.Index(credentials, ":")
		username := credentials[0:credentialSeparator]
		password := credentials[credentialSeparator+1:]
		esURL = protocol + "://" + url.PathEscape(username) + ":" + url.PathEscape(password) + "@" + host
	}
	return esURL
}

// GetVersion returns the es version
func GetVersion() int {
	// Get the version if not present
	if version == 0 {
		esVersion, err := client7.ElasticsearchVersion(GetESURL())
		if err != nil {
			log.Fatal("Error encountered: ", fmt.Errorf("error while retrieving the elastic version: %v", err))
		}
		var splitStr = strings.Split(esVersion, ".")
		if len(splitStr) > 0 && splitStr[0] != "" {
			version, _ = strconv.Atoi(splitStr[0])
			if err != nil {
				log.Errorln("Error encountered: error while calculating the elastic version", err)
			}
		}
	}
	return version
}

// GetSemanticVersion returns the es version
func GetSemanticVersion() string {
	// Get the version if not present
	if semanticVersion == "" {
		esVersion, err := client7.ElasticsearchVersion(GetESURL())
		if err != nil {
			log.Fatal("Error encountered: ", fmt.Errorf("error while retrieving the elastic version: %v", err))
		} else {
			semanticVersion = esVersion
		}
	}
	return semanticVersion
}

// GetClusterType will determine whether the cluster is an OpenSearch
// or an ElasticSearch cluster.
func GetClusterType() *ClusterType {
	if clusterType == nil {
		requestOptions := es7.PerformRequestOptions{
			Method: "GET",
			Path:   "/",
		}
		response, statErr := client7.PerformRequest(context.Background(), requestOptions)
		if statErr != nil {
			log.Fatal("Error encountered: ", fmt.Errorf("error while retrieving the cluster type: %s", statErr.Error()))
		} else {
			if response.StatusCode != http.StatusOK {
				log.Fatal("Error encountered: ", fmt.Errorf("error while retrieving the cluster type, got non OK status: %d", response.StatusCode))
			}

			// Read the body and then read the tagline
			statusMap := make(map[string]interface{})
			unmarshalErr := json.Unmarshal(response.Body, &statusMap)
			if unmarshalErr != nil {
				log.Fatal("Error encountered: ", fmt.Errorf("error while retrieving the cluster type, error while unmarshalling: %s", unmarshalErr.Error()))
			}

			// Parse the tagline
			tagLine, taglinePresent := statusMap["tagline"]
			if !taglinePresent {
				log.Fatal("Error encountered: ", fmt.Errorf("error while retrieving the cluster type, no tagline present!"))
			}

			taglineAsStr, asStrOk := tagLine.(string)
			if !asStrOk {
				log.Fatal("Error encountered: ", fmt.Errorf("error while retrieving the cluster type, tagline is not string!"))
			}

			clusterTypeRead := ElasticSearch
			if strings.Contains(strings.ToLower(taglineAsStr), "opensearch") {
				clusterTypeRead = OpenSearch
			}

			clusterType = &clusterTypeRead
		}
	}

	return clusterType
}

// HiddenIndexSettings to set plugin indices as hidden index
func HiddenIndexSettings() string {
	esVersion, _ := v.NewVersion(GetSemanticVersion())
	hiddenIndexVersion, _ := v.NewVersion("7.7.0")
	if esVersion.GreaterThanOrEqual(hiddenIndexVersion) {
		return `"index.hidden": true,`
	}

	return ""
}

func isSniffingEnabled() bool {
	setSniffing := os.Getenv("SET_SNIFFING")
	sniffing := false
	if setSniffing == "true" {
		sniffing = true
	}
	return sniffing
}

func initClient7() {
	var err error
	// Initialize the ES v7 client

	loggerT := log.New()
	wrappedLoggerDebug := &WrapKitLoggerDebug{*loggerT}
	wrappedLoggerError := &WrapKitLoggerError{*loggerT}

	client7, err = es7.NewClient(
		es7.SetURL(GetESURL()),
		es7.SetRetrier(NewRetrier()),
		es7.SetSniff(isSniffingEnabled()),
		es7.SetHealthcheck(false),
		es7.SetHttpClient(HTTPClient()),
		es7.SetErrorLog(wrappedLoggerError),
		es7.SetInfoLog(wrappedLoggerDebug),
		es7.SetTraceLog(wrappedLoggerDebug),
	)
	if err != nil {
		log.Fatal("Error encountered: ", fmt.Errorf("error while initializing elastic v7 client: %v", err))
	}
}

// NewClient instantiates the ES v6 and v7 clients
func NewClient() {
	clientInit.Do(func() {
		// Initialize the ES v7 client
		initClient7()
		// Get the ES version
		GetVersion()

		log.Println("clients instantiated, elastic search version is", version)
	})
}
