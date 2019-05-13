package main

import "github.com/appbaseio-confidential/arc/plugins/auth"
import "github.com/appbaseio-confidential/arc/arc"

var PluginInstance arc.Plugin = auth.Instance()
