package main

import (
	"github.com/appbaseio-confidential/reactivesearch/plugins"
	"github.com/appbaseio-confidential/reactivesearch/plugins/synonyms"
)

// PluginInstance main plugin instance
var PluginInstance plugins.Plugin = synonyms.Instance()
