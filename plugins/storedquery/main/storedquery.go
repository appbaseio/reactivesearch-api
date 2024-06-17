package main

import (
	"github.com/appbaseio-confidential/reactivesearch/plugins"
	"github.com/appbaseio-confidential/reactivesearch/plugins/storedquery"
)

var PluginInstance plugins.Plugin = storedquery.Instance()
