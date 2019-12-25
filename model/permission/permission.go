package permission

import (
	"context"
	"fmt"
	"log"
	"net"
	"regexp"
	"strings"
	"time"

	"github.com/appbaseio/arc/errors"
	"github.com/appbaseio/arc/model/acl"
	"github.com/appbaseio/arc/model/category"
	"github.com/appbaseio/arc/model/op"
	"github.com/appbaseio/arc/util"
	"github.com/google/uuid"
)

type contextKey string

const (
	// Credential is a value stored against request.Maker key in the context.
	// It basically acts as an identifier that tells whether the request uses
	// permission credential.
	Credential = contextKey("permission_credential")

	// ctxKey is the key against which a permission is stored in a context.
	ctxKey = contextKey("permission")
)

// Permission defines a permission type.
type Permission struct {
	Username    string              `json:"username"`
	Password    string              `json:"password"`
	Owner       string              `json:"owner"`
	Creator     string              `json:"creator"`
	Role        string              `json:"role"`
	Categories  []category.Category `json:"categories"`
	ACLs        []acl.ACL           `json:"acls"`
	Ops         []op.Operation      `json:"ops"`
	Indices     []string            `json:"indices"`
	Sources     []string            `json:"sources"`
	Referers    []string            `json:"referers"`
	CreatedAt   string              `json:"created_at"`
	TTL         time.Duration       `json:"ttl"`
	Limits      *Limits             `json:"limits"`
	Description string              `json:"description"`
	Includes    []string            `json:"include_fields"`
	Excludes    []string            `json:"exclude_fields"`
	Expired     bool                `json:"expired"`
}

