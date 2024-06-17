package main

import (
	"github.com/appbaseio-confidential/reactivesearch/plugins"
	"github.com/appbaseio-confidential/reactivesearch/plugins/querytranslate"
)

var PluginInstance plugins.RSPlugin = querytranslate.Instance()
