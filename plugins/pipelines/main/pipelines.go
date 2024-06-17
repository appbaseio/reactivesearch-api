package main

import (
	"github.com/appbaseio-confidential/reactivesearch/plugins"
	"github.com/appbaseio-confidential/reactivesearch/plugins/pipelines"
)

var PluginInstance plugins.Plugin = pipelines.Instance()
