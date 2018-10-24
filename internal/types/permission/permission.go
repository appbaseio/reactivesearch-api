package permission

import (
	"context"
	"fmt"
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
	// Credential is a value stored against request.Maker key in the context.
	// It basically acts as an identifier that tells whether the request uses
	// permission credential.
	Credential = contextKey("permission_credential")

	// CtxKey is the key against which a permission is stored in a context.
	CtxKey = contextKey("permission")

	// IndexMapping for the index that houses permission data.
	IndexMapping = `{ "settings" : { "number_of_shards" : 3, "number_of_replicas" : 2 } }`
)

// Permission defines a permission type.
type Permission struct {
	Username  string         `json:"username"`
	Password  string         `json:"password"`
	Owner     string         `json:"owner"`
	Creator   string         `json:"creator"`
	ACLs      []acl.ACL      `json:"acls"`
	Ops       []op.Operation `json:"ops"`
	Indices   []string       `json:"indices"`
	CreatedAt string         `json:"created_at"`
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

// SetOwner sets the owner of a permission.
func SetOwner(owner string) Options {
	return func(p *Permission) error {
		if owner == "" {
			return fmt.Errorf("permission owner cannot be an empty string")
		}
		p.Owner = owner
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
// default permission in case no Options are provided. The default owner of
// the permission is the creator itself.
func New(creator string, opts ...Options) (*Permission, error) {
	if creator == "" {
		return nil, fmt.Errorf("permission creator cannot be an empty string")
	}

	// create a default permission
	p := &Permission{
		Username:  util.RandStr(),
		Password:  uuid.New().String(),
		Owner:     creator,
		Creator:   creator,
		ACLs:      defaultACLs,
		Ops:       defaultOps,
		Indices:   []string{},
		CreatedAt: time.Now().Format(time.RFC3339),
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

// NewAdmin creates a new admin permission by running the Options on it. It returns
// a permission with admin defaults in case no Options are provided. The default owner
// of the permission is the creator itself.
func NewAdmin(creator string, opts ...Options) (*Permission, error) {
	if creator == "" {
		return nil, fmt.Errorf("permission creator cannot be an empty string")
	}

	p := &Permission{
		Username:  util.RandStr(),
		Password:  uuid.New().String(),
		Owner:     creator,
		Creator:   creator,
		ACLs:      adminACLs,
		Ops:       adminOps,
		Indices:   []string{"*"},
		CreatedAt: time.Now().Format(time.RFC3339),
		TTL:       time.Duration(util.DaysInCurrentYear()) * 24 * time.Hour,
		Limits:    &defaultAdminLimits,
	}

	// run the options on it
	for _, option := range opts {
		if err := option(p); err != nil {
			return nil, err
		}
	}

	return p, nil
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
func (p *Permission) IsExpired() (bool, error) {
	createdAt, err := time.Parse(time.RFC3339, p.CreatedAt)
	if err != nil {
		return false, fmt.Errorf("invalid time format for field \"created_at\": %s", p.CreatedAt)
	}
	return time.Since(createdAt) > p.TTL, nil
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
		pattern = strings.Replace(pattern, "*", ".*", -1)
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
func (p *Permission) GetLimitFor(a acl.ACL) (int64, error) {
	switch a {
	case acl.Docs:
		return p.Limits.DocsLimit, nil
	case acl.Search:
		return p.Limits.SearchLimit, nil
	case acl.Indices:
		return p.Limits.IndicesLimit, nil
	case acl.Cat:
		return p.Limits.CatLimit, nil
	case acl.Clusters:
		return p.Limits.ClustersLimit, nil
	case acl.Misc:
		return p.Limits.MiscLimit, nil
	default:
		return -1, fmt.Errorf(`we do not rate limit "%s" acl`, a)
	}
}

// GetIPLimit returns the IPLimit i.e. the number of requests allowed per IP address per hour.
func (p *Permission) GetIPLimit() int64 {
	return p.Limits.IPLimit
}

// GetPatch generates a patch doc from the non-zero values in the permission.
func (p *Permission) GetPatch() (map[string]interface{}, error) {
	patch := make(map[string]interface{})

	if p.Owner != "" {
		patch["owner"] = p.Owner
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
	if p.CreatedAt != "" {
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
