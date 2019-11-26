package util

import (
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"

	es7 "github.com/olivere/elastic/v7"
	es6 "gopkg.in/olivere/elastic.v6"
)

// EsVersion is a list of supported es versions
type EsVersion int

// ES_VERSION represents the current es version to be used
var VERSION EsVersion

const (
	ES6 EsVersion = iota
	ES7
)

// String is the implementation of Stringer interface that returns the string representation of Plan type.
func (o EsVersion) String() string {
	return [...]string{
		"Es6",
		"Es7",
	}[o]
}

var (
	clientInit          sync.Once
	Client7             *es7.Client
	Client6             *es6.Client
	IsCurrentVersionES6 bool
)

// NewClient instantiates the ES v6 and v7 clients
func NewClient(url string) (*es6.Client, *es7.Client) {
	clientInit.Do(func() {
		var err error
		// Initialize the ES v7 client
		Client7, err = es7.NewClient(
			es7.SetURL(url),
			es7.SetRetrier(NewRetrier()),
			es7.SetSniff(false),
			es7.SetHttpClient(HTTPClient()),
		)
		// Get the ES version
		version, err := Client7.ElasticsearchVersion(url)
		var splitStr = strings.Split(version, ".")
		if len(splitStr) > 0 && splitStr[0] != "" {
			majorVersion, _ := strconv.Atoi(splitStr[0])
			// set the version
			if majorVersion == 6 {
				VERSION = ES6
			} else {
				VERSION = ES7
			}
			if err != nil {
				log.Println("Error encountered: ", fmt.Errorf("error while calculating the elastic version: %v", err))
			}
		}
		if err != nil {
			log.Fatal("Error encountered: ", fmt.Errorf("error while retrieving the elastic version: %v", err))
		}
		IsCurrentVersionES6 = (VERSION == ES6)
		if err != nil {
			log.Fatal("Error encountered: ", fmt.Errorf("error while initializing elastic v7 client: %v", err))
		}
		// Initialize the ES v6 client
		Client6, err = es6.NewClient(
			es6.SetURL(url),
			es6.SetRetrier(NewRetrier()),
			es6.SetSniff(false),
			es6.SetHttpClient(HTTPClient()),
		)
		if err != nil {
			log.Fatal("Error encountered: ", fmt.Errorf("error while initializing elastic v6 client: %v", err))
		}
		fmt.Println("clients instantiated, elastic search version is", VERSION)
	})
	return Client6, Client7
}
