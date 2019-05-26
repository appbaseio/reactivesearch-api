package main

import "github.com/appbaseio-confidential/arc/plugins/reindexer"
import "github.com/appbaseio-confidential/arc/plugins"

var PluginInstance plugins.Plugin = reindexer.Instance()
