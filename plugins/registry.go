package plugins

import (
	"log"
	"sort"
	"strconv"

	"github.com/appbaseio-confidential/arc/middleware"

	"github.com/gorilla/mux"
)

const logTag = "[registry]"

// plugins is a map of a unique identifier, usually the plugin name,
// to the Plugin. So, in practice all plugins must have a name,
// preferably following the same practice while naming a package.
var plugins = make(map[string]Plugin)

// Plugin is a type that holds information about the plugin.
type Plugin interface {
	// Name returns the name of the plugin. Name of the plugin must be
	// unique as it is the name of the plugin that is used as a key
	// to identify a plugin in the plugins map.
	Name() string

	// InitFunc returns the plugin's setup function that is executed
	// before the plugin routes are loaded in the router.
	// 
	// mw takes a array of middleware to be intialized by the plugin.
        // This is expected to be populated only for the ES plugin.
	InitFunc(mw []middleware.Middleware) error

	// Routes returns the http routes that a plugin handles or is
	// associated with.
	Routes() []Route

	// The plugin's elastic search middleware, if any.
	ESMiddleware() [] middleware.Middleware
}

// RegisterPlugin plugs in plugin. All plugins must have a name:
// preferably lowercase and one word. The name of the plugin must
// be unique. A plugin, however, may not define any routes, but
// still be useful, like a middleware.
func RegisterPlugin(p Plugin) {
	name := p.Name()
	if name == "" {
		panic("plugin must have a name.")
	}
	if _, dup := plugins[name]; dup {
		panic("plugin named " + name + " is already registered.")
	}
	plugins[name] = p
}

// LoadPlugin is currently responsible for two things: firstly,
// it executes the plugin's initFunc to ensure it makes all the
// initializations before the plugin is functional and second,
// it registers the routes to the router that are associated with
// that plugin.
func LoadPlugin(router *mux.Router, p Plugin, mw [] middleware.Middleware) error {
	// TODO: asynchronous and more validation before loading plugin routes?
	log.Printf("%s: Initializing plugin: %s", logTag, p.Name())
	err := p.InitFunc(mw)
	if err != nil {
		return err
	}
	for _, r := range p.Routes() {
		err := router.Methods(r.Methods...).
			Name(r.Name).
			Path(r.Path).
			HandlerFunc(r.HandlerFunc).
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
	for _, p := range plugins {
		list = append(list, p)
	}
	return list
}

// By is the type of a "less" function that defines the ordering of Plugins.
type PluginBy func(p1, p2 Plugin) bool

// Sort is a method on the function type, By, that sorts
// the argument slice according to the function.
func (by PluginBy) PluginSort(plugins []Plugin) {
	ps := &pluginSorter{
		plugins: plugins,
		by:      by,
	}
	sort.Sort(ps)
}

// pluginSorter joins a By function and a slice of Plugins to be sorted.
type pluginSorter struct {
	plugins []Plugin
	by      PluginBy
}

// Len is part of sort.Interface that returns the length
// of slice to be sorted.
func (s *pluginSorter) Len() int {
	return len(s.plugins)
}

// Swap is part of sort.Interface that defines a way
// to swap two plugins in the slice.
func (s *pluginSorter) Swap(i, j int) {
	s.plugins[i], s.plugins[j] = s.plugins[j], s.plugins[i]
}

// Less is part of sort.Interface. It is implemented by calling
// the "by" closure in the sorter.
func (s *pluginSorter) Less(i, j int) bool {
	return s.by(s.plugins[i], s.plugins[j])
}
