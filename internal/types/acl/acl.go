package acl

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/appbaseio-confidential/arc/internal/errors"
	"github.com/appbaseio-confidential/arc/internal/types/category"
)

type contextKey string

// CtxKey is a key against which an acl.ACL is stored in the context.
const CtxKey = contextKey("category")

// ACL represents acl type
type ACL int

// Currently supported acls.
const (
	Docs ACL = iota
	Search
	Indices
	Cat
	Clusters
	Misc
	User
	Permission
	Analytics
	Streams
)

// String is an implementation of Stringer interface that returns the string representation of acl.ACL type.
func (a ACL) String() string {
	return [...]string{
		"docs",
		"search",
		"indices",
		"cat",
		"clusters",
		"misc",
		"user",
		"permission",
		"analytics",
		"streams",
	}[a]
}

// UnmarshalJSON is an implementation of Unmarshaler interface for unmarshaling acl.ACL type.
func (a *ACL) UnmarshalJSON(bytes []byte) error {
	var category string
	err := json.Unmarshal(bytes, &category)
	if err != nil {
		return err
	}
	switch category {
	case Docs.String():
		*a = Docs
	case Search.String():
		*a = Search
	case Indices.String():
		*a = Indices
	case Cat.String():
		*a = Cat
	case Clusters.String():
		*a = Clusters
	case Misc.String():
		*a = Misc
	case User.String():
		*a = User
	case Permission.String():
		*a = Permission
	case Analytics.String():
		*a = Analytics
	case Streams.String():
		*a = Streams
	default:
		return fmt.Errorf("invalid category encountered: %v" + category)
	}
	return nil
}

// MarshalJSON is the implementation of Marshaler interface for marshaling acl.ACL type.
func (a ACL) MarshalJSON() ([]byte, error) {
	var acl string
	switch a {
	case Docs:
		acl = Docs.String()
	case Search:
		acl = Search.String()
	case Indices:
		acl = Indices.String()
	case Cat:
		acl = Cat.String()
	case Clusters:
		acl = Clusters.String()
	case Misc:
		acl = Misc.String()
	case User:
		acl = User.String()
	case Permission:
		acl = Permission.String()
	case Analytics:
		acl = Analytics.String()
	case Streams:
		acl = Streams.String()
	default:
		return nil, fmt.Errorf("invalid category encountered: %v" + a.String())
	}
	return json.Marshal(acl)
}

// IsFromES checks whether the acl is one of the elasticsearch acls, i.e.
// one of [docs, search, indices, cat, clusters, misc]
func (a ACL) IsFromES() bool {
	return a == Docs ||
		a == Search ||
		a == Indices ||
		a == Cat ||
		a == Clusters ||
		a == Misc
}

// HasCategory checks whether the given category is a value in the acl categories.
func (a ACL) HasCategory(c category.Category) bool {
	return category.Contains(a.Categories(), c)
}

// Categories returns the categories associated with the acl.
func (a ACL) Categories() []category.Category {
	switch a {
	case Docs:
		return []category.Category{
			category.Reindex,
			category.Termvectors,
			category.Update,
			category.Create,
			category.Mtermvectors,
			category.Bulk,
			category.Delete,
			category.Source,
			category.DeleteByQuery,
			category.Get,
			category.Mget,
			category.UpdateByQuery,
			category.Index,
			category.Exists,
		}
	case Search:
		return []category.Category{
			category.FieldCaps,
			category.Msearch,
			category.Validate,
			category.RankEval,
			category.Render,
			category.SearchShards,
			category.Search,
			category.Count,
			category.Explain,
		}
	case Cat:
		return []category.Category{
			category.Cat,
		}
	case Indices:
		return []category.Category{
			category.Upgrade,
			category.Settings,
			category.Indices,
			category.Split,
			category.Aliases,
			category.Stats,
			category.Template,
			category.Open,
			category.Mapping,
			category.Recovery,
			category.Analyze,
			category.Cache,
			category.Forcemerge,
			category.Alias,
			category.Refresh,
			category.Segments,
			category.Close,
			category.Flush,
			category.Shrink,
			category.ShardStores,
			category.Rollover,
		}
	case Clusters:
		return []category.Category{
			category.Remote,
			category.Cat,
			category.Nodes,
			category.Tasks,
			category.Cluster,
		}
	case Misc:
		return []category.Category{
			category.Scripts,
			category.Get,
			category.Ingest,
			category.Snapshot,
		}
	default:
		return []category.Category{}
	}
}

// CategoriesFor returns a list of all the categories for given acls.
func CategoriesFor(acls ...ACL) []category.Category {
	var categories []category.Category
	set := make(map[category.Category]bool)
	for _, acl := range acls {
		for _, c := range acl.Categories() {
			if _, ok := set[c]; !ok {
				set[c] = true
				categories = append(categories, c)
			}
		}
	}
	return categories
}

// FromContext retrieves the acl stored against the acl.CtxKey from the context.
func FromContext(ctx context.Context) (*ACL, error) {
	ctxACL := ctx.Value(CtxKey)
	if ctxACL == nil {
		return nil, errors.NewNotFoundInRequestContextError("*acl.ACL")
	}
	reqACL, ok := ctxACL.(*ACL)
	if !ok {
		return nil, errors.NewInvalidCastError("ctxACL", "*acl.ACL")
	}
	return reqACL, nil
}

// FromString returns the ACl from string tags.
func FromString(tag string) ACL {
	switch tag {
	case "docs":
		return Docs
	case "search":
		return Search
	case "indices":
		return Indices
	case "cat":
		return Cat
	case "tasks":
		return Clusters
	case "cluster":
		return Clusters
	default:
		return Misc
	}
}
