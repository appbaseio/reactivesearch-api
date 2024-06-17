package main

import (
	"github.com/appbaseio-confidential/reactivesearch/plugins"
	"github.com/appbaseio-confidential/reactivesearch/plugins/logs"
)

var PluginInstance plugins.Plugin = logs.Instance()
