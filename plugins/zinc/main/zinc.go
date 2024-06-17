package main

import (
	"github.com/appbaseio-confidential/reactivesearch/plugins"
	"github.com/appbaseio-confidential/reactivesearch/plugins/zinc"
)

var PluginInstance plugins.Plugin = zinc.Instance()
