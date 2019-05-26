package main

import "github.com/appbaseio-confidential/arc/plugins/users"
import "github.com/appbaseio-confidential/arc/plugins"

var PluginInstance plugins.Plugin = users.Instance()
