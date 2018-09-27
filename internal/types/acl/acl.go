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

func (c ACL) String() string {
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
	}[c]
}

func (c *ACL) UnmarshalJSON(bytes []byte) error {
	var category string
	err := json.Unmarshal(bytes, &c)
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
	default:
		return errors.New("invalid category encountered: " + category)
	}
	return nil
}

func (c ACL) MarshalJSON() ([]byte, error) {
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
	default:
		return nil, errors.New("invalid category encountered: " + c.String())
	}
	return json.Marshal(category)
}
