package main

import (
	"github.com/appbaseio/reactivesearch-api/plugins"
	"github.com/appbaseio/reactivesearch-api/plugins/analytics"
)

var PluginInstance plugins.Plugin = analytics.Instance()
