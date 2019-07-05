package main

import "github.com/appbaseio/arc/plugins/reindexer"
import "github.com/appbaseio/arc/plugins"

var PluginInstance plugins.Plugin = reindexer.Instance()
