package main

import (
	"github.com/appbaseio/reactivesearch-api/plugins"
	"github.com/appbaseio/reactivesearch-api/plugins/sync"
)

var PluginInstance plugins.Plugin = sync.Instance()
