package main

import (
	"github.com/appbaseio/reactivesearch-api/plugins"
	"github.com/appbaseio/reactivesearch-api/plugins/zinc"
)

var PluginInstance plugins.Plugin = zinc.Instance()
