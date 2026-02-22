package main

import (
	"github.com/appbaseio/reactivesearch-api/plugins"
	"github.com/appbaseio/reactivesearch-api/plugins/openai"
)

var PluginInstance plugins.Plugin = openai.Instance()
