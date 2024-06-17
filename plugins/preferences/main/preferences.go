package main

import (
	"github.com/appbaseio-confidential/reactivesearch/plugins"
	"github.com/appbaseio-confidential/reactivesearch/plugins/preferences"
)

var PluginInstance plugins.Plugin = preferences.Instance()
