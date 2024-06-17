package main

import (
	"github.com/appbaseio-confidential/reactivesearch/plugins"
	"github.com/appbaseio-confidential/reactivesearch/plugins/analytics"
)

var PluginInstance plugins.Plugin = analytics.Instance()
