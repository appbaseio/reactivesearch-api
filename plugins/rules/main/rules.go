package main

import (
	"github.com/appbaseio/reactivesearch-api/plugins"
	"github.com/appbaseio/reactivesearch-api/plugins/rules"
)

var PluginInstance plugins.Plugin = rules.Instance()