// Limits defines the rate limits for each category.
type Limits struct {
	IPLimit          int64 `json:"ip_limit"`
	DocsLimit        int64 `json:"docs_limit"`
	SearchLimit      int64 `json:"search_limit"`
	IndicesLimit     int64 `json:"indices_limit"`
	CatLimit         int64 `json:"cat_limit"`
	ClustersLimit    int64 `json:"clusters_limit"`
	MiscLimit        int64 `json:"misc_limit"`
	UserLimit        int64 `json:"user_limit"`
	PermissionLimit  int64 `json:"permission_limit"`
	AnalyticsLimit   int64 `json:"analytics_limit"`
	RulesLimit       int64 `json:"rules_limit"`
	TemplatesLimit   int64 `json:"templates_limit"`
	SuggestionsLimit int64 `json:"suggestions_limit"`
	StreamsLimit     int64 `json:"streams_limit"`
	AuthLimit        int64 `json:"auth_limit"`
	FunctionsLimit   int64 `json:"functions_limit"`
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

func SetRole(role string) Options {
	return func(p *Permission) error {
		p.Role = role
		return nil
	}
}

// SetCategories sets the categories a permission can have access to.
func SetCategories(categories []category.Category) Options {
	return func(p *Permission) error {
		if categories == nil {
			return errors.ErrNilCategories
		}
		p.Categories = categories
		return nil
	}
}

// SetACLs sets the acls a permission can have access to.
func SetACLs(acls []acl.ACL) Options {
	return func(p *Permission) error {
		if acls == nil {
			return errors.ErrNilCategories
		}

		for _, c := range acls {
			if !p.hasCategoryForACL(c) {
				return fmt.Errorf(`permission doesn't have category to access "%s" acl`, c)
			}
		}

		p.ACLs = acls
		return nil
	}
}

// SetOps sets the operations a permission can perform.
func SetOps(ops []op.Operation) Options {
	return func(p *Permission) error {
		if ops == nil {
			return errors.ErrNilOps
		}
		p.Ops = ops
		return nil
	}
}

// SetIndices sets the indices or index patterns a permission can have access to.
func SetIndices(indices []string) Options {
	return func(p *Permission) error {
		if indices == nil {
			return errors.ErrNilIndices
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

// SetSources sets the sources from which the permission can make request from.
// Sources are accepted and parsed in CIDR notation.
func SetSources(sources []string) Options {
	return func(p *Permission) error {
		if sources == nil {
			return errors.ErrNilSources
		}
		if err := validateSources(sources); err != nil {
			return err
		}
		p.Sources = sources
		return nil
	}
}

// SetIncludes sets the includes fields
func SetIncludes(includes []string) Options {
	return func(p *Permission) error {
		p.Includes = includes
		return nil
	}
}

// SetExcludes sets the excludes fields
func SetExcludes(excludes []string) Options {
	return func(p *Permission) error {
		p.Excludes = excludes
		return nil
	}
}

func validateSources(sources []string) error {
	for _, source := range sources {
		_, _, err := net.ParseCIDR(source)
		if err != nil {
			return fmt.Errorf(`source "%s" is not a valid CIDR notation: %v`, source, err)
		}
	}
	return nil
}

// SetReferers sets the referers from which the permission can make request from.
func SetReferers(referers []string) Options {
	return func(p *Permission) error {
		if referers == nil {
			return errors.ErrNilReferers
		}
		if err := validateReferers(referers); err != nil {
			return err
		}
		p.Referers = referers
		return nil
	}
}

func validateReferers(referers []string) error {
	for _, referer := range referers {
		referer = strings.Replace(referer, "*", ".*", -1)
		if _, err := regexp.Compile(referer); err != nil {
			return fmt.Errorf(`invalid referer regexp "%s" encountered: %v`, referer, err)
		}
	}
	return nil
}

// SetLimits sets the rate limits for each category in a permission.
func SetLimits(limits *Limits) Options {
	return func(p *Permission) error {
		p.Limits = limits
		return nil
	}
}

// SetDescription sets the permission description.
func SetDescription(description string) Options {
	return func(p *Permission) error {
		p.Description = description
		return nil
	}
}

// SetTTL sets the permission's time-to-live.
func SetTTL(duration time.Duration) Options {
	return func(p *Permission) error {
		p.TTL = duration
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
		Username:   util.RandStr(),
		Password:   uuid.New().String(),
		Owner:      creator,
		Creator:    creator,
		Role:       "",
		Categories: defaultCategories,
		Ops:        defaultOps,
		Indices:    []string{},
		Sources:    []string{"0.0.0.0/0"},
		Referers:   []string{"*"},
		CreatedAt:  time.Now().Format(time.RFC3339),
		TTL:        -1,
		Limits:     &defaultLimits,
	}

	// run the options on it
	for _, option := range opts {
		if err := option(p); err != nil {
			return nil, err
		}
	}

	// set the acls if not set by options explicitly
	if p.ACLs == nil {
		p.ACLs = category.ACLsFor(p.Categories...)
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
		Username:   util.RandStr(),
		Password:   uuid.New().String(),
		Owner:      creator,
		Creator:    creator,
		Role:       "",
		Categories: adminCategories,
		Ops:        adminOps,
		Indices:    []string{"*"},
		Sources:    []string{"0.0.0.0/0"},
		Referers:   []string{"*"},
		CreatedAt:  time.Now().Format(time.RFC3339),
		TTL:        -1,
		Limits:     &defaultAdminLimits,
	}

	// run the options on it
	for _, option := range opts {
		if err := option(p); err != nil {
			return nil, err
		}
	}

	// set the acls if not set by options explicitly
	if p.ACLs == nil {
		p.ACLs = category.ACLsFor(p.Categories...)
	}

	return p, nil
}

// NewContext returns a new context with the given permission.
func NewContext(ctx context.Context, p *Permission) context.Context {
	return context.WithValue(ctx, ctxKey, p)
}

// FromContext retrieves the permission stored against permission.CtxKey from the context.
func FromContext(ctx context.Context) (*Permission, error) {
	ctxPermission := ctx.Value(ctxKey)
	if ctxPermission == nil {
		return nil, errors.NewNotFoundInContextError("*permission.Permission")
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
	return p.TTL >= 0 && time.Since(createdAt) > p.TTL, nil
}

// HasCategory checks whether the permission has access to the given category.
func (p *Permission) HasCategory(category category.Category) bool {
	for _, c := range p.Categories {
		if c == category {
			return true
		}
	}
	return false
}

func (p *Permission) hasCategoryForACL(acl acl.ACL) bool {
	for _, c := range p.Categories {
		if c.HasACL(acl) {
			return true
		}
	}
	return false
}

// ValidateACLs checks if the permission can possess the given set of categories.
func (p *Permission) ValidateACLs(acls ...acl.ACL) error {
	for _, a := range acls {
		if !p.hasCategoryForACL(a) {
			return fmt.Errorf(`permission doesn't have category to access "%s" acl`, a)
		}
	}
	return nil
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

// CanAccessCluster checks whether the user can access cluster level routes.
func (p *Permission) CanAccessCluster() (bool, error) {
	for _, pattern := range p.Indices {
		pattern = strings.Replace(pattern, "*", ".*", -1)
		matched, err := regexp.MatchString(pattern, "*")
		if err != nil {
			return false, err
		}
		if matched {
			return true, nil
		}
	}
	return false, nil
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

// CanAccessIndices checks whether the user has access to the given indices.
func (p *Permission) CanAccessIndices(indices ...string) (bool, error) {
	for _, index := range indices {
		if ok, err := p.CanAccessIndex(index); !ok || err != nil {
			return ok, err
		}
	}
	return true, nil
}

// GetLimitFor returns the rate limit for the given category in the permission.
func (p *Permission) GetLimitFor(c category.Category) (int64, error) {
	switch c {
	case category.Docs:
		return p.Limits.DocsLimit, nil
	case category.Search:
		return p.Limits.SearchLimit, nil
	case category.Indices:
		return p.Limits.IndicesLimit, nil
	case category.Cat:
		return p.Limits.CatLimit, nil
	case category.Clusters:
		return p.Limits.ClustersLimit, nil
	case category.Misc:
		return p.Limits.MiscLimit, nil
	case category.User:
		return p.Limits.UserLimit, nil
	case category.Permission:
		return p.Limits.PermissionLimit, nil
	case category.Analytics:
		return p.Limits.AnalyticsLimit, nil
	case category.Rules:
		return p.Limits.RulesLimit, nil
	case category.Templates:
		return p.Limits.TemplatesLimit, nil
	case category.Suggestions:
		return p.Limits.SuggestionsLimit, nil
	case category.Auth:
		return p.Limits.AuthLimit, nil
	case category.Streams:
		return p.Limits.StreamsLimit, nil
	case category.Functions:
		return p.Limits.FunctionsLimit, nil
	default:
		return -1, fmt.Errorf(`we do not rate limit "%s" category`, c)
	}
}

// GetIPLimit returns the IPLimit i.e. the number of requests allowed per IP address per hour.
func (p *Permission) GetIPLimit() int64 {
	return p.Limits.IPLimit
}

// GetPatch generates a patch doc from the non-zero values in the permission.
func (p *Permission) GetPatch(rolePatched bool) (map[string]interface{}, error) {
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
	if rolePatched {
		patch["role"] = p.Role
	}
	if p.Categories != nil {
		patch["categories"] = p.Categories
		if p.ACLs != nil {
			if err := p.ValidateACLs(p.ACLs...); err != nil {
				return nil, err
			}
			patch["acls"] = p.ACLs
		} else {
			patch["acls"] = category.ACLsFor(p.Categories...)
		}
	}
	if p.Ops != nil {
		patch["ops"] = p.Ops
	}
	if p.Indices != nil {
		patch["indices"] = p.Indices
	}
	if p.Sources != nil {
		if err := validateSources(p.Sources); err != nil {
			return nil, err
		}
		patch["sources"] = p.Sources
	}
	if p.Referers != nil {
		if err := validateReferers(p.Referers); err != nil {
			return nil, err
		}
		patch["referers"] = p.Referers
	}
	if p.CreatedAt != "" {
		return nil, errors.NewUnsupportedPatchError("permission", "created_at")
	}
	if p.TTL.String() != "0s" {
		patch["ttl"] = p.TTL
	}
	// Cannot patch individual limits to 0
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
		if p.Limits.UserLimit != 0 {
			limits["user_limit"] = p.Limits.UserLimit
		}
		if p.Limits.PermissionLimit != 0 {
			limits["permission_limit"] = p.Limits.PermissionLimit
		}
		if p.Limits.AnalyticsLimit != 0 {
			limits["analytics_limit"] = p.Limits.AnalyticsLimit
		}
		if p.Limits.RulesLimit != 0 {
			limits["rules_limit"] = p.Limits.RulesLimit
		}
		if p.Limits.TemplatesLimit != 0 {
			limits["templates_limit"] = p.Limits.TemplatesLimit
		}
		if p.Limits.SuggestionsLimit != 0 {
			limits["suggestions_limit"] = p.Limits.SuggestionsLimit
		}
		if p.Limits.StreamsLimit != 0 {
			limits["streams_limit"] = p.Limits.StreamsLimit
		}
		if p.Limits.AuthLimit != 0 {
			limits["auth_limit"] = p.Limits.AuthLimit
		}
		if p.Limits.FunctionsLimit != 0 {
			limits["functions_limit"] = p.Limits.FunctionsLimit
		}

		patch["limits"] = limits
	}
	if p.Description != "" {
		patch["description"] = p.Description
	}
	if p.Includes != nil {
		patch["include_fields"] = p.Includes
	}
	if p.Excludes != nil {
		patch["exclude_fields"] = p.Excludes
	}

	return patch, nil
}

func (p *Permission) Id() string {
	return p.Username
}
