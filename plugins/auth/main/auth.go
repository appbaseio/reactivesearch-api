package main

import "github.com/appbaseio/arc/plugins/auth"
import "github.com/appbaseio/arc/plugins"

var PluginInstance plugins.Plugin = auth.Instance()
