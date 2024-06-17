package main

import (
	"github.com/appbaseio-confidential/reactivesearch/plugins"
	"github.com/appbaseio-confidential/reactivesearch/plugins/searchrelevancy"
)

// PluginInstance main plugin instance
var PluginInstance plugins.Plugin = searchrelevancy.Instance()
