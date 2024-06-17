package main

import (
	"github.com/appbaseio-confidential/reactivesearch/plugins"
	"github.com/appbaseio-confidential/reactivesearch/plugins/users"
)

var PluginInstance plugins.Plugin = users.Instance()
