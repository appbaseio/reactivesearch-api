package main

import "github.com/appbaseio-confidential/arc/plugins/reindexer"
import "github.com/appbaseio-confidential/arc/arc"

var PluginInstance arc.Plugin = reindexer.Instance()
