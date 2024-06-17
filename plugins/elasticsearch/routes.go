package elasticsearch

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"

	log "github.com/sirupsen/logrus"

	"github.com/appbaseio-confidential/reactivesearch/middleware"
	"github.com/appbaseio-confidential/reactivesearch/model/acl"
	"github.com/appbaseio-confidential/reactivesearch/model/category"
	"github.com/appbaseio-confidential/reactivesearch/model/op"
	"github.com/appbaseio-confidential/reactivesearch/plugins"
	"github.com/appbaseio-confidential/reactivesearch/plugins/auth"
	"github.com/appbaseio-confidential/reactivesearch/plugins/elasticsearch/static"
	"github.com/appbaseio-confidential/reactivesearch/util"
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

func (es *elasticsearch) preprocess(mw []middleware.Middleware) error {

	directoryName := "api"
	staticFiles, fileErr := static.AssetDir(directoryName)
	if fileErr != nil {
		return fmt.Errorf("error while fetching all files from the `%s` directory with err: %s", directoryName, fileErr.Error())
	}

	apis := make([]api, 0)

	// Iterate the files and open them one by one to read them
	for _, fileName := range staticFiles {
		// Open only files that are JSON and do not start with an underscore (_)
		if filepath.Ext(fileName) != ".json" || strings.HasPrefix(fileName, "_") {
			continue
		}

		log.Debug(logTag, ": opening file with name: ", fileName)
		fileContents, readErr := static.Asset(fmt.Sprintf("%s/%s", directoryName, fileName))
		if readErr != nil {
			errMsg := "error while reading file with name: " + fileName
			log.Debug(logTag, ": ", errMsg)
			return fmt.Errorf(errMsg)
		}

		api, decodeErr := decodeSpecFileFromBytes(fileContents, fileName)
		if decodeErr != nil {
			errMsg := fmt.Errorf("error while decoding file: `%s`: %s", fileName, decodeErr.Error())
			return errMsg
		}
		apis = append(apis, *api)
	}

	// Print the length of apis
	log.Debug(logTag, ": total apis: ", len(apis))

	middlewareFunction := (&chain{}).Wrap

	for _, api := range apis {
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
		Methods:     []string{http.MethodGet, http.MethodHead},
		Path:        "/",
		HandlerFunc: (&chain{}).Adapt(es.pingES(), classifyCategory, classifyOp, auth.BasicAuth()),
		Description: "You know, for search",
	}
	healthCheckRoute := plugins.Route{
		Name:        "health check",
		Methods:     []string{http.MethodGet, http.MethodHead},
		Path:        "/arc/health",
		HandlerFunc: es.healthCheck(),
		Description: "Retrieve the cluster health, both reactivesearch.io and Elasticsearch",
	}
	routes = append(routes, indexRoute, healthCheckRoute)
	return nil
}

func decodeSpecFileFromBytes(content []byte, name string) (*api, error) {
	var err error

	decoder := json.NewDecoder(bytes.NewReader(content))
	_, err = decoder.Token() // skip opening braces
	if err != nil {
		log.Fatal(err)
		return nil, err
	}
	_, err = decoder.Token() // skip object name
	if err != nil {
		log.Fatal(err)
		return nil, err
	}

	var s spec
	err = decoder.Decode(&s)
	if err != nil {
		log.Fatal(err)
		return nil, err
	}

	specName := strings.TrimSuffix(name, ".json")
	specCategory := decodeCategory(&s)
	specOp := decodeOp(&s)
	specACL, err := decodeACL(specName, &s)
	if err != nil {
		// info, ping specs don't have ACLs
		if !(specName == "info" || specName == "ping") {
			log.Errorln(logTag, ": unable to categorize spec", specName, ":", err)
		}
	}

	return &api{
		name:     specName,
		category: specCategory,
		op:       specOp,
		acl:      *specACL,
		spec:     &s,
	}, nil
}

func (es *elasticsearch) routes() []plugins.Route {
	return routes
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
			c, err := acl.ACLString(pathToken)
			if err != nil {
				return nil, err
			}
			return &c, nil
		}
	}

	aclString := strings.Split(specName, ".")[0]
	a, err := acl.ACLString(aclString)
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
	log.Println("| **Category** | **ACLs** |")
	log.Println("|----------|------|")
	for c, a := range acls {
		log.Println("| ", c, " | ")
		log.Println("<ul>")
		for k := range a {
			log.Println("<li>", k, "</li>")
		}
		log.Println("</ul> |")
	}
}
