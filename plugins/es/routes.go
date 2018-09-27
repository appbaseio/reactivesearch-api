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
	"github.com/appbaseio-confidential/arc/internal/types/category"
)

type api struct {
	name     string
	category category.Category
	spec     spec
	regexps  []string
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
	files := make(chan string)
	apis := make(chan api)

	path, err := apiDirPath()
	if err != nil {
		log.Printf("%s: unable to fetch api dir path: %v", logTag, err)
		return nil
	}

	go fetchSpecFiles(path, files)
	go decodeSpecFiles(files, apis)

	// init the middleware
	//var i = interceptor.New()

	var routes []plugin.Route
	for api := range apis {
		for _, path := range api.spec.URL.Paths {
			if !strings.HasPrefix(path, "/") {
				path = "/" + path
			}
			if len(path) == 1 {
				continue
			}
			route := plugin.Route{
				Name:        api.name,
				Methods:     api.spec.Methods,
				Path:        path,
				HandlerFunc: es.classify(es.redirectHandler()),
				Description: api.spec.Documentation,
			}
			routes = append(routes, route)
		}
		es.specs = append(es.specs, api)
	}

	// append the index route last in order to avoid early
	// matches for other specific routes
	indexRoute := plugin.Route{
		Name:        "ping",
		Methods:     []string{http.MethodGet},
		Path:        "/",
		HandlerFunc: es.classify(es.redirectHandler()),
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
		go decodeSpec(file, &wg, apis)
	}
	wg.Wait()
	close(apis)
}

func decodeSpec(file string, wg *sync.WaitGroup, apis chan<- api) {
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

	name := strings.TrimSuffix(filepath.Base(file), ".json")
	c := getCategory(s)
	regexps := getRegexps(s.URL.Paths)

	apis <- api{
		name:     name,
		spec:     s,
		category: c,
		regexps:  regexps,
	}
}

func getRegexps(paths []string) []string {
	var regexps []string
	for _, path := range paths {
		path = replaceVars(path)
		regexps = append(regexps, path)
	}
	return regexps
}

func replaceVars(path string) string {
	vars := strings.Split(path, "/")
	for i, v := range vars {
		if strings.HasPrefix(v, "{") && strings.HasSuffix(v, "}") {
			regexp, ok := varRegexp[v]
			if !ok {
				log.Printf("%s: path var %s not found in the map.", logTag, v)
				continue
			}
			vars[i] = regexp
		}
	}
	return "^" + strings.Join(vars, "/") + "(\\?.*)?$"
}

func getCategory(s spec) category.Category {
	docTokens := strings.Split(s.Documentation, "/")
	tag := strings.TrimSuffix(docTokens[len(docTokens)-1], ".html")
	tagTokens := strings.Split(tag, "-")
	tagName := tagTokens[0]
	return categories[tagName]
}

var categories = map[string]category.Category{
	"docs":    category.Docs,
	"search":  category.Search,
	"indices": category.Indices,
	"cat":     category.Cat,

	"tasks":   category.Clusters,
	"cluster": category.Clusters,

	"ingest":   category.Misc,
	"snapshot": category.Misc,
	"modules":  category.Misc,
}

var varRegexp = map[string]string{
	"{index}": "[^_][^\\s/]*",
	"{type}":  "[^_][^\\s/]*",
	//"{type}":                 "[_]*[^\\s/]*",
	"{id}":                   "[^_][^\\s/]*",
	"{name}":                 "[^_][^\\s/]*",
	"{task_id}":              "[^_][^\\s/]*",
	"{scroll_id}":            "[^_][^\\s/]*",
	"{fields}":               "[^_][^\\s/]*",
	"{target}":               "[^_][^\\s/]*",
	"{metric}":               "[^_][^\\s/]*",
	"{alias}":                "[^_][^\\s/]*",
	"{new_index}":            "[^_][^\\s/]*",
	"{node_id}":              "[^_][^\\s/]*",
	"{repository}":           "[^_][^\\s/]*",
	"{thread_pool_patterns}": "[^_][^\\s/]*",
	"{index_metric}":         "[^_][^\\s/]*",
	"{context}":              "[^_][^\\s/]*",
	"{snapshot}":             "[^_][^\\s/]*",
}
