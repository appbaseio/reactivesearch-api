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
	"github.com/appbaseio-confidential/arc/internal/types/acl"
	"github.com/appbaseio-confidential/arc/internal/util"
	"github.com/appbaseio-confidential/arc/middleware/interceptor"
	"github.com/appbaseio-confidential/arc/middleware/logger"
	"github.com/appbaseio-confidential/arc/plugins/analytics"
	"github.com/appbaseio-confidential/arc/plugins/auth"
)

const esSpecsDir = "plugins/es/api"

type api struct {
	name        string
	acl         acl.ACL
	keywords    map[string]bool
	spec        spec
	regexps     []string
	pathRegexps map[string]string
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

// TODO: major refactoring
func (es *es) routes() []plugin.Route {
	// fetch es api
	files := make(chan string)
	apis := make(chan api)

	path, err := apiDirPath()
	if err != nil {
		log.Printf("%s: unable to fetch api dir path: %v", logTag, err)
		return nil
	}

	go fetchSpecFiles(path, files)
	go decodeSpecFiles(files, apis)

	// init the necessary middleware
	var (
		redirectRequest   = interceptor.Instance().Intercept
		basicAuth         = auth.Instance().BasicAuth
		reqLogger         = logger.Instance().Log
		classifier        = es.classifier
		analyticsRecorder = analytics.Instance().Recorder
		//ratelimit       = ratelimiter.New().RateLimit
	)

	// TODO: validate permission for index being accessed
	// TODO: chain common middleware
	// handler
	var handlerFunc = reqLogger(classifier(basicAuth(validateOp(validateACL(redirectRequest(analyticsRecorder(es.handler())))))))

	// accumulate the routes
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
				HandlerFunc: handlerFunc,
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
		HandlerFunc: handlerFunc,
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
	return filepath.Join(wd, esSpecsDir), nil
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
		if !info.IsDir() && filepath.Ext(path) == ".json" &&
			!strings.HasPrefix(info.Name(), "_") {
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
	a := getACL(s)
	keywords := getKeywords(s.URL.Paths)
	regexps := getRegexps(s.URL.Paths)
	paths := getPaths(s.URL.Paths)
	apis <- api{
		name:        name,
		spec:        s,
		acl:         a,
		keywords:    keywords,
		regexps:     regexps,
		pathRegexps: paths,
	}
}

func getPaths(paths []string) map[string]string {
	pathRegexps := make(map[string]string)
	for _, path := range paths {
		regexp := replaceVars(path)
		pathRegexps[path] = regexp
	}
	return pathRegexps
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
			vars[i] = varRegexps[v]
		}
	}
	return "^" + strings.Join(vars, "/") + "(\\?.*)?$"
}

func getACL(s spec) acl.ACL {
	docTokens := strings.Split(s.Documentation, "/")
	tag := strings.TrimSuffix(docTokens[len(docTokens)-1], ".html")
	tagTokens := strings.Split(tag, "-")
	tagName := tagTokens[0]
	return categories[tagName]
}

func getKeywords(paths []string) map[string]bool {
	set := make(map[string]bool)
	for _, path := range paths {
		tokens := strings.Split(path, "/")
		for _, token := range tokens {
			if strings.HasPrefix(token, "_") && util.Contains(keywords, token) {
				if _, ok := set[token]; !ok {
					set[token] = true
				}
			}
		}
	}
	return set
}
