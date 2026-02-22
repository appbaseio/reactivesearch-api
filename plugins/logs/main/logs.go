package main

import (
	"github.com/appbaseio/reactivesearch-api/plugins"
	"github.com/appbaseio/reactivesearch-api/plugins/logs"
)

var PluginInstance plugins.Plugin = logs.Instance()
