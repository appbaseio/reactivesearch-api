package permission

import (
	"log"
	"regexp"
	"time"

	"github.com/appbaseio-confidential/arc/internal/errors"
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

type Permission struct {
	UserId    string         `json:"user_id"`
	UserName  string         `json:"user_name"`
	Password  string         `json:"password"`
	Creator   string         `json:"creator"`
	ACLs      []acl.ACL      `json:"acl"`
	Ops       []op.Operation `json:"op"`
	Indices   []string       `json:"indices"`
	CreatedAt time.Time      `json:"created_at"`
	TTL       time.Duration  `json:"ttl"`
	Limits    *Limits        `json:"limits"`
}

type Limits struct {
	IPLimit          int64 `json:"ip_limit"`
	DocsLimit        int64 `json:"docs_limit"`
	SearchLimit      int64 `json:"search_limit"`
	IndicesLimit     int64 `json:"indices_limit"`
	CatLimit         int64 `json:"cat_limit"`
	ClustersLimit    int64 `json:"clusters_limit"`
	MiscLimit        int64 `json:"misc_limit"`
	UsersLimit       int64 `json:"users_limit"`
	PermissionsLimit int64 `json:"permissions_limit"`
	AnalyticsLimit   int64 `json:"analytics_limit"`
	StreamsLimit     int64 `json:"streams_limit"`
}

type Options func(p *Permission) error

func SetUserId(userId string) Options {
	return func(p *Permission) error {
		p.UserId = userId
		return nil
	}
}

func SetACLs(acls []acl.ACL) Options {
	return func(p *Permission) error {
		if acls == nil {
			return errors.NilACLsError
		}
		p.ACLs = acls
		return nil
	}
}

func SetOps(ops []op.Operation) Options {
	return func(p *Permission) error {
		if ops == nil {
			return errors.NilOpsError
		}
		p.Ops = ops
		return nil
	}
}

func SetIndices(indices []string) Options {
	return func(p *Permission) error {
		if indices == nil {
			return errors.NilIndicesError
		}
		p.Indices = indices
		return nil
	}
}

func SetLimits(limits *Limits) Options {
	return func(p *Permission) error {
		p.Limits = limits
		return nil
	}
}

func New(creator string, opts ...Options) (*Permission, error) {
	// create a default permission
	p := &Permission{
		UserId:    creator,
		UserName:  util.RandStr(),
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

// TODO: Temporary?
func NewAdmin(creator string) *Permission {
	return &Permission{
		UserId:    creator,
		UserName:  util.RandStr(),
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

func (p *Permission) IsExpired() bool {
	return time.Since(p.CreatedAt) > p.TTL
}

func (p *Permission) HasACL(acl acl.ACL) bool {
	for _, a := range p.ACLs {
		if a == acl {
			return true
		}
	}
	return false
}

func (p *Permission) Can(op op.Operation) bool {
	for _, o := range p.Ops {
		if o == op {
			return true
		}
	}
	return false
}

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
	case acl.User:
		return p.Limits.UsersLimit
	case acl.Permission:
		return p.Limits.PermissionsLimit
	case acl.Analytics:
		return p.Limits.AnalyticsLimit
	case acl.Streams:
		return p.Limits.StreamsLimit
	default:
		// TODO: unreachable state?
		return p.Limits.IPLimit
	}
}

func (p *Permission) GetPatch() (map[string]interface{}, error) {
	patch := make(map[string]interface{})

	if p.UserId != "" {
		patch["user_id"] = p.UserId
	}
	if p.UserName != "" {
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
	if p.TTL.String() != "" {
		patch["ttl"] = p.TTL
	}
	if p.Limits != nil {
		patch["limits"] = p.Limits
	}

	return patch, nil
}
