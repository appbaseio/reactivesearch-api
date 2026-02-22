package main

import "github.com/appbaseio/reactivesearch-api/plugins/proxy"
import "github.com/appbaseio/reactivesearch-api/plugins"

var PluginInstance plugins.Plugin = proxy.Instance()
