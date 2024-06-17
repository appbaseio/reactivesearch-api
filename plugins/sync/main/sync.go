package main

import (
	"github.com/appbaseio-confidential/reactivesearch/plugins"
	"github.com/appbaseio-confidential/reactivesearch/plugins/sync"
)

var PluginInstance plugins.Plugin = sync.Instance()
