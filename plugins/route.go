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
	"github.com/gdexlab/go-render/render"
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
	// Matcher function to match for the route. This field might not be provided
	// in which case we need to ignore it
	Matcher mux.MatcherFunc
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
	mu            sync.Mutex
	router        *mux.Router
	port          *int
	address       *string
	isHttps       *bool
	server        http.Server
	Routes        []Route
	isDown        bool
	manualTrigger chan interface{}
}

var (
	singleton            *RouterSwapper
	singletonHealthCheck *RouterHealthCheck
	singletonRSUtil      *RSUtil
	once                 sync.Once
	healthCheckOnce      sync.Once
	rsUtilOnce           sync.Once
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

// GetManualTrigger will return the manual trigger if
// used or a default one that would be empty
func (rs *RouterSwapper) GetManualTrigger() chan interface{} {
	if rs.manualTrigger == nil {
		rs.manualTrigger = make(chan interface{}, 1)
	}
	return rs.manualTrigger
}

// StopServer will stop the server by sending a manual trigger
// to stop the server
func (rs *RouterSwapper) StopServer() {
	rs.GetManualTrigger() <- 1
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

	log.Debug(logTag, "server addr: ", &rs.server, " : server passed: ", render.AsCode(rs.server))

	idleConnectionsClosed := make(chan struct{})
	go func() {
		sigint := make(chan os.Signal, 1)
		signal.Notify(sigint, os.Interrupt)

		// Wait for either interrupt or a internal
		// signal
		select {
		case <-sigint:
		case value := <-rs.GetManualTrigger():
			log.Debug(logTag, ": received manual trigger to shutdown with value: ", value)
		}

		// We received an interrupt signal, shut down.
		log.Debug(logTag, ": going to shutdown the server now:")
		if err := rs.server.Shutdown(context.Background()); err != nil {
			// Error from closing listeners, or context timeout:
			log.Printf("HTTP server Shutdown: %v", err)
		}
		log.Debug(logTag, ": succesfully closed server")
		close(idleConnectionsClosed)
	}()

	var serverError error

	rs.server.Addr = addr
	rs.server.Handler = handler
	rs.isDown = false

	if *rs.isHttps {
		httpsCert := os.Getenv("HTTPS_CERT")
		httpsKey := os.Getenv("HTTPS_KEY")
		serverError = rs.server.ListenAndServeTLS(httpsCert, httpsKey)
	} else {
		serverError = rs.server.ListenAndServe()
	}

	log.Debug(logTag, ": manual trigger value: ", rs.manualTrigger)
	rs.manualTrigger = nil
	rs.isDown = true
	log.Debug(logTag, ": listen and serve exited!")

	if serverError != http.ErrServerClosed {
		// Error starting or closing listener:
		log.Fatalf("HTTP server ListenAndServe: %v", serverError)
	}

	<-idleConnectionsClosed
}

// RestartServer shuts down the current server and starts it again
//
// It is useful when a router swap happens
func (rs *RouterSwapper) RestartServer() {
	// Access the server and shut it down
	log.Debug(logTag, ": Shutting down the current server")

	// Trigger a server shutdown by using the manual trigger
	rs.StopServer()

	shutdownComplete := make(chan bool, 1)
	go func() {
		for !rs.isDown {
			continue
		}
		shutdownComplete <- true
	}()

	<-shutdownComplete

	log.Debug(logTag, ": shutdown complete")

	// Create a new server
	log.Debug(logTag, ": Updating the server since variable")
	var newServer http.Server
	log.Debug(logTag, ": older server: ", &rs.server)

	rs.server = newServer

	log.Debug(logTag, ": newere server: ", &rs.server)
	// If shutdown was successful, start again.
	rs.StartServer()
}

// RouterHealthCheck will handle checking the routers
// health.
type RouterHealthCheck struct {
	// CheckDetails can contain maximum 3 elements.
	CheckedDetails []bool
	port           *int
	address        *string
	isHttps        *bool
}

// RouterHealthCheckInstance returns one instance and should be the
// only way health check is accessed
func RouterHealthCheckInstance() *RouterHealthCheck {
	healthCheckOnce.Do(func() { singletonHealthCheck = &RouterHealthCheck{} })
	return singletonHealthCheck
}

// Append will append the newly added detail to the HealthCheck
// array making sure that it is of length 3.
func (h *RouterHealthCheck) Append(status bool) {
	// We do not need to check the length of the array.
	// We can add an element at the end and store the
	// last 3 elements.
	h.CheckedDetails = append(h.CheckedDetails, status)

	checkDetailsLength := len(h.CheckedDetails)

	// If length is more than 3, keep the last 3
	if checkDetailsLength > 3 {
		h.CheckedDetails = h.CheckedDetails[checkDetailsLength-3:]
	}
}

// Check will check the routers health by
// hitting the dry health check endpoint.
//
// This function should be run with a cron job to be effective.
func (h *RouterHealthCheck) Check() {
	endpoint := "/arc/_health"

	// Build the URL to hit
	ssl := "http"
	if *h.isHttps {
		ssl = "https"
	}

	urlToHit := fmt.Sprintf("%s://%s:%d%s", ssl, *h.address, *h.port, endpoint)
	log.Debug(logTag, ": Hitting ", urlToHit, " for health check")

	status := true

	// Hit the URL now
	//
	// We don't need the response, just need
	// to check if there was an error and accordingly set the status.
	res, err := http.Get(urlToHit)
	if err != nil || res.StatusCode != http.StatusOK {
		status = false
	}
	log.Debug(logTag, ": health check status: ", status)

	h.Append(status)

	// Check if last 3 were false and if so
	// raise a log.Fatal
	//
	// NOTE: If the status is not of length 3, just ignore the error
	if !status && len(h.CheckedDetails) > 2 {
		failCount := 0
		for _, status := range h.CheckedDetails {
			if !status {
				failCount += 1
			}
		}

		if failCount >= 3 {
			// Make the server exit
			log.Fatalln("reactivesearch-api server has stopped accepting requests. Restarting server...!")
		}
	}
}

// SetAttrs sets the router related attributes in the HealthCheck
// struct.
func (h *RouterHealthCheck) SetAttrs(port int, address string, isHttps bool) {
	h.port = &port
	h.address = &address
	h.isHttps = &isHttps
}
