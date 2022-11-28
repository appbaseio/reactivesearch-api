package permission

import (
	"context"
	"fmt"
	"net"
	"os"
	"regexp"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/appbaseio/reactivesearch-api/errors"
	"github.com/appbaseio/reactivesearch-api/model/acl"
	"github.com/appbaseio/reactivesearch-api/model/category"
	"github.com/appbaseio/reactivesearch-api/model/op"
	"github.com/appbaseio/reactivesearch-api/util"
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

type ReactiveSearchConfig struct {
	MaxSize            *int  `json:"maxSize,omitempty"`
	MaxAggregationSize *int  `json:"maxAggregationSize,omitempty"`
	DisbaleQueryDSL    *bool `json:"disableQueryDSL,omitempty"`
}

// Permission defines a permission type.
type Permission struct {
	Username             string                `json:"username"`
	Password             string                `json:"password"`
	Owner                string                `json:"owner"`
	Creator              string                `json:"creator"`
	Role                 string                `json:"role"`
	Categories           []category.Category   `json:"categories"`
	ACLs                 []acl.ACL             `json:"acls"`
	Ops                  []op.Operation        `json:"ops"`
	Indices              []string              `json:"indices"`
	Pipelines            []string              `json:"pipelines"`
	Sources              []string              `json:"sources"`
	SourcesXffValue      *int                  `json:"sources_xff_value"`
	Referers             []string              `json:"referers"`
	CreatedAt            string                `json:"created_at"`
	TTL                  time.Duration         `json:"ttl"`
	Limits               *Limits               `json:"limits"`
	Description          string                `json:"description"`
	Includes             []string              `json:"include_fields"`
	Excludes             []string              `json:"exclude_fields"`
	Expired              bool                  `json:"expired"`
	ReactiveSearchConfig *ReactiveSearchConfig `json:"reactivesearchConfig,omitempty"`
	UpdatedAt            string                `json:"updated_at"`
}

// Limits defines the rate limits for each category.
type Limits struct {
	IPLimit               int64 `json:"ip_limit"`
	DocsLimit             int64 `json:"docs_limit"`
	SearchLimit           int64 `json:"search_limit"`
	IndicesLimit          int64 `json:"indices_limit"`
	CatLimit              int64 `json:"cat_limit"`
	ClustersLimit         int64 `json:"clusters_limit"`
	MiscLimit             int64 `json:"misc_limit"`
	UserLimit             int64 `json:"user_limit"`
	PermissionLimit       int64 `json:"permission_limit"`
	AnalyticsLimit        int64 `json:"analytics_limit"`
	RulesLimit            int64 `json:"rules_limit"`
	SuggestionsLimit      int64 `json:"suggestions_limit"`
	StreamsLimit          int64 `json:"streams_limit"`
	AuthLimit             int64 `json:"auth_limit"`
	ReactiveSearchLimit   int64 `json:"reactivesearch_limit"`
	SearchRelevancyLimit  int64 `json:"searchrelevancy_limit"`
	SearchGraderLimit     int64 `json:"searchgrader_limit"`
	EcommIntegrationLimit int64 `json:"ecommintegration_limit"`
	LogsLimit             int64 `json:"logs_limit"`
	SynonymsLimit         int64 `json:"synonyms_limit"`
	CacheLimit            int64 `json:"cache_limit"`
	StoredQueryLimit      int64 `json:"storedquery_limit"`
	PipelinesLimit        int64 `json:"pipelines_limit"`
	SyncLimit             int64 `json:"sync_limit"`
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

// SetPipelines sets the pipelines or pipeline pattern a permission can have access to.
func SetPipelines(pipelines []string) Options {
	return func(p *Permission) error {
		if pipelines == nil {
			return errors.ErrNilPipelines
		}
		for _, pattern := range pipelines {
			pattern = strings.Replace(pattern, "*", ".*", -1)
			if _, err := regexp.Compile(pattern); err != nil {
				return err
			}
		}
		p.Pipelines = pipelines
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

// SetIncludes sets the sources_xff_value fields
func SetSourcesXffValue(value *int) Options {
	return func(p *Permission) error {
		p.SourcesXffValue = value
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

func getNormalizedLimit(limit int64, defaultLimit int64) int64 {
	if limit == 0 {
		return defaultLimit
	}
	return limit
}

// SetLimits sets the rate limits for each category in a permission.
func SetLimits(limits *Limits, isAdmin bool) Options {
	return func(p *Permission) error {
		var defaults *Limits
		// Set the default limits for each property if not defined
		if isAdmin {
			defaults = &defaultAdminLimits
		} else {
			defaults = &defaultLimits
		}
		// Todo change
		p.Limits = &Limits{
			IPLimit:               getNormalizedLimit(limits.IPLimit, defaults.IPLimit),
			DocsLimit:             getNormalizedLimit(limits.DocsLimit, defaults.DocsLimit),
			SearchLimit:           getNormalizedLimit(limits.SearchLimit, defaults.SearchLimit),
			IndicesLimit:          getNormalizedLimit(limits.IndicesLimit, defaults.IndicesLimit),
			CatLimit:              getNormalizedLimit(limits.CatLimit, defaults.CatLimit),
			ClustersLimit:         getNormalizedLimit(limits.ClustersLimit, defaults.ClustersLimit),
			MiscLimit:             getNormalizedLimit(limits.MiscLimit, defaults.MiscLimit),
			UserLimit:             getNormalizedLimit(limits.UserLimit, defaults.UserLimit),
			PermissionLimit:       getNormalizedLimit(limits.PermissionLimit, defaults.PermissionLimit),
			AnalyticsLimit:        getNormalizedLimit(limits.AnalyticsLimit, defaults.AnalyticsLimit),
			RulesLimit:            getNormalizedLimit(limits.RulesLimit, defaults.RulesLimit),
			SuggestionsLimit:      getNormalizedLimit(limits.SuggestionsLimit, defaults.SuggestionsLimit),
			StreamsLimit:          getNormalizedLimit(limits.StreamsLimit, defaults.StreamsLimit),
			AuthLimit:             getNormalizedLimit(limits.AuthLimit, defaults.AuthLimit),
			ReactiveSearchLimit:   getNormalizedLimit(limits.ReactiveSearchLimit, defaults.ReactiveSearchLimit),
			SearchRelevancyLimit:  getNormalizedLimit(limits.SearchRelevancyLimit, defaults.SearchRelevancyLimit),
			SearchGraderLimit:     getNormalizedLimit(limits.SearchGraderLimit, defaults.SearchGraderLimit),
			EcommIntegrationLimit: getNormalizedLimit(limits.EcommIntegrationLimit, defaults.EcommIntegrationLimit),
			LogsLimit:             getNormalizedLimit(limits.LogsLimit, defaults.LogsLimit),
			SynonymsLimit:         getNormalizedLimit(limits.SynonymsLimit, defaults.SynonymsLimit),
			CacheLimit:            getNormalizedLimit(limits.CacheLimit, defaults.CacheLimit),
			StoredQueryLimit:      getNormalizedLimit(limits.StoredQueryLimit, defaults.StoredQueryLimit),
			SyncLimit:             getNormalizedLimit(limits.SyncLimit, defaults.SyncLimit),
			PipelinesLimit:        getNormalizedLimit(limits.PipelinesLimit, defaults.PipelinesLimit),
		}
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

// SetDescription sets the permission reactivesearchConfig.
func SetReactivesearchConfig(config ReactiveSearchConfig) Options {
	return func(p *Permission) error {
		p.ReactiveSearchConfig = &config
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
		UpdatedAt:  time.Now().Format(time.RFC3339),
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
		UpdatedAt:  time.Now().Format(time.RFC3339),
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
	indices := p.Indices
	// If permission has suggestions category present then allow access to `.suggestions` index
	if p.HasCategory(category.Suggestions) {
		suggestionsIndex := os.Getenv("SUGGESTIONS_META_ES_INDEX")
		if suggestionsIndex == "" {
			suggestionsIndex = ".suggestions"
		}
		indices = append(indices, suggestionsIndex)
	}
	for _, pattern := range indices {
		matched, err := util.ValidateIndex(pattern, name)
		if err != nil {
			log.Errorln("invalid index regexp", pattern, "encountered: ", err)
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

// CanAccessPipeline checks whether the permission has access to given pipeline or pipeline pattern.
func (p *Permission) CanAccessPipeline(pipeline string) (bool, error) {
	pipelines := p.Pipelines
	for _, pattern := range pipelines {
		matched, err := util.ValidateIndex(pattern, pipeline)
		if err != nil {
			log.Errorln("invalid pipeline regexp", pattern, "encountered: ", err)
			return false, err
		}
		if matched {
			return matched, nil
		}
	}
	return false, nil
}

// CanAccessPipelines checks whether the user has access to the given pipelines.
func (p *Permission) CanAccessPipelines(pipelines ...string) (bool, error) {
	for _, pipeline := range pipelines {
		if ok, err := p.CanAccessPipeline(pipeline); !ok || err != nil {
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
	case category.Suggestions:
		return p.Limits.SuggestionsLimit, nil
	case category.Auth:
		return p.Limits.AuthLimit, nil
	case category.Streams:
		return p.Limits.StreamsLimit, nil
	case category.ReactiveSearch:
		return p.Limits.ReactiveSearchLimit, nil
	case category.SearchRelevancy:
		return p.Limits.SearchRelevancyLimit, nil
	case category.SearchGrader:
		return p.Limits.SearchGraderLimit, nil
	case category.UIBuilder:
		return p.Limits.EcommIntegrationLimit, nil
	case category.Logs:
		return p.Limits.LogsLimit, nil
	case category.Synonyms:
		return p.Limits.SynonymsLimit, nil
	case category.Cache:
		return p.Limits.CacheLimit, nil
	case category.StoredQuery:
		return p.Limits.StoredQueryLimit, nil
	case category.Pipelines:
		return p.Limits.PipelinesLimit, nil
	case category.Sync:
		return p.Limits.SyncLimit, nil
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
	if p.SourcesXffValue != nil {
		patch["sources_xff_value"] = p.SourcesXffValue
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
		if p.Limits.SuggestionsLimit != 0 {
			limits["suggestions_limit"] = p.Limits.SuggestionsLimit
		}
		if p.Limits.StreamsLimit != 0 {
			limits["streams_limit"] = p.Limits.StreamsLimit
		}
		if p.Limits.AuthLimit != 0 {
			limits["auth_limit"] = p.Limits.AuthLimit
		}
		if p.Limits.ReactiveSearchLimit != 0 {
			limits["reactivesearch_limit"] = p.Limits.ReactiveSearchLimit
		}
		if p.Limits.SearchRelevancyLimit != 0 {
			limits["searchrelevancy_limit"] = p.Limits.SearchRelevancyLimit
		}
		if p.Limits.SearchGraderLimit != 0 {
			limits["searchgrader_limit"] = p.Limits.SearchGraderLimit
		}
		if p.Limits.EcommIntegrationLimit != 0 {
			limits["ecommintegration_limit"] = p.Limits.EcommIntegrationLimit
		}
		if p.Limits.LogsLimit != 0 {
			limits["logs_limit"] = p.Limits.LogsLimit
		}
		if p.Limits.SynonymsLimit != 0 {
			limits["synonyms_limit"] = p.Limits.SynonymsLimit
		}
		if p.Limits.CacheLimit != 0 {
			limits["cache_limit"] = p.Limits.CacheLimit
		}
		if p.Limits.StoredQueryLimit != 0 {
			limits["storedquery_limit"] = p.Limits.StoredQueryLimit
		}
		if p.Limits.SyncLimit != 0 {
			limits["sync_limit"] = p.Limits.SyncLimit
		}
		if p.Limits.PipelinesLimit != 0 {
			limits["pipelines_limit"] = p.Limits.PipelinesLimit
		}

		patch["limits"] = limits
	}
	if p.Description != "" {
		patch["description"] = p.Description
	}
	if p.ReactiveSearchConfig != nil {
		patch["reactivesearchConfig"] = p.ReactiveSearchConfig
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
