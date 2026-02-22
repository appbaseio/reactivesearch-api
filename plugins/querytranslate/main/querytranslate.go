package main

import (
	"github.com/appbaseio/reactivesearch-api/plugins"
	"github.com/appbaseio/reactivesearch-api/plugins/querytranslate"
)

var PluginInstance plugins.RSPlugin = querytranslate.Instance()
