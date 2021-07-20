package main

import (
	"github.com/appbaseio/reactivesearch-api/plugins"
	"github.com/appbaseio/reactivesearch-api/plugins/permissions"
)

var PluginInstance plugins.Plugin = permissions.Instance()
