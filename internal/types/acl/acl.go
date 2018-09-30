package acl

import (
	"encoding/json"
	"errors"
)

type contextKey string

const CtxKey = contextKey("category")

type ACL int

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
		return errors.New("invalid category encountered: " + category)
	}
	return nil
}

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
		return nil, errors.New("invalid category encountered: " + a.String())
	}
	return json.Marshal(acl)
}

func (a ACL) IsFromES() bool {
	return a == Docs ||
		a == Search ||
		a == Indices ||
		a == Cat ||
		a == Clusters ||
		a == Misc
}

func Contains(slice []ACL, val ACL) bool {
	for _, v := range slice {
		if v == val {
			return true
		}
	}
	return false
}
