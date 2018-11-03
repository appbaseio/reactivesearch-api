package category

type contextKey string

// CtxKey is a key against which an category.Category is stored in the context.
const CtxKey = contextKey("category")

// Category is a type that represents an elasticsearch category.
type Category int

// Elasticsearch request categories.
const (
	Cat Category = iota
	Bulk
	Cluster
	Search
	Remote
	Create
	Count
	Scripts
	Delete
	Doc
	Source
	FieldCaps
	Close
	Analyze
	Exists
	Get
	Template
	Explain
	Indices
	Alias
	Aliases
	DeleteByQuery
	Cache
	Index
	Mapping
	Flush
	Forcemerge
	Upgrade
	Settings
	Open
	Recovery
	Mappings
	Rollover
	Refresh
	Segments
	Shrink
	Split
	ShardStores
	Stats
	Ingest
	Validate
	Msearch
	Mget
	Nodes
	Mtermvectors
	Reindex
	UpdateByQuery
	Render
	RankEval
	SearchShards
	Snapshot
	Tasks
	Termvectors
	Update
)
