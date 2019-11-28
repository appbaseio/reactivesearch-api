package util

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"

	es7 "github.com/olivere/elastic/v7"
	es6 "gopkg.in/olivere/elastic.v6"
)

var version int
var isTestingMode bool

var (
	clientInit sync.Once
	client7    *es7.Client
	client6    *es6.Client
)

// GetClient7 returns the es7 client
func GetClient7() *es7.Client {
	// initialize the client if not present
	if client7 == nil {
		clientInit.Do(func() {
			initClient7()
		})
	}
	return client7
}

// GetClient6 returns the es6 client
func GetClient6() *es6.Client {
	if client6 == nil {
		clientInit.Do(func() {
			initClient6()
		})
	}
	return client6
}

// GetVersion returns the es version
func GetVersion() int {
	if isTestingMode {
		// set the default es version for testing
		version = 7
	}
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
				log.Println("Error encountered: ", fmt.Errorf("error while calculating the elastic version: %v", err))
			}
		}
	}
	return version
}

func getURL() string {
	url := os.Getenv("ES_CLUSTER_URL")
	if url == "" {
		log.Fatal("Error encountered: ", fmt.Errorf("ES_CLUSTER_URL must be set in the environment variables"))
	}
	return url
}

// EnableTestMode enables the testing mode
func EnableTestMode() {
	isTestingMode = true
}

func initClient6() {
	var err error
	// Initialize the ES v6 client
	if isTestingMode {
		client6, err = es6.NewSimpleClient(
			es6.SetURL(getURL()),
			// ES LOGS: uncomment to see the elasticsearch query logs
			// es6.SetInfoLog(log.New(os.Stdout, "", log.LstdFlags)),
			// es6.SetTraceLog(log.New(os.Stderr, "[[ELASTIC]]", 0)),
		)
	} else {
		client6, err = es6.NewClient(
			es6.SetURL(getURL()),
			es6.SetRetrier(NewRetrier()),
			es6.SetSniff(false),
			es6.SetHttpClient(HTTPClient()),
			// ES LOGS: uncomment to see the elasticsearch query logs
			// es6.SetInfoLog(log.New(os.Stdout, "", log.LstdFlags)),
			// es6.SetTraceLog(log.New(os.Stderr, "[[ELASTIC]]", 0)),
		)
	}

	if err != nil {
		log.Fatal("Error encountered: ", fmt.Errorf("error while initializing elastic v6 client: %v", err))
	}
}

func initClient7() {
	var err error
	// Initialize the ES v7 client
	if isTestingMode {
		client7, err = es7.NewSimpleClient(
			es7.SetURL(getURL()),
			// ES LOGS: uncomment to see the elasticsearch query logs
			// es7.SetInfoLog(log.New(os.Stdout, "", log.LstdFlags)),
			// es7.SetTraceLog(log.New(os.Stderr, "[[ELASTIC]]", 0)),
		)
	} else {
		client7, err = es7.NewClient(
			es7.SetURL(getURL()),
			es7.SetRetrier(NewRetrier()),
			es7.SetSniff(false),
			es7.SetHttpClient(HTTPClient()),
			// ES LOGS: uncomment to see the elasticsearch query logs
			// es7.SetInfoLog(log.New(os.Stdout, "", log.LstdFlags)),
			// es7.SetTraceLog(log.New(os.Stderr, "[[ELASTIC]]", 0)),
		)
	}
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

		fmt.Println("clients instantiated, elastic search version is", version)
	})
}
