package main

import (
	"github.com/appbaseio-confidential/reactivesearch/plugins"
	"github.com/appbaseio-confidential/reactivesearch/plugins/searchgrader"
)

var PluginInstance plugins.Plugin = searchgrader.Instance()
