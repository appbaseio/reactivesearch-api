package main

import (
	"github.com/appbaseio-confidential/reactivesearch/plugins"
	"github.com/appbaseio-confidential/reactivesearch/plugins/openai"
)

var PluginInstance plugins.Plugin = openai.Instance()
