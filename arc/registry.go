package arc

import (
	"log"
	"sort"
	"strconv"

	"github.com/appbaseio-confidential/arc/arc/plugin"
	"github.com/gorilla/mux"
)

const logTag = "[registry]"

// plugins is a map of a unique identifier, usually the plugin name,
// to the Plugin. So, in practice all plugins must have a name,
// preferably following the same practice while naming a package.
var plugins = make(map[string]plugin.Plugin)

// RegisterPlugin plugs in plugin. All plugins must have a name:
// preferably lowercase and one word. The name of the plugin must
// be unique. A plugin, however, may not define any routes, but
// still be useful, like a middleware.
func RegisterPlugin(p plugin.Plugin) {
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
func LoadPlugin(router *mux.Router, p plugin.Plugin) error {
	// TODO: asynchronous and more validation before loading plugin routes?
	log.Printf("%s: Initializing plugin: %s", logTag, p.Name())
	err := p.InitFunc()
	if err != nil {
		return err
	}
	for _, route := range p.Routes() {
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
func ListPlugins() []plugin.Plugin {
	var list []plugin.Plugin
	for _, p := range plugins {
		list = append(list, p)
	}
	return list
}

// By is the type of a "less" function that defines the
// ordering its Plugin arguments
type By func(p1, p2 plugin.Plugin) bool

// Sort is a method on the function type, By, that sorts
// the argument slice according to the function.
func (by By) Sort(plugins []plugin.Plugin) {
	ps := &pluginSorter{
		plugins: plugins,
		by:      by,
	}
	sort.Sort(ps)
}

// pluginSorted joins a By function and a slice of Plugins
// to be sorted.
type pluginSorter struct {
	plugins []plugin.Plugin
	by      By
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
