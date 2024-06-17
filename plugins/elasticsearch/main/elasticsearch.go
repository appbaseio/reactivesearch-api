package main

import (
	"github.com/appbaseio-confidential/reactivesearch/plugins"
	"github.com/appbaseio-confidential/reactivesearch/plugins/elasticsearch"
)

var PluginInstance plugins.ESPlugin = elasticsearch.Instance()
