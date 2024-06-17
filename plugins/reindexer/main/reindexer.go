package main

import (
	"github.com/appbaseio-confidential/reactivesearch/plugins"
	"github.com/appbaseio-confidential/reactivesearch/plugins/reindexer"
)

var PluginInstance plugins.Plugin = reindexer.Instance()
