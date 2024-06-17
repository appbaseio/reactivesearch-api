package main

import (
	"github.com/appbaseio-confidential/reactivesearch/plugins"
	"github.com/appbaseio-confidential/reactivesearch/plugins/applycache"
)

var PluginInstance plugins.Plugin = applycache.Instance()
