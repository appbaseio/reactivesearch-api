package main

import (
	"github.com/appbaseio/reactivesearch-api/plugins"
	"github.com/appbaseio/reactivesearch-api/plugins/storedquery"
)

var PluginInstance plugins.Plugin = storedquery.Instance()
