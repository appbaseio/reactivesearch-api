package main

import (
	"github.com/appbaseio/reactivesearch-api/plugins"
	"github.com/appbaseio/reactivesearch-api/plugins/analyticsrequest"
)

var PluginInstance plugins.Plugin = analyticsrequest.Instance()
