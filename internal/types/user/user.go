package user

import (
	"regexp"
	"strings"

	"github.com/appbaseio-confidential/arc/internal/errors"
	"github.com/appbaseio-confidential/arc/internal/types/acl"
	"github.com/appbaseio-confidential/arc/internal/types/op"
)

type contextKey string

const (
	CtxKey       = contextKey("user")
	IndexMapping = `{"settings":{"number_of_shards":3, "number_of_replicas":2}}`
)

type User struct {
	UserId   string         `json:"user_id"`
	Password string         `json:"password"`
	IsAdmin  *bool          `json:"is_admin"`
	ACLs     []acl.ACL      `json:"acls"`
	Email    string         `json:"email"`
	Ops      []op.Operation `json:"ops"`
	Indices  []string       `json:"indices"`
}

type Options func(u *User) error

func SetIsAdmin(isAdmin *bool) Options {
	return func(u *User) error {
		u.IsAdmin = isAdmin
		return nil
	}
}

func SetACLs(acls []acl.ACL) Options {
	return func(u *User) error {
		if acls == nil {
			return errors.NilACLsError
		}
		u.ACLs = acls
		return nil
	}
}

func SetEmail(email string) Options {
	return func(u *User) error {
		u.Email = email
		return nil
	}
}

func SetOps(ops []op.Operation) Options {
	return func(u *User) error {
		if ops == nil {
			return errors.NilOpsError
		}
		u.Ops = ops
		return nil
	}
}

func SetIndices(indices []string) Options {
	return func(u *User) error {
		if indices == nil {
			return errors.NilIndicesError
		}
		for _, pattern := range indices {
			pattern = strings.Replace(pattern, "*", ".*", -1)
			if _, err := regexp.Compile(pattern); err != nil {
				return err
			}
		}
		u.Indices = indices
		return nil
	}
}

func New(userId, password string, opts ...Options) (*User, error) {
	// create a default user
	u := &User{
		UserId:   userId,
		Password: password,
		IsAdmin:  &isAdminFalse, // pointer to bool
		ACLs:     defaultACLs,
		Ops:      defaultOps,
		Indices:  []string{},
	}

	// run the options on it
	for _, option := range opts {
		if err := option(u); err != nil {
			return nil, err
		}
	}

	return u, nil
}

func NewAdmin(userId, password string) *User {
	return &User{
		UserId:   userId,
		Password: password,
		IsAdmin:  &isAdminTrue,
		ACLs:     defaultAdminACLs,
		Ops:      defaultAdminOps,
		Indices:  []string{"*"},
	}
}

func (u *User) HasACL(acl acl.ACL) bool {
	for _, a := range u.ACLs {
		if a == acl {
			return true
		}
	}
	return false
}

func (u *User) CanDo(op op.Operation) bool {
	for _, o := range u.Ops {
		if o == op {
			return true
		}
	}
	return false
}

func (u *User) CanAccessIndex(name string) (bool, error) {
	for _, pattern := range u.Indices {
		pattern := strings.Replace(pattern, "*", ".*", -1)
		matched, err := regexp.MatchString(pattern, name)
		if err != nil {
			return false, err
		}
		if matched {
			return true, nil
		}
	}
	return false, nil
}

func (u *User) GetPatch() (map[string]interface{}, error) {
	patch := make(map[string]interface{})

	if u.UserId != "" {
		patch["user_id"] = u.UserId
	}
	if u.Password != "" {
		patch["password"] = u.Password
	}
	if u.IsAdmin != nil {
		patch["is_admin"] = u.IsAdmin
	}
	if u.Email != "" {
		patch["email"] = u.Email
	}
	if u.ACLs != nil {
		patch["acls"] = u.ACLs
	}
	if u.Ops != nil {
		patch["ops"] = u.Ops
	}
	if u.Indices != nil {
		patch["indices"] = u.Indices
	}

	return patch, nil
}
