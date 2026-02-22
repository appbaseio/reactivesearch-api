package main

import (
	"github.com/appbaseio/reactivesearch-api/plugins"
	"github.com/appbaseio/reactivesearch-api/plugins/users"
)

var PluginInstance plugins.Plugin = users.Instance()
