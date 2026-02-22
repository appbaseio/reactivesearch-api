# Plugins

ReactiveSearch API can be extended with **plugins**. Plugins can enhance/augment the ReactiveSearch. These are "plugged-in" at compile time. ReactiveSearch provides a set of functionalities on top of Elasticsearch such as a declarative querying API, authentication and access controls. A ReactiveSearch plugin can usually interacts with HTTP requests and responses to implement a custom business logic. A plugin can extend the functionality of ReactiveSearch by handling a set of routes or it can serve as a simple middleware to other plugins. 

## File structure

For brevity, all the plugins should (but is not restricted to) follow the following file structure that outline the individual functionalities of each plugin component:

* `routes.go`: Defines a list of routes to interact with the entities and logic that is handled by the plugin.
* `handlers.go`: Defines the respective handlers for the routes handled by the plugin.
* `dao.go`: Plugins might probably require to retain and manage some configuration or information provided by the user. 
	This file should contain the *database access object* that is required to interact with the data store of your choice.
* `middleware.go`: Defines middleware or *chain* of middleware required to wrap the handlers to perform operations before the request is actually served.
* `plugin_name.go`: This is the core of the plugin. This is where the plugin gets registered to Arc.

These are the main components that plugins usually deal with, however a plugin is not restricted to these files structure strictly.

## Implementing a custom plugin

We will consider creating a simple `greeter` plugin that handles the route `GET /greet` and returns a simple greeting message in response to the request.

### 1. Create a package and a type that implements the plugin interface

Following the above directory structure we will create the following required files in a separate Go package called `greeter`:
```
greeter
├── greeter.go
├── routes.go
└── handlers.go
```
The `greeter.go` would be responsible for implementing the `Plugin` interface and registering itself as a plugin to the Arc. The `Plugin` interface has three methods that a type must implement in order to register itself as a plugin:

- `greeter.go`
		
	```go
	package greeter

	import (
		"fmt"
	
		"github.com/appbaseio/reactivesearch-api/plugins"
	)

	const pluginName = "greeter"

	type Greeter struct {
		message string
	}

	// Name returns the name of the plugin. Name of the plugin must be
	// unique as it is the name of the plugin that is used as a key
	// to identify a plugin in the plugins map.
	func (g *Greeter) Name() string {
		return pluginName	
	}

	// InitFunc returns the plugin's setup function that is executed
	// before the plugin routes are loaded in the router.
	func (g *Greeter) InitFunc() error {
		fmt.Printf("%s: initializing plugin...", pluginName)
		return nil	
	}

	// Routes returns the http routes that a plugin handles or is
	// associated with.
	func (g *Greeter) Routes() []plugin.Route {
		return g.routes()
	}
	```

### 2. Define the routes

Define a list of routes that the plugin aims to handle.
- `routes.go`

	```go
	package greeter

	import (
	 	"net/http"
 	
  		"github.com/appbaseio/reactivesearch-api/plugins"
	)
	
	func (g *Greeter) routes() []plugin.Route {
		return []plugin.Route{	
			{
				Name: "Get greetings",
				Methods: []string{http.MethodGet},
				Path: "/greet",
				HandlerFunc: g.greetHandler(),
				Description: "Send greetings",
			},
		}
	}
	```
	
### 3. Implement the handlers

In `handlers.go` implement a method that returns a `http.HandlerFunc` which encapsulates the custom logic to handle a specific route or a set of routes.
- `handlers.go`
	```go
	package greeter 

	import "net/http"

	func (g *Greeter) greetHandler() http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOk)
			w.Write([]byte(g.message))
		}
	}
	```

### 4. Register your plugin.

Place the plugin inside of the `plugins` directory for it to be compiled and registered with the ReactiveSearch API. Finally, to instantiate the plugin, use the `initFunc` function:

- `greeter.go`
	```go
	...
	func InitFunc() {
		// perform any state instantiation, e.g. reading env variables


		return nil
	}
	...
	```