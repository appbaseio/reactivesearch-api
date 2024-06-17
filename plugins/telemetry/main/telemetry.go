package main

import (
	"github.com/appbaseio-confidential/reactivesearch/plugins"
	"github.com/appbaseio-confidential/reactivesearch/plugins/telemetry"
)

var PluginInstance plugins.Plugin = telemetry.Instance()
