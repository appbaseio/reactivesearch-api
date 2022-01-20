package category

import (
	"context"
	"encoding/json"

	"github.com/appbaseio/reactivesearch-api/errors"
	"github.com/appbaseio/reactivesearch-api/model/acl"
)

type contextKey string

// ctxKey is a key against which an category.Categories is stored in the context.
const ctxKey = contextKey("category")

// Category represents category type
type Category int

// Currently supported category.
const (
	Docs Category = iota
	Search
	Indices
	Cat
	Clusters
	Misc
	User
	Permission
	Analytics
	Streams
	Rules
	Suggestions
	Auth
	ReactiveSearch
	SearchRelevancy
	Synonyms
	SearchGrader
	UIBuilder
	Logs
	Cache
	StoredQuery
	Sync
	Pipelines
)

// String is an implementation of Stringer interface that returns the string representation of category.Categories.
func (c Category) String() string {
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
		"rules",
		"suggestions",
		"auth",
		"reactivesearch",
		"searchrelevancy",
		"synonyms",
		"searchgrader",
		"uibuilder",
		"logs",
		"cache",
		"storedquery",
		"sync",
		"pipelines",
	}[c]
}

// UnmarshalJSON is an implementation of Unmarshaler interface for unmarshaling category.Categories.
func (c *Category) UnmarshalJSON(bytes []byte) error {
	var category string
	err := json.Unmarshal(bytes, &category)
	if err != nil {
		return err
	}
	switch category {
	case Docs.String():
		*c = Docs
	case Search.String():
		*c = Search
	case Indices.String():
		*c = Indices
	case Cat.String():
		*c = Cat
	case Clusters.String():
		*c = Clusters
	case Misc.String():
		*c = Misc
	case User.String():
		*c = User
	case Permission.String():
		*c = Permission
	case Analytics.String():
		*c = Analytics
	case Streams.String():
		*c = Streams
	case Rules.String():
		*c = Rules
	case Suggestions.String():
		*c = Suggestions
	case Auth.String():
		*c = Auth
	case ReactiveSearch.String():
		*c = ReactiveSearch
	case SearchRelevancy.String():
		*c = SearchRelevancy
	case Synonyms.String():
		*c = Synonyms
	case SearchGrader.String():
		*c = SearchGrader
	case UIBuilder.String():
		*c = UIBuilder
	case Logs.String():
		*c = Logs
	case Cache.String():
		*c = Cache
	case StoredQuery.String():
		*c = StoredQuery
	case Sync.String():
		*c = Sync
	case Pipelines.String():
		*c = Pipelines
	default:
		return nil
	}
	return nil
}

// MarshalJSON is the implementation of Marshaler interface for marshaling category.Categories type.
func (c Category) MarshalJSON() ([]byte, error) {
	var category string
	switch c {
	case Docs:
		category = Docs.String()
	case Search:
		category = Search.String()
	case Indices:
		category = Indices.String()
	case Cat:
		category = Cat.String()
	case Clusters:
		category = Clusters.String()
	case Misc:
		category = Misc.String()
	case User:
		category = User.String()
	case Permission:
		category = Permission.String()
	case Analytics:
		category = Analytics.String()
	case Streams:
		category = Streams.String()
	case Rules:
		category = Rules.String()
	case Suggestions:
		category = Suggestions.String()
	case Auth:
		category = Auth.String()
	case ReactiveSearch:
		category = ReactiveSearch.String()
	case SearchRelevancy:
		category = SearchRelevancy.String()
	case Synonyms:
		category = Synonyms.String()
	case SearchGrader:
		category = SearchGrader.String()
	case UIBuilder:
		category = UIBuilder.String()
	case Logs:
		category = Logs.String()
	case Cache:
		category = Cache.String()
	case StoredQuery:
		category = StoredQuery.String()
	case Sync:
		category = Sync.String()
	case Pipelines:
		category = Pipelines.String()
	default:
		return nil, nil
	}
	return json.Marshal(category)
}

// IsFromES checks whether the category is one of the elasticsearch category, i.e.
// one of [docs, search, indices, cat, clusters, misc]
func (c Category) IsFromES() bool {
	return c == Docs ||
		c == Search ||
		c == Indices ||
		c == Cat ||
		c == Clusters ||
		c == Misc
}

// IsFromRS checks whether the category is of the reactivesearch category.
func (c Category) IsFromRS() bool {
	return c == ReactiveSearch
}

// HasACL checks whether the given acl is a value in the category categories.
func (c Category) HasACL(a acl.ACL) bool {
	return acl.Contains(c.ACLs(), a)
}

// ACLs returns the categories associated with the category.
func (c Category) ACLs() []acl.ACL {
	switch c {
	case Docs:
		return []acl.ACL{
			acl.Reindex,
			acl.Termvectors,
			acl.Update,
			acl.Create,
			acl.Mtermvectors,
			acl.Bulk,
			acl.Delete,
			acl.Source,
			acl.DeleteByQuery,
			acl.Get,
			acl.Mget,
			acl.UpdateByQuery,
			acl.Index,
			acl.Exists,
		}
	case Search:
		return []acl.ACL{
			acl.FieldCaps,
			acl.Msearch,
			acl.Validate,
			acl.RankEval,
			acl.Render,
			acl.SearchShards,
			acl.Search,
			acl.Count,
			acl.Explain,
		}
	case Cat:
		return []acl.ACL{
			acl.Cat,
		}
	case Indices:
		return []acl.ACL{
			acl.Upgrade,
			acl.Settings,
			acl.Indices,
			acl.Split,
			acl.Aliases,
			acl.Stats,
			acl.Template,
			acl.Open,
			acl.Mapping,
			acl.Recovery,
			acl.Analyze,
			acl.Cache,
			acl.Forcemerge,
			acl.Alias,
			acl.Refresh,
			acl.Segments,
			acl.Close,
			acl.Flush,
			acl.Shrink,
			acl.ShardStores,
			acl.Rollover,
		}
	case Clusters:
		return []acl.ACL{
			acl.Remote,
			acl.Cat,
			acl.Nodes,
			acl.Tasks,
			acl.Cluster,
		}
	case Misc:
		return []acl.ACL{
			acl.Scripts,
			acl.Get,
			acl.Ingest,
			acl.Snapshot,
		}
	default:
		return []acl.ACL{}
	}
}

// ACLsFor given categories returns a list of all the acls that belong to those categories.
func ACLsFor(categories ...Category) []acl.ACL {
	acls := make([]acl.ACL, 0)
	set := make(map[acl.ACL]bool)
	for _, c := range categories {
		for _, a := range c.ACLs() {
			if _, ok := set[a]; !ok {
				set[a] = true
				acls = append(acls, a)
			}
		}
	}
	return acls
}

// NewContext returns a new context with the given Category.
func NewContext(ctx context.Context, c *Category) context.Context {
	return context.WithValue(ctx, ctxKey, c)
}

// FromContext retrieves the category stored against the category.CtxKey from the context.
func FromContext(ctx context.Context) (*Category, error) {
	ctxACL := ctx.Value(ctxKey)
	if ctxACL == nil {
		return nil, errors.NewNotFoundInContextError("*category.Categories")
	}
	reqACL, ok := ctxACL.(*Category)
	if !ok {
		return nil, errors.NewInvalidCastError("ctxACL", "*category.Categories")
	}
	return reqACL, nil
}

// FromString returns the Categories from string tags.
func FromString(tag string) Category {
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
