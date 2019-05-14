package main

import "github.com/appbaseio-confidential/arc/plugins/users"
import "github.com/appbaseio-confidential/arc/arc"

var PluginInstance arc.Plugin = users.Instance()
