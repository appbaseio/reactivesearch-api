package main

import (
	"github.com/appbaseio/reactivesearch-api/plugins"
	"github.com/appbaseio/reactivesearch-api/plugins/applycache"
)

var PluginInstance plugins.Plugin = applycache.Instance()
