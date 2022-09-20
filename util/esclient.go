package util

import (
	"encoding/base64"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"

	v "github.com/hashicorp/go-version"
	es7 "github.com/olivere/elastic/v7"
	log "github.com/sirupsen/logrus"
	es6 "gopkg.in/olivere/elastic.v6"
)

var version int
var semanticVersion string

var (
	clientInit sync.Once
	// External ES client
	client7         *es7.Client
	internalClient7 *es7.Client
	client6         *es6.Client
)

// GetClient7 returns the es7 client
func GetClient7() *es7.Client {
	// initialize the client if not present
	if client7 == nil {
		initClient7()
	}
	return client7
}

// GetInternalClient7 returns the es7 client
func GetInternalClient7() *es7.Client {
	// initialize the client if not present

	// If external then return user client instead
	if ExternalElasticsearch == "true" {
		return client7
	}

	if internalClient7 == nil {
		initClient7()
	}
	return internalClient7
}

// GetClient6 returns the es6 client
func GetClient6() *es6.Client {
	// initialize the client if not present
	if client6 == nil {
		initClient6()
	}
	return client6
}

// GetSearchClientESURL returns the ES URL used by SLS and non-SSL plugins
// Plugins calling the external ES must use search client URL
func GetSearchClientESURL() string {
	if IsSLSDisabled() {
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
	searchBackend := GetBackend()
	if searchBackend != nil {
		var esURL string
		var esHeader string
		if *searchBackend == ElasticSearch {
			esURL = GetGlobalESURL()
			if esURL == "" {
				log.Fatal("Error encountered: ", fmt.Errorf("ES_URL must be set in the global environment variables"))
			}
			esHeader = GetGlobalESHeader()
		} else if *searchBackend == OpenSearch {
			esURL = GetGlobalOSURL()
			if esURL == "" {
				log.Fatal("Error encountered: ", fmt.Errorf("OS_URL must be set in the global environment variables"))
			}
			esHeader = GetGlobalOSHeader()
		}
		if esURL != "" {
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
			} else {
				if esHeader != "" {
					authHeader, err := base64.StdEncoding.DecodeString(esHeader)
					if err != nil {
						log.Fatal("Error encountered: ", fmt.Errorf("ES_HEADER must be set in base64 format"))
					}
					protocolWithCredentials := strings.Split(esURL, "://")
					host := protocolWithCredentials[1]
					protocol := protocolWithCredentials[0]

					credentials := string(authHeader)
					credentialSeparator := strings.Index(credentials, ":")
					username := credentials[0:credentialSeparator]
					password := credentials[credentialSeparator+1:]
					esURL = protocol + "://" + url.PathEscape(username) + ":" + url.PathEscape(password) + "@" + host
				}
			}
			return esURL
		}
	}
	return ""
}

