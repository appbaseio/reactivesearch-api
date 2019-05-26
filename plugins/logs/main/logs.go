package main

import "github.com/appbaseio-confidential/arc/plugins/logs"
import "github.com/appbaseio-confidential/arc/plugins"

var PluginInstance plugins.Plugin = logs.Instance()
