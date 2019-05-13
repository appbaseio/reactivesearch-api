package main

import "github.com/appbaseio-confidential/arc/plugins/permissions"
import "github.com/appbaseio-confidential/arc/arc"

var PluginInstance arc.Plugin = permissions.Instance()
