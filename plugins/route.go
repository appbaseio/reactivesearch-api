package plugins

import (
	"net/http"
	"sort"
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
