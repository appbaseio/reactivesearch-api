package main

import (
	"github.com/appbaseio/reactivesearch-api/plugins"
	"github.com/appbaseio/reactivesearch-api/plugins/auth"
)

var PluginInstance plugins.Plugin = auth.Instance()
