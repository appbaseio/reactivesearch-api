package main

import "github.com/appbaseio-confidential/arc/plugins/logs"
import "github.com/appbaseio-confidential/arc/arc"

var PluginInstance arc.Plugin = logs.Instance()
