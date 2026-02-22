package main

import (
	"github.com/appbaseio/reactivesearch-api/plugins"
	"github.com/appbaseio/reactivesearch-api/plugins/cache"
)

var PluginInstance plugins.Plugin = cache.Instance()
