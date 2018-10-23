package permission

import (
	"context"
	"log"
	"regexp"
	"strings"
	"time"

	"github.com/appbaseio-confidential/arc/internal/errors"
	"github.com/appbaseio-confidential/arc/internal/types/acl"
	"github.com/appbaseio-confidential/arc/internal/types/op"
	"github.com/appbaseio-confidential/arc/internal/util"
	"github.com/google/uuid"
)

type contextKey string

const (
	// CtxKey is the key against which a permission is stored in a context.
	CtxKey = contextKey("permission")

	// IndexMapping for the index that houses permission data.
	IndexMapping = `{"settings":{"number_of_shards":3, "number_of_replicas":2}}`
)

// Permission defines a permission type.
type Permission struct {
	UserID    string         `json:"user_id"`
	Username  string         `json:"username"`
	Password  string         `json:"password"`
	Creator   string         `json:"creator"`
	ACLs      []acl.ACL      `json:"acls"`
	Ops       []op.Operation `json:"ops"`
	Indices   []string       `json:"indices"`
	CreatedAt time.Time      `json:"created_at"`
	TTL       time.Duration  `json:"ttl"`
	Limits    *Limits        `json:"limits"`
}

// Limits defines the rate limits for each acls.
type Limits struct {
	IPLimit       int64 `json:"ip_limit"`
	DocsLimit     int64 `json:"docs_limit"`
	SearchLimit   int64 `json:"search_limit"`
	IndicesLimit  int64 `json:"indices_limit"`
	CatLimit      int64 `json:"cat_limit"`
	ClustersLimit int64 `json:"clusters_limit"`
	MiscLimit     int64 `json:"misc_limit"`
}

// Options is a function type used to define a permission's properties.
type Options func(p *Permission) error

// SetUserID sets the userID of a permission.
func SetUserID(userID string) Options {
	return func(p *Permission) error {
		p.UserID = userID
		return nil
	}
}

// SetACLs sets the acls a permission can have access to.
func SetACLs(acls []acl.ACL) Options {
	return func(p *Permission) error {
		if acls == nil {
			return errors.NilACLsError
		}
		p.ACLs = acls
		return nil
	}
}

// SetOps sets the operations a permission can perform.
func SetOps(ops []op.Operation) Options {
	return func(p *Permission) error {
		if ops == nil {
			return errors.NilOpsError
		}
		p.Ops = ops
		return nil
	}
}

// SetIndices sets the indices or index pattens a permission can have access to.
func SetIndices(indices []string) Options {
	return func(p *Permission) error {
		if indices == nil {
			return errors.NilIndicesError
		}
		for _, pattern := range indices {
			pattern = strings.Replace(pattern, "*", ".*", -1)
			if _, err := regexp.Compile(pattern); err != nil {
				return err
			}
		}
		p.Indices = indices
		return nil
	}
}

// SetLimits sets the rate limits for each acl in a permission.
func SetLimits(limits *Limits) Options {
	return func(p *Permission) error {
		p.Limits = limits
		return nil
	}
}

// New creates a new permission by running the Options on it. It returns a
// default permission in case no Options are provided.
func New(creator string, opts ...Options) (*Permission, error) {
	// create a default permission
	p := &Permission{
		UserID:    creator,
		Username:  util.RandStr(),
		Password:  uuid.New().String(),
		Creator:   creator,
		ACLs:      defaultACLs,
		Ops:       defaultOps,
		Indices:   []string{},
		CreatedAt: time.Now(),
		TTL:       time.Duration(util.DaysInCurrentYear()) * 24 * time.Hour,
		Limits:    &defaultLimits,
	}

	// run the options on it
	for _, option := range opts {
		if err := option(p); err != nil {
			return nil, err
		}
	}

	return p, nil
}

// TODO: Remove?
func NewAdmin(creator string) *Permission {
	return &Permission{
		UserID:    creator,
		Username:  util.RandStr(),
		Password:  uuid.New().String(),
		Creator:   creator,
		ACLs:      defaultAdminACLs,
		Ops:       defaultAdminOps,
		Indices:   []string{"*"},
		CreatedAt: time.Now(),
		TTL:       time.Duration(util.DaysInCurrentYear()) * 24 * time.Hour,
		Limits:    &defaultAdminLimits,
	}
}

