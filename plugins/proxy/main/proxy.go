package main

import "github.com/appbaseio/arc/plugins/proxy"
import "github.com/appbaseio/arc/plugins"

var PluginInstance plugins.Plugin = proxy.Instance()
