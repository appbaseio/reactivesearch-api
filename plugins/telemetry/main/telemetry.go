package main

import (
	"github.com/appbaseio/reactivesearch-api/plugins"
	"github.com/appbaseio/reactivesearch-api/plugins/telemetry"
)

var PluginInstance plugins.Plugin = telemetry.Instance()
