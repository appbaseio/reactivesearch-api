package main

import "github.com/appbaseio/reactivesearch-api/plugins/suggestions"
import "github.com/appbaseio/reactivesearch-api/plugins"

var PluginInstance plugins.Plugin = suggestions.Instance()
