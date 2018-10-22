package acl

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/appbaseio-confidential/arc/internal/errors"
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
