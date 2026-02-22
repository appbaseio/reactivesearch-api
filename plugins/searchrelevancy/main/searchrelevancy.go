package main

import (
	"github.com/appbaseio/reactivesearch-api/plugins"
	"github.com/appbaseio/reactivesearch-api/plugins/searchrelevancy"
)

// PluginInstance main plugin instance
var PluginInstance plugins.Plugin = searchrelevancy.Instance()
