package main

import "github.com/appbaseio-confidential/arc/plugins/elasticsearch"
import "github.com/appbaseio-confidential/arc/arc"

var PluginInstance arc.Plugin = elasticsearch.Instance()