// GetESURL returns elasticsearch url with escaped auth
func GetESURL() string {
	if IsSLSDisabled() {
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
	return GetInternalESURL()
}

// GetInternalESURL returns elasticsearch url for internal use
func GetInternalESURL() string {
	esURL := ""

	if IsSLSEnabled() {
		esURL = fmt.Sprint(ACCAPI, "es")
		return esURL
	}

	return esURL
}

// GetVersion returns the es version
func GetVersion() int {
	// Get the version if not present
	if version == 0 {
		esVersion, err := client7.ElasticsearchVersion(GetSearchClientESURL())
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
		esVersion, err := client7.ElasticsearchVersion(GetSearchClientESURL())
		if err != nil {
			log.Fatal("Error encountered: ", fmt.Errorf("error while retrieving the elastic version: %v", err))
		} else {
			semanticVersion = esVersion
		}
	}
	return semanticVersion
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

func initClient6() {
	var err error

	loggerT := log.New()
	wrappedLoggerDebug := &WrapKitLoggerDebug{*loggerT}
	wrappedLoggerError := &WrapKitLoggerError{*loggerT}

	esHttpClient := HTTPClient()

	if ExternalElasticsearch != "true" {
		esHttpClient.Transport = &CustomESTransport{originalTransport: esHttpClient.Transport}
	}

	// Initialize the ES v6 client
	client6, err = es6.NewClient(
		es6.SetURL(GetESURL()),
		es6.SetRetrier(NewRetrier()),
		es6.SetSniff(isSniffingEnabled()),
		es6.SetHttpClient(esHttpClient),
		es6.SetErrorLog(wrappedLoggerError),
		es6.SetInfoLog(wrappedLoggerDebug),
		es6.SetTraceLog(wrappedLoggerDebug),
	)

	if err != nil {
		log.Fatal("Error encountered: ", fmt.Errorf("error while initializing elastic v6 client: %v", err))
	}
}

func initClient7() {
	var err error
	// Initialize the ES v7 client

	loggerT := log.New()
	wrappedLoggerDebug := &WrapKitLoggerDebug{*loggerT}
	wrappedLoggerError := &WrapKitLoggerError{*loggerT}

	esHttpClient := HTTPClient()
	if IsSLSEnabled() {
		internalEsHttpClient := HTTPClient()
		internalEsHttpClient.Transport = &CustomESTransport{originalTransport: internalEsHttpClient.Transport}
		internalClient7, err = es7.NewClient(
			es7.SetURL(GetInternalESURL()),
			es7.SetRetrier(NewRetrier()),
			es7.SetSniff(isSniffingEnabled()),
			es7.SetHttpClient(internalEsHttpClient),
			es7.SetErrorLog(wrappedLoggerError),
			es7.SetInfoLog(wrappedLoggerDebug),
			es7.SetTraceLog(wrappedLoggerDebug),
		)

		if err != nil {
			log.Fatal("Error encountered while initializing internal ES client: ", err)
		}
		// init search client if BE is OpenSearch or Elasticsearch
		backend := GetBackend()
		if backend != nil {
			if *backend == ElasticSearch || *backend == OpenSearch {
				internalEsHttpClient.Transport = &CustomESTransport{originalTransport: internalEsHttpClient.Transport}
				client7, err = es7.NewClient(
					es7.SetURL(GetSearchClientESURL()),
					es7.SetRetrier(NewRetrier()),
					es7.SetSniff(isSniffingEnabled()),
					es7.SetHttpClient(internalEsHttpClient),
					es7.SetErrorLog(wrappedLoggerError),
					es7.SetInfoLog(wrappedLoggerDebug),
					es7.SetTraceLog(wrappedLoggerDebug),
				)

				if err != nil {
					log.Fatal("Error encountered while initializing internal ES client: ", err)
				}
			}
		} else {
			log.Fatal("Error while checking backend from passed `APPBASE_ID`: not present")
		}
	} else {
		client7, err = es7.NewClient(
			es7.SetURL(GetESURL()),
			es7.SetRetrier(NewRetrier()),
			es7.SetSniff(isSniffingEnabled()),
			es7.SetHttpClient(esHttpClient),
			es7.SetErrorLog(wrappedLoggerError),
			es7.SetInfoLog(wrappedLoggerDebug),
			es7.SetTraceLog(wrappedLoggerDebug),
		)
		if err != nil {
			log.Fatal("Error encountered: ", fmt.Errorf("error while initializing elastic v7 client: %v", err))
		}
	}
}

// NewClient instantiates the ES v6 and v7 clients
func NewClient() {
	clientInit.Do(func() {
		// Initialize the ES v7 client
		initClient7()
		// Initialize the ES v6 client
		// initClient6()
		// Get the ES version
		GetVersion()

		log.Println("clients instantiated, elastic search version is", version)
	})
}

// CreateCustomTransport will create a custom transport
// that will append a new header to all ES requests.
//
// This should only be used if External ES is false.
func CreateCustomTransport() {

}
