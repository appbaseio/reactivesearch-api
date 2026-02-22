package main

import (
	"github.com/appbaseio/reactivesearch-api/plugins"
	"github.com/appbaseio/reactivesearch-api/plugins/preferences"
)

var PluginInstance plugins.Plugin = preferences.Instance()
