package util

import (
	"fmt"
	"sync"

	"github.com/olivere/elastic/v7"
	es6 "gopkg.in/olivere/elastic.v6"
)

var (
	clientInit sync.Once
	client7    *elastic.Client
	client6    *es6.Client
)

// NewClient instantiates the ES v6 and v7 clients
func NewClient(url string) {
	clientInit.Do(func() {
		// Initialize the ES v7 client
		var err error
		client7, err = elastic.NewClient(
			elastic.SetURL(url),
			elastic.SetRetrier(NewRetrier()),
			elastic.SetSniff(false),
			elastic.SetHttpClient(HTTPClient()),
		)
		if err != nil {
			fmt.Println("Error encountered: ", fmt.Errorf("error while initializing elastic v7 client: %v", err))
		}

		// Initialize the ES v6 client
		client6, err = es6.NewClient(
			es6.SetURL(url),
			es6.SetRetrier(NewRetrier()),
			es6.SetSniff(false),
			es6.SetHttpClient(HTTPClient()),
		)
		if err != nil {
			fmt.Println("Error encountered: ", fmt.Errorf("error while initializing elastic v6 client: %v", err))
		}
		fmt.Println("clients instantiated")
	})
}
