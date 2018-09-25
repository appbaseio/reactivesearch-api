package arc

import (
	"strconv"

	"github.com/appbaseio-confidential/arc/arc/plugin"
	"github.com/gorilla/mux"
)

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
	err := p.InitFunc()
	if err != nil {
		return err
	}
	for _, route := range p.Routes() {
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
func ListPlugins() []plugin.Plugin {
	var list []plugin.Plugin
	for _, p := range plugins {
		list = append(list, p)
	}
	return list
}
