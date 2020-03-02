package main

import (
	"github.com/appbaseio/arc/plugins"
	"github.com/appbaseio/arc/plugins/auth"
)

var PluginInstance plugins.Plugin = auth.Instance()
