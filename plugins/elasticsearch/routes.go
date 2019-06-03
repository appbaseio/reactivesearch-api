package elasticsearch

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"path/filepath"
	"strings"
	"sync"

	"github.com/appbaseio/arc/middleware"
	"github.com/appbaseio/arc/plugins"
	"github.com/appbaseio/arc/model/acl"
	"github.com/appbaseio/arc/model/category"
	"github.com/appbaseio/arc/model/op"
	"github.com/appbaseio/arc/util"
	"github.com/gobuffalo/packr"
)

var (
	routes     []plugins.Route
	routeSpecs = make(map[string]api)
	acls       = make(map[category.Category]map[acl.ACL]bool)
)

type api struct {
	name     string
	category category.Category
	acl      acl.ACL
	op       op.Operation
	spec     *spec
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

func (es *elasticsearch) preprocess(mw [] middleware.Middleware) error {
	files := make(chan string)
	apis := make(chan api)

	box := packr.NewBox("./api")

	go fetchSpecFiles(&box, files)
	go decodeSpecFiles(&box, files, apis)

	middlewareFunction := (&chain{}).Wrap

	for api := range apis {
		for _, path := range api.spec.URL.Paths {
			if !strings.HasPrefix(path, "/") {
				path = "/" + path
			}
			if path == "/" {
				continue
			}
			r := plugins.Route{
				Name:        api.name,
				Methods:     api.spec.Methods,
				Path:        path,
				HandlerFunc: middlewareFunction(mw, es.handler()),
				Description: api.spec.Documentation,
			}
			routes = append(routes, r)
			for _, method := range api.spec.Methods {
				key := fmt.Sprintf("%s:%s", method, path)
				routeSpecs[key] = api
			}
		}
		if _, ok := acls[api.category]; !ok {
			acls[api.category] = make(map[acl.ACL]bool)
		}
		if _, ok := acls[api.category][api.acl]; !ok {
			acls[api.category][api.acl] = true
		}
	}

	// sort the routes
	criteria := func(r1, r2 plugins.Route) bool {
		f1, c1 := util.CountComponents(r1.Path)
		f2, c2 := util.CountComponents(r2.Path)
		if f1 == f2 {
			return c1 < c2
		}
		return f1 > f2
	}
	plugins.RouteBy(criteria).RouteSort(routes)

	// append index route last in order to avoid early matches for other specific routes
	indexRoute := plugins.Route{
		Name:        "ping",
		Methods:     []string{http.MethodGet},
		Path:        "/",
		HandlerFunc: middlewareFunction(mw, es.handler()),
		Description: "You know, for search",
	}
	routes = append(routes, indexRoute)

	return nil
}

func (es *elasticsearch) routes() []plugins.Route {
	return routes
}

func fetchSpecFiles(box *packr.Box, files chan<- string) {
	defer close(files)
	for _, file := range box.List() {
		if filepath.Ext(file) == ".json" && !strings.HasPrefix(file, "_") {
			files <- file
		}
	}
}

func decodeSpecFiles(box *packr.Box, files <-chan string, apis chan<- api) {
	var wg sync.WaitGroup
	for file := range files {
		wg.Add(1)
		go decodeSpecFile(box, file, &wg, apis)
	}

	go func() {
		wg.Wait()
		close(apis)
	}()
}

func decodeSpecFile(box *packr.Box, file string, wg *sync.WaitGroup, apis chan<- api) {
	defer wg.Done()

	content, err := box.Find(file)
	if err != nil {
		log.Printf("can't read file: %v", err)
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

	specName := strings.TrimSuffix(filepath.Base(file), ".json")
	specCategory := decodeCategory(&s)
	specOp := decodeOp(&s)
	specACL, err := decodeACL(specName, &s)
	if err != nil {
		log.Printf(`%s: unable to categorize spec "%s": %v\n`, logTag, specName, err)
	}

	apis <- api{
		name:     specName,
		category: specCategory,
		op:       specOp,
		acl:      *specACL,
		spec:     &s,
	}
}

func decodeCategory(spec *spec) category.Category {
	docTokens := strings.Split(spec.Documentation, "/")
	tag := strings.TrimSuffix(docTokens[len(docTokens)-1], ".html")
	tagTokens := strings.Split(tag, "-")
	tagName := tagTokens[0]
	return category.FromString(tagName)
}

func decodeACL(specName string, spec *spec) (*acl.ACL, error) {
	pathTokens := strings.Split(spec.URL.Path, "/")
	for _, pathToken := range pathTokens {
		if strings.HasPrefix(pathToken, "_") {
			pathToken = strings.TrimPrefix(pathToken, "_")
			c, err := acl.FromString(pathToken)
			if err != nil {
				return nil, err
			}
			return &c, nil
		}
	}

	aclString := strings.Split(specName, ".")[0]
	a, err := acl.FromString(aclString)
	if err != nil {
		defaultACL := acl.Get
		return &defaultACL, err
	}

	return &a, nil
}

func decodeOp(spec *spec) op.Operation {
	var specOp op.Operation
	methods := spec.Methods

out:
	for _, method := range methods {
		switch method {
		case http.MethodPut:
			specOp = op.Write
			break out
		case http.MethodPatch:
			specOp = op.Write
			break out
		case http.MethodDelete:
			specOp = op.Delete
			break out
		case http.MethodGet:
			specOp = op.Read
			break out
		case http.MethodHead:
			specOp = op.Read
			break out
		case http.MethodPost:
			specOp = op.Write
		default:
			specOp = op.Read
			break out
		}
	}

	return specOp
}

func printCategoryACLMDTable() {
	fmt.Printf("| **Category** | **ACLs** |\n")
	fmt.Printf("|----------|------|\n")
	for c, a := range acls {
		fmt.Printf("| `%s` | ", c)
		fmt.Printf("<ul>")
		for k := range a {
			fmt.Printf("<li>`%s`</li>", k)
		}
		fmt.Printf("</ul> |")
		fmt.Printf("\n")
	}
}
