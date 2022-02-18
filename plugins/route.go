package plugins

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"sort"
	"sync"

	"github.com/appbaseio/reactivesearch-api/middleware/logger"
	"github.com/appbaseio/reactivesearch-api/model/tracktime"
	"github.com/gorilla/mux"
	"github.com/rs/cors"
	log "github.com/sirupsen/logrus"
)

// Route is a type that contains information about a route.
type Route struct {
	// Name is the name of the route. In order to avoid conflicts in
	// the router, the name preferably should be a combination of both
	// http method type and the path. For example: "Get foobar" would
	// be an appropriate name for [GET foobar] endpoint.
	Name string

	// Methods represents an array of HTTP method type. It is preferable
	// to use values defined in net/http package to avoid typos.
	Methods []string

	// Path is the path that it expects to serve the requests on.
	// If the path contains any variables, then they must be declared
	// in accordance to the format define by the router to which
	// these routes are registered which in our case is: gorilla/mux.
	Path string

	// PathPrefix serves as a matcher for the URL path prefix.
	// This matches if the given template is a prefix of the full
	// URL path. See Route.Path() for details on the tpl argument.
	// Note that it does not treat slashes specially ("/foobar/"
	// will be matched by the prefix "/foo") so you may want to
	// use a trailing slash here.
	PathPrefix string

	// HandlerFunc is the handler function that is responsible for
	// responding the request made to this route.
	HandlerFunc http.HandlerFunc

	// Description about this route.
	Description string

	// Indicate whether the current route is a special pipeline router
	// or not
	IsPipeline bool
}

// By is the type of a "less" function that defines the ordering of routes.
type RouteBy func(r1, r2 Route) bool

// Sort is a method on the function type, By, that sorts
// the argument slice according to the function.
func (by RouteBy) RouteSort(routes []Route) {
	rs := &routeSorter{
		routes: routes,
		by:     by,
	}
	sort.Sort(rs)
}

// routeSorter joins a By function and a slice of routes to be sorted.
type routeSorter struct {
	routes []Route
	by     RouteBy
}

// Len is part of sort.Interface that returns the length
// of slice to be sorted.
func (rs *routeSorter) Len() int {
	return len(rs.routes)
}

// Swap is part of sort.Interface that defines a way
// to swap two plugins in the slice.
func (rs *routeSorter) Swap(i, j int) {
	rs.routes[i], rs.routes[j] = rs.routes[j], rs.routes[i]
}

// Less is part of sort.Interface. It is implemented by calling
// the "by" closure in the sorter.
func (rs *routeSorter) Less(i, j int) bool {
	return rs.by(rs.routes[i], rs.routes[j])
}

// routerSwapper lets routers to be swapper
type RouterSwapper struct {
	mu      sync.Mutex
	router  *mux.Router
	port    *int
	address *string
	isHttps *bool
	server  http.Server
	Routes  []Route
}

var (
	singleton *RouterSwapper
	once      sync.Once
)

// RouterSwapperInstance returns one instance and should be the
// only way swapper is accessed
// Pipelines plugin deals with managing user defined pipelines.
func RouterSwapperInstance() *RouterSwapper {
	once.Do(func() { singleton = &RouterSwapper{} })
	return singleton
}

// Router exposes the router from the RouterSwapper instance
func (rs *RouterSwapper) Router() *mux.Router {
	return rs.router
}

// Swap swaps the passed router with the older one
func (rs *RouterSwapper) Swap(newRouter *mux.Router) {
	rs.mu.Lock()
	rs.router = newRouter
	rs.mu.Unlock()
}

// SetRouterAttrs sets the router attributes to the current
// instance of RouterSwapper
func (rs *RouterSwapper) SetRouterAttrs(address string, port int, isHttps bool) {
	rs.address = &address
	rs.port = &port
	rs.isHttps = &isHttps
}

// StartServer starts the server by using the latest routerswapper
// interface router, creating a handler and listening
func (rs *RouterSwapper) StartServer() {
	// CORS policy
	c := cors.New(cors.Options{
		AllowedOrigins: []string{"*"},
		AllowedMethods: []string{"HEAD", "GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders: []string{"*"},
		ExposedHeaders: []string{"*"},
	})

	handler := c.Handler(rs.Router())

	// Add time tracker middleware
	handler = tracktime.Track(handler)
	// Add logger middleware
	handler = logger.Log(handler)

	// Listen and serve ...
	addr := fmt.Sprintf("%s:%d", *rs.address, *rs.port)
	log.Println(logTag, ":listening on", addr)

	idleConnsClosed := make(chan struct{})
	go func() {
		sigint := make(chan os.Signal, 1)
		signal.Notify(sigint, os.Interrupt)
		<-sigint

		// We received an interrupt signal, shut down.
		if err := rs.server.Shutdown(context.Background()); err != nil {
			// Error from closing listeners, or context timeout:
			log.Printf("HTTP server Shutdown: %v", err)
		}
		close(idleConnsClosed)
	}()

	var serverError error

	rs.server.Addr = addr
	rs.server.Handler = handler

	if *rs.isHttps {
		httpsCert := os.Getenv("HTTPS_CERT")
		httpsKey := os.Getenv("HTTPS_KEY")
		serverError = rs.server.ListenAndServeTLS(httpsCert, httpsKey)
	} else {
		serverError = rs.server.ListenAndServe()
	}

	if serverError != http.ErrServerClosed {
		// Error starting or closing listener:
		log.Fatalf("HTTP server ListenAndServe: %v", serverError)
	}

	<-idleConnsClosed
}

// RestartServer shuts down the current server and starts it again
//
// It is useful when a router swap happens
func (rs *RouterSwapper) RestartServer() {
	// Access the server and shut it down
	err := rs.server.Shutdown(context.Background())
	if err != nil {
		log.Errorln("Something went wrong while shutting down server: ", err)
		return
	}

	// If shutdown was succesfull, start again.
	rs.StartServer()
}
