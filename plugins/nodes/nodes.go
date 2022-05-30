package nodes

const (
	logTag            = "[nodes]"
	defaultNodesIndex = ".nodes"
	typeName          = "_doc"
	envEsURL          = "ES_CLUSTER_URL"
	settings          = `{ "settings" : { %s "index.number_of_shards" : 1, "index.number_of_replicas" : %d } }`
)
