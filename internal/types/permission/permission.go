package permission

import (
	"time"

	"github.com/appbaseio-confidential/arc/internal/types/acl"
	"github.com/appbaseio-confidential/arc/internal/types/op"
	"github.com/appbaseio-confidential/arc/internal/util"
	"github.com/google/uuid"
)

type contextKey string

const (
	CtxKey       = contextKey("permission")
	IndexMapping = `{"settings":{"number_of_shards":3, "number_of_replicas":2}}`
)

// TODO: map category to acls
type Permission struct {
	UserId    string         `json:"user_id"`
	UserName  string         `json:"user_name"`
	Password  string         `json:"password"`
	Creator   string         `json:"creator"`
	ACL       []acl.ACL      `json:"acl"`
	Op        []op.Operation `json:"op"`
	Indices   []string       `json:"indices"`
	CreatedAt time.Time      `json:"created_at"`
	TTL       time.Duration  `json:"expires_at"`
	IPLimit   int64          `json:"ip_limit"`
	ACLLimit  int64          `json:"acl_limit"`
}

type Builder interface {
	Permission(Permission) Builder
	Username(string) Builder
	Creator(string) Builder
	ACL([]acl.ACL) Builder
	Op([]op.Operation) Builder
	Indices([]string) Builder
	Build() Permission
}

type permissionBuilder struct {
	userId   string
	username string
	password string
	creator  string
	acl      []acl.ACL
	op       []op.Operation
	indices  []string
}

func (p *permissionBuilder) Permission(permission Permission) Builder {
	p.username = permission.UserName
	p.password = permission.Password
	p.creator = permission.Creator
	p.acl = permission.ACL
	p.op = permission.Op
	p.indices = permission.Indices
	return p
}

func (p *permissionBuilder) Username(username string) Builder {
	p.username = username
	return p
}

func (p *permissionBuilder) Creator(creator string) Builder {
	p.creator = creator
	return p
}

func (p *permissionBuilder) ACL(acl []acl.ACL) Builder {
	p.acl = acl
	return p
}

func (p *permissionBuilder) Op(op []op.Operation) Builder {
	p.op = op
	return p
}

func (p *permissionBuilder) Indices(indices []string) Builder {
	p.indices = indices
	return p
}

func (p *permissionBuilder) Build() Permission {
	return Permission{
		UserId:    p.userId,
		UserName:  util.RandStr(),
		Password:  uuid.New().String(),
		Creator:   p.creator,
		ACL:       p.acl,
		Op:        p.op,
		Indices:   p.indices,
		CreatedAt: time.Now(),
		TTL:       24 * time.Hour,
	}
}

func NewBuilder(userId string) Builder {
	return &permissionBuilder{
		userId: userId,
	}
}

// TODO: Avoid exposing functions that assumes required fields
func New(userId, creator string) Permission {
	return Permission{
		UserId:   userId,
		UserName: util.RandStr(),
		Password: uuid.New().String(),
		Creator:  creator,
		ACL:      []acl.ACL{},
		Op:       []op.Operation{op.Read},
		Indices:  []string{},
	}
}
