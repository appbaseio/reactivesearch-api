package main

import (
	"github.com/appbaseio-confidential/reactivesearch/plugins"
	"github.com/appbaseio-confidential/reactivesearch/plugins/auth"
)

var PluginInstance plugins.Plugin = auth.Instance()
