package plugin

import "net/http"

// Plugin is a type that holds information about the plugin.
type Plugin interface {
	// Name returns the name of the plugin. Name of the plugin must be
	// unique as it is the name of the plugin that is used as a key
	// to identify a plugin in the plugins map.
	Name() string

	// InitFunc returns the plugin's setup function that is executed
	// before the plugin routes are loaded in the router.
	InitFunc() error

	// Routes returns the http routes that a plugin handles or is
	// associated with.
	Routes() []Route
}

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
