package acl

import (
	"context"

	"github.com/appbaseio/reactivesearch-api/errors"
)

type contextKey string

// ctxKey is a key against which an acl.ACL is stored in the context.
const ctxKey = contextKey("acl")

// ACL is a type that represents an elasticsearch acl.
type ACL int

// Elasticsearch request Categories.
const (
	Cat ACL = iota
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
	ReactiveSearch
)

// NewContext returns a new context with the given ACL.
func NewContext(ctx context.Context, acl *ACL) context.Context {
	return context.WithValue(ctx, ctxKey, acl)
}

// FromContext retrieves the acl stored against the acl.CtxKey from the context.
func FromContext(ctx context.Context) (*ACL, error) {
	ctxCategory := ctx.Value(ctxKey)
	if ctxCategory == nil {
		return nil, errors.NewNotFoundInContextError("*acl.ACL")
	}
	reqCategory, ok := ctxCategory.(*ACL)
	if !ok {
		return nil, errors.NewInvalidCastError("ctxCategory", "*acl.ACL")
	}
	return reqCategory, nil
}

// Contains checks if the given slice of category contains the given acl.
func Contains(acls []ACL, acl ACL) bool {
	for _, a := range acls {
		if a == acl {
			return true
		}
	}
	return false
}
