package main

import (
	"github.com/appbaseio-confidential/reactivesearch/plugins"
	"github.com/appbaseio-confidential/reactivesearch/plugins/rules"
)

var PluginInstance plugins.Plugin = rules.Instance()
