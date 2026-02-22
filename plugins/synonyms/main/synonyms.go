package main

import (
	"github.com/appbaseio/reactivesearch-api/plugins"
	"github.com/appbaseio/reactivesearch-api/plugins/synonyms"
)

// PluginInstance main plugin instance
var PluginInstance plugins.Plugin = synonyms.Instance()
