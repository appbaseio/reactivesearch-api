package main

import (
	"github.com/appbaseio/reactivesearch-api/plugins"
	"github.com/appbaseio/reactivesearch-api/plugins/elasticsearch"
)

var PluginInstance plugins.ESPlugin = elasticsearch.Instance()
