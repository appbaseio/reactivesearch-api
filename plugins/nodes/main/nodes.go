package main

import (
	"github.com/appbaseio/reactivesearch-api/plugins"
	"github.com/appbaseio/reactivesearch-api/plugins/nodes"
)

var PluginInstance plugins.Plugin = nodes.Instance()
