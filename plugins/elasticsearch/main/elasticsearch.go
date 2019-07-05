package main

import "github.com/appbaseio/arc/plugins/elasticsearch"
import "github.com/appbaseio/arc/plugins"

var PluginInstance plugins.Plugin = elasticsearch.Instance()
