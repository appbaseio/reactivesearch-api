package main

import "github.com/appbaseio-confidential/reactivesearch/plugins/suggestions"
import "github.com/appbaseio-confidential/reactivesearch/plugins"

var PluginInstance plugins.Plugin = suggestions.Instance()
