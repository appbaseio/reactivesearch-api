package main

import (
	"github.com/appbaseio-confidential/reactivesearch/plugins"
	"github.com/appbaseio-confidential/reactivesearch/plugins/permissions"
)

var PluginInstance plugins.Plugin = permissions.Instance()
