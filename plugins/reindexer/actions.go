package reindexer

type Action int

const (
	Mappings Action = iota
	Settings
	Data
	SearchRelevancy
	Synonyms
)

func (o Action) String() string {
	return [...]string{"mappings", "settings", "data", "search_relevancy", "synonyms"}[o]
}

type ReIndexOperation int

const (
	ReIndexWithDelete ReIndexOperation = iota
	ReindexWithClone
)

func (o ReIndexOperation) String() string {
	return [...]string{"reindex_with_delete", "reindex_with_clone"}[o]
}
