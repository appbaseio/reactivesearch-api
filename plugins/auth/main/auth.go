package main

import "github.com/appbaseio-confidential/arc/plugins/auth"
import "github.com/appbaseio-confidential/arc/plugins"

var PluginInstance plugins.Plugin = auth.Instance()
