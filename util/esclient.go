package util

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"

	es7 "github.com/olivere/elastic/v7"
	log "github.com/sirupsen/logrus"
	es6 "gopkg.in/olivere/elastic.v6"
)

var version int
var semanticVersion string

var (
	clientInit sync.Once
	client7    *es7.Client
	client6    *es6.Client
)

// GetClient7 returns the es7 client
func GetClient7() *es7.Client {
	// initialize the client if not present
	if client7 == nil {
		initClient7()
	}
	return client7
}

// GetClient6 returns the es6 client
func GetClient6() *es6.Client {
	// initialize the client if not present
	if client6 == nil {
		initClient6()
	}
	return client6
}

// GetVersion returns the es version
func GetVersion() int {
	// Get the version if not present
	if version == 0 {
		esVersion, err := client7.ElasticsearchVersion(getURL())
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
		esVersion, err := client7.ElasticsearchVersion(getURL())
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
	esVersion := GetSemanticVersion()
	if strings.Contains(esVersion, "7.7") {
		return `"index.hidden": true,`
	}

	return ""
}

func getURL() string {
	url := os.Getenv("ES_CLUSTER_URL")
	if url == "" {
		log.Fatal("Error encountered: ", fmt.Errorf("ES_CLUSTER_URL must be set in the environment variables"))
	}
	return url
}

func initClient6() {
	var err error

	loggerT := log.New()
	wrappedLoggerDebug := &WrapKitLoggerDebug{*loggerT}
	wrappedLoggerError := &WrapKitLoggerError{*loggerT}

	// Initialize the ES v6 client
	client6, err = es6.NewClient(
		es6.SetURL(getURL()),
		es6.SetRetrier(NewRetrier()),
		es6.SetSniff(true),
		es6.SetHttpClient(HTTPClient()),
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

	client7, err = es7.NewClient(
		es7.SetURL(getURL()),
		es7.SetRetrier(NewRetrier()),
		es7.SetSniff(true),
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
		// Initialize the ES v6 client
		initClient6()
		// Get the ES version
		GetVersion()

		log.Println("clients instantiated, elastic search version is", version)
	})
}
