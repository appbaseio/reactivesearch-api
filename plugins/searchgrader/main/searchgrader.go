package main

import (
	"github.com/appbaseio/reactivesearch-api/plugins"
	"github.com/appbaseio/reactivesearch-api/plugins/searchgrader"
)

var PluginInstance plugins.Plugin = searchgrader.Instance()
