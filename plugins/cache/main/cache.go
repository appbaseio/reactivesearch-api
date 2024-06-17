package main

import (
	"github.com/appbaseio-confidential/reactivesearch/plugins"
	"github.com/appbaseio-confidential/reactivesearch/plugins/cache"
)

var PluginInstance plugins.Plugin = cache.Instance()
