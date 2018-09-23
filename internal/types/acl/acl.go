package acl

import (
	"encoding/json"
	"errors"
)

type ACL int

const (
	Get ACL = iota
	Post
	Delete
	Settings
	Bulk
	Search
	Streams
	Analytics
)

func (a ACL) String() string {
	return [...]string{
		"get",
		"post",
		"delete",
		"settings",
		"bulk",
		"search",
		"streams",
		"analytics",
	}[a]
}

func (a *ACL) UnmarshalJSON(bytes []byte) error {
	var acl string
	err := json.Unmarshal(bytes, &acl)
	if err != nil {
		return err
	}
	switch acl {
	case Get.String():
		*a = Get
	case Post.String():
		*a = Post
	case Delete.String():
		*a = Delete
	case Settings.String():
		*a = Settings
	case Bulk.String():
		*a = Bulk
	case Search.String():
		*a = Search
	case Streams.String():
		*a = Streams
	case Analytics.String():
		*a = Analytics
	default:
		return errors.New("invalid acl encountered: " + acl)
	}
	return nil
}

func (a ACL) MarshalJSON() ([]byte, error) {
	var acl string
	switch a {
	case Get:
		acl = Get.String()
	case Post:
		acl = Post.String()
	case Delete:
		acl = Delete.String()
	case Settings:
		acl = Settings.String()
	case Bulk:
		acl = Bulk.String()
	case Search:
		acl = Search.String()
	case Streams:
		acl = Streams.String()
	case Analytics:
		acl = Analytics.String()
	default:
		return nil, errors.New("invalid acl encountered: " + a.String())
	}
	return json.Marshal(acl)
}
