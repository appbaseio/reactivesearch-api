package user

import (
	"github.com/appbaseio-confidential/arc/internal/types/acl"
	"github.com/appbaseio-confidential/arc/internal/types/op"
)

type contextKey string

const (
	CtxKey       = contextKey("user")
	IndexMapping = `{"settings":{"number_of_shards":3, "number_of_replicas":2}}`
)

type User struct {
	UserId   string       `json:"user_id"`
	Password string       `json:"password"`
	IsAdmin  bool         `json:"is_admin"`
	ACL      []acl.ACL    `json:"acl"`
	Email    string       `json:"email"`
	Op       []op.Operation `json:"op"`
	Indices  []string     `json:"indices"`
}

type Builder interface {
	UserId(string) Builder
	Password(string) Builder
	IsAdmin(bool) Builder
	ACL([]acl.ACL) Builder
	Email(string) Builder
	Op([]op.Operation) Builder
	Indices([]string) Builder
	Build() User
}

type userBuilder struct {
	userId   string
	password string
	isAdmin  bool
	acl      []acl.ACL
	email    string
	op       []op.Operation
	indices  []string
}

func New() Builder {
	return &userBuilder{}
}

func (u *userBuilder) UserId(userId string) Builder {
	u.userId = userId
	return u
}

func (u *userBuilder) Password(password string) Builder {
	u.password = password
	return u
}

func (u *userBuilder) IsAdmin(isAdmin bool) Builder {
	u.isAdmin = isAdmin
	return u
}

func (u *userBuilder) ACL(acl []acl.ACL) Builder {
	u.acl = acl
	return u
}

func (u *userBuilder) Email(email string) Builder {
	u.email = email
	return u
}

func (u *userBuilder) Op(op []op.Operation) Builder {
	u.op = op
	return u
}

func (u *userBuilder) Indices(indices []string) Builder {
	u.indices = indices
	return u
}

func (u *userBuilder) User(user User) Builder {
	u.userId = user.UserId
	u.password = user.Password
	u.isAdmin = user.IsAdmin
	u.acl = user.ACL
	u.email = user.Email
	u.op = user.Op
	u.indices = user.Indices
	return u
}

func (u *userBuilder) Build() User {
	// TODO: Ensure usable zero valued properties
	return User{
		UserId:   u.userId,
		Password: u.password,
		IsAdmin:  u.isAdmin,
		ACL:      u.acl,
		Email:    u.email,
		Op:       u.op,
		Indices:  u.indices,
	}
}
