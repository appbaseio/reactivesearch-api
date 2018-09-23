package arc

import (
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
)

// plugins is a map of a unique identifier, usually the plugin name,
// to the Plugin. So, in practice all plugins must have a name,
// preferably following the same practice while naming a package.
var plugins = make(map[string]Plugin)

// Plugin is a type that holds information about the plugin.
type Plugin struct {
	// name is the name of the plugin. Name of the plugin must be
	// unique as it is the name of the plugin that is used as a key
	// to identify a plugin in the plugins map.
	name string

	// initFunc is plugin's setup function that is executed
	// before the plugin routes are loaded in the router.
	initFunc InitFunc

	// routes are the http routes that a plugin handles or is
	// associated with.
	routes []Route
}

// InitFunc is a setup function that is called before loading the
// plugin's routes. It will be called once per plugin and typically,
// it must carry out any kind of initializations before the plugin
// is functional.
type InitFunc func()

// NoSetup is a utility that returns an empty function of type InitFunc.
func NoSetup() InitFunc { return func() {} }

// Route is a type that contains information about a route.
type Route struct {
	// Name is the name of the route. In order to avoid conflicts in
	// the router, the name should be a combination of both http
	// method type and the path. For example: "Get foobar" would be an
	// appropriate name for [GET foobar] endpoint.
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

// NewPlugin returns a new instance of the plugin. Once a plugin
// instance is created it cannot be modified.
func NewPlugin(name string, initFunc InitFunc, routes []Route) Plugin {
	return Plugin{name, initFunc, routes}
}

// Name returns the name of the plugin.
func (p Plugin) Name() string { return p.name }

// InitFunc returns the initFunc associated with the plugin.
func (p Plugin) InitFunc() InitFunc { return p.initFunc }

// Routes returns the routes associated with the plugin.
func (p Plugin) Routes() []Route { return p.routes }

// RegisterPlugin plugs in plugin. All plugins must have a name:
// preferably lowercase and one word. The name of the plugin must
// be unique. A plugin, however, may not define any routes, but
// still be useful, like a middleware.
func RegisterPlugin(p Plugin) {
	if p.name == "" {
		panic("plugin must have a name.")
	}
	if _, dup := plugins[p.name]; dup {
		panic("plugin named " + p.name + " is already registered.")
	}
	plugins[p.name] = p
}

// LoadPlugin is currently responsible for two things: firstly,
// it executes the plugin's initFunc to ensure it makes all the
// initializations before the plugin is functional and second,
// it registers the routes to the router that are associated with
// that plugin.
func LoadPlugin(router *mux.Router, plugin Plugin) error {
	// TODO: asynchronous and more validation before loading plugin routes?
	plugin.initFunc()
	for _, route := range plugin.routes {
		// if router.Get(route.Name) != nil {
		// 	fmt.Println("route with name " + route.Name + " already exists, skipping...")
		// 	continue
		// }

		// TODO: Eliminate this ugly workaround
		// We are handling a path tree here
		// if route.Path == "" {
		// 	err := router.Methods(route.Methods...).
		// 		Name(route.Name).
		// 		PathPrefix(route.PathPrefix).
		// 		HandlerFunc(route.HandlerFunc).
		// 		GetError()
		// 	if err != nil {
		// 		return err
		// 	}
		// 	continue
		// }

		// We are handling specific paths here
		err := router.Methods(route.Methods...).
			Name(route.Name).
			Path(route.Path).
			HandlerFunc(route.HandlerFunc).
			GetError()
		if err != nil {
			return err
		}
	}
	return nil
}

// ListPluginsStr returns a string listing the registered plugins.
func ListPluginsStr() string {
	str := "Registered plugins:\n"
	pl := ListPlugins()
	for i := 0; i < len(pl); i++ {
		str += "\t" + strconv.Itoa(i+1) + ". " + pl[i].Name() + "\n"
	}
	return str
}

// ListPlugins returns the list of plugins that are currently registered.
func ListPlugins() []Plugin {
	var list []Plugin
	for _, plugin := range plugins {
		list = append(list, plugin)
	}
	return list
}

// Middleware is a type that represents a middleware function. A
// middleware usually operates on the request before and after the
// request is served.
type Middleware func(http.HandlerFunc) http.HandlerFunc

// Chainer is implemented by any value that has a Wrap method,
// which wraps a single middleware or a 'chain' of middleware on
// top of the given handler function and returns the resulting
// 'wrapped' handler function.
type Chainer interface {
	// Wrap wraps a handler in a single middleware or a set of middleware.
	// The sequence of in which the handler is wrapped can be defined
	// manually or it can be defined by an Adapter.
	Wrap(http.HandlerFunc) http.HandlerFunc
}

// Adapter is implemented by any value that has Adapt method.
// Adapter is a type that can 'adapt' a given handler to the
// set of middleware in a specific order.
type Adapter interface {
	// Adapt adapts a handler to a given set of middleware in a
	// specific order. The request gets passed on to the next
	// middleware according to the order in which the handler is
	// adapted.
	Adapt(http.HandlerFunc, ...Middleware) http.HandlerFunc
}