// FromContext retrieves the permission stored against permission.CtxKey from the context.
func FromContext(ctx context.Context) (*Permission, error) {
	ctxPermission := ctx.Value(CtxKey)
	if ctxPermission == nil {
		return nil, errors.NewNotFoundInRequestContextError("*permission.Permission")
	}
	reqPermission, ok := ctxPermission.(*Permission)
	if !ok {
		return nil, errors.NewInvalidCastError("ctxPermission", "*permission.Permission")
	}
	return reqPermission, nil
}

// IsExpired checks whether the permission is expired or not.
func (p *Permission) IsExpired() bool {
	return time.Since(p.CreatedAt) > p.TTL
}

// HasACL checks whether the permission has access to the given acl.
func (p *Permission) HasACL(acl acl.ACL) bool {
	for _, a := range p.ACLs {
		if a == acl {
			return true
		}
	}
	return false
}

// CanDo checks whether the permission can perform a given operation.
func (p *Permission) CanDo(op op.Operation) bool {
	for _, o := range p.Ops {
		if o == op {
			return true
		}
	}
	return false
}

// CanAccessIndex checks whether the permission has access to given index or index pattern.
func (p *Permission) CanAccessIndex(name string) (bool, error) {
	for _, pattern := range p.Indices {
		matched, err := regexp.MatchString(pattern, name)
		if err != nil {
			log.Printf("invalid index regexp %s encontered: %v", pattern, err)
			return false, err
		}
		if matched {
			return matched, nil
		}
	}
	return false, nil
}

// GetLimitFor returns the rate limit for the given acl in the permission.
func (p *Permission) GetLimitFor(a acl.ACL) int64 {
	switch a {
	case acl.Docs:
		return p.Limits.DocsLimit
	case acl.Search:
		return p.Limits.SearchLimit
	case acl.Indices:
		return p.Limits.IndicesLimit
	case acl.Cat:
		return p.Limits.CatLimit
	case acl.Clusters:
		return p.Limits.ClustersLimit
	case acl.Misc:
		return p.Limits.MiscLimit
	default:
		return 0 // TODO: correct default value?
	}
}

// GetPatch generates a patch doc from the non-zero values in the permission.
func (p *Permission) GetPatch() (map[string]interface{}, error) {
	patch := make(map[string]interface{})

	if p.UserID != "" {
		patch["user_id"] = p.UserID
	}
	if p.Username != "" {
		return nil, errors.NewUnsupportedPatchError("permission", "username")
	}
	if p.Password != "" {
		return nil, errors.NewUnsupportedPatchError("permission", "password")
	}
	if p.Creator != "" {
		return nil, errors.NewUnsupportedPatchError("permission", "creator")
	}
	if p.ACLs != nil {
		patch["acls"] = p.ACLs
	}
	if p.Ops != nil {
		patch["ops"] = p.Ops
	}
	if p.Indices != nil {
		patch["indices"] = p.Indices
	}
	if !p.CreatedAt.Equal(time.Time{}) {
		return nil, errors.NewUnsupportedPatchError("permission", "created_at")
	}
	if p.TTL.String() != "0s" {
		patch["ttl"] = p.TTL
	}
	// TODO: cannot currently patch individual limits to 0
	if p.Limits != nil {
		limits := make(map[string]interface{})
		if p.Limits.IPLimit != 0 {
			limits["ip_limit"] = p.Limits.IPLimit
		}
		if p.Limits.DocsLimit != 0 {
			limits["docs_limit"] = p.Limits.DocsLimit
		}
		if p.Limits.SearchLimit != 0 {
			limits["search_limit"] = p.Limits.SearchLimit
		}
		if p.Limits.IndicesLimit != 0 {
			limits["indices_limit"] = p.Limits.IndicesLimit
		}
		if p.Limits.CatLimit != 0 {
			limits["cat_limit"] = p.Limits.CatLimit
		}
		if p.Limits.ClustersLimit != 0 {
			limits["clusters_limit"] = p.Limits.ClustersLimit
		}
		if p.Limits.MiscLimit != 0 {
			limits["misc_limit"] = p.Limits.MiscLimit
		}
		patch["limits"] = limits
	}

	return patch, nil
}
