package main

import (
	"github.com/appbaseio/reactivesearch-api/plugins"
	"github.com/appbaseio/reactivesearch-api/plugins/pipelines"
)

var PluginInstance plugins.Plugin = pipelines.Instance()
