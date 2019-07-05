package main

import "github.com/appbaseio/arc/plugins/logs"
import "github.com/appbaseio/arc/plugins"

var PluginInstance plugins.Plugin = logs.Instance()
