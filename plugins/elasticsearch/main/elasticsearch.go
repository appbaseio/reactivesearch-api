package main

import "github.com/appbaseio-confidential/arc/plugins/elasticsearch"
import "github.com/appbaseio-confidential/arc/plugins"

var PluginInstance plugins.Plugin = elasticsearch.Instance()
