package main

import (
	"github.com/appbaseio/reactivesearch-api/plugins"
	"github.com/appbaseio/reactivesearch-api/plugins/reindexer"
)

var PluginInstance plugins.Plugin = reindexer.Instance()
