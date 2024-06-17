package main

import (
	"github.com/appbaseio-confidential/reactivesearch/plugins"
	"github.com/appbaseio-confidential/reactivesearch/plugins/analyticsrequest"
)

var PluginInstance plugins.Plugin = analyticsrequest.Instance()
