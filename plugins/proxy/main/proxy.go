package main

import "github.com/appbaseio-confidential/reactivesearch/plugins/proxy"
import "github.com/appbaseio-confidential/reactivesearch/plugins"

var PluginInstance plugins.Plugin = proxy.Instance()
