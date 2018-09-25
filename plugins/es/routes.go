package es

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/appbaseio-confidential/arc/arc/plugin"
	"github.com/appbaseio-confidential/arc/middleware/interceptor"
)

type api struct {
	name string
	spec spec
}

type spec struct {
	Documentation string   `json:"documentation"`
	Methods       []string `json:"methods"`
	URL           struct {
		Path   string      `json:"path"`
		Paths  []string    `json:"paths,omitempty"`
		Parts  interface{} `json:"parts,omitempty"`
		Params interface{} `json:"params,omitempty"`
	} `json:"url"`
	Body struct {
		Description string `json:"description"`
		Required    bool   `json:"required,omitempty"`
		Serialize   string `json:"serialize,omitempty"`
	} `json:"body,omitempty"`
}

func (es *ES) routes() []plugin.Route {
	var i = interceptor.New()

	files := make(chan string)
	apis := make(chan api)

	path, err := apiDirPath()
	if err != nil {
		log.Printf("%s: unable to fetch api dir path: %v", logTag, err)
		return nil
	}

	go fetchSpecFiles(path, files)
	go decodeSpecFiles(files, apis)

	var routes []plugin.Route
	for api := range apis {
		name := strings.TrimSuffix(api.name, ".json")
		methods := api.spec.Methods
		description := api.spec.Documentation
		for _, path := range api.spec.URL.Paths {
			if !strings.HasPrefix(path, "/") {
				path = "/" + path
			}
			if len(path) == 1 {
				continue
			}
			route := plugin.Route{
				Name:        name,
				Methods:     methods,
				Path:        path,
				HandlerFunc: i.Wrap(es.redirectHandler),
				Description: description,
			}
			routes = append(routes, route)
		}
	}

	// append the index route last in order to avoid early
	// matches for other specific routes
	indexRoute := plugin.Route{
		Name:        "ping",
		Methods:     []string{http.MethodGet},
		Path:        "/",
		HandlerFunc: i.Wrap(es.redirectHandler),
		Description: "You know, for search",
	}
	routes = append(routes, indexRoute)
	return routes
}

func apiDirPath() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", nil
	}
	return filepath.Join(wd, "plugins/es/api"), nil
}

func fetchSpecFiles(path string, files chan<- string) {
	defer close(files)

	info, err := os.Stat(path)
	if err != nil {
		log.Fatal(err)
		return
	}
	if !info.IsDir() {
		log.Printf("%s: cannot walk through a file path", logTag)
		return
	}
	err = filepath.Walk(path, func(path string, info os.FileInfo, err error) error {
		if !info.IsDir() && filepath.Ext(path) == ".json" && !strings.HasPrefix(info.Name(), "_") {
			files <- path
		}
		return nil
	})
	if err != nil {
		log.Fatal(err)
		return
	}
}

func decodeSpecFiles(files <-chan string, apis chan<- api) {
	var wg sync.WaitGroup
	for file := range files {
		wg.Add(1)
		go func(file string, wg *sync.WaitGroup) {
			defer wg.Done()
			content, err := ioutil.ReadFile(file)
			if err != nil {
				log.Fatal(err)
				return
			}
			decoder := json.NewDecoder(bytes.NewReader(content))
			_, err = decoder.Token() // skip opening braces
			if err != nil {
				log.Fatal(err)
				return
			}
			_, err = decoder.Token() // skip object name
			if err != nil {
				log.Fatal(err)
				return
			}
			var s spec
			err = decoder.Decode(&s)
			if err != nil {
				log.Fatal(err)
				return
			}
			apis <- api{filepath.Base(file), s}
		}(file, &wg)
	}
	wg.Wait()
	close(apis)
}
