package main

import "github.com/appbaseio/arc/plugins/users"
import "github.com/appbaseio/arc/plugins"

var PluginInstance plugins.Plugin = users.Instance()
