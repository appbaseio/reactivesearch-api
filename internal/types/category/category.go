package category

import (
	"context"

	"github.com/appbaseio-confidential/arc/internal/errors"
)

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

// FromContext retrieves the category stored against the category.CtxKey from the context.
func FromContext(ctx context.Context) (*Category, error) {
	ctxCategory := ctx.Value(CtxKey)
	if ctxCategory == nil {
		return nil, errors.NewNotFoundInRequestContextError("*category.Category")
	}
	reqCategory, ok := ctxCategory.(*Category)
	if !ok {
		return nil, errors.NewInvalidCastError("ctxCategory", "*category.Category")
	}
	return reqCategory, nil
}

// Contains checks if the given slice of categories contains the given category.
func Contains(categories []Category, category Category) bool {
	for _, c := range categories {
		if c == category {
			return true
		}
	}
	return false
}
