package user

import (
	"context"
	"fmt"
	"net"
	"regexp"
	"strings"
	"time"

	"github.com/appbaseio/reactivesearch-api/errors"
	"github.com/appbaseio/reactivesearch-api/model/acl"
	"github.com/appbaseio/reactivesearch-api/model/category"
	"github.com/appbaseio/reactivesearch-api/util"
)

type contextKey string

const (
	// Credential is a value stored against request.Credential key in the context.
	// It basically acts as an identifier that tells whether the request uses user
	// credentials.
	Credential = contextKey("user_credential")

	// ctxKey is a key against which a *User is stored in the context.
	ctxKey = contextKey("user")
)

// User defines a user type.
type User struct {
	Username         string              `json:"username"`
	Password         string              `json:"password"`
	PasswordHashType string              `json:"password_hash_type"`
	IsAdmin          *bool               `json:"is_admin"`
	Categories       []category.Category `json:"categories"`
	AllowedActions   *[]UserAction       `json:"allowed_actions"`
	ACLs             []acl.ACL           `json:"acls"`
	Email            string              `json:"email"`
	Indices          []string            `json:"indices"`
	CreatedAt        string              `json:"created_at"`
	Sources          *[]string           `json:"sources"`
}

// Options is a function type used to define a user's properties.
type Options func(u *User) error

// SetIsAdmin defines whether a user is an admin or not.
// It sets the default actions for admin users
func SetIsAdmin(isAdmin bool) Options {
	return func(u *User) error {
		u.IsAdmin = &isAdmin
		// Set default actions and categories for admin users
		if *u.IsAdmin {
			u.AllowedActions = &adminActions
			u.Categories = GetCategories(adminActions)
		}
		return nil
	}
}

// SetAllowedActions sets the actions a user can have access to.
// It also sets the categories based on the allowed actions
// Categories must always be set before setting the ACLs.
func SetAllowedActions(actions []UserAction) Options {
	return func(u *User) error {
		if actions == nil {
			return errors.ErrNilCategories
		}
		// Set admin actions to admin users and ignore the actions from request body
		if *u.IsAdmin {
			u.AllowedActions = &adminActions
			u.Categories = GetCategories(adminActions)
		} else {
			u.AllowedActions = &actions
			u.Categories = GetCategories(actions)
		}
		return nil
	}
}

// SetSources sets the sources from which the permission can make request from.
// Sources are accepted and parsed in CIDR notation.
func SetSources(sources []string) Options {
	return func(u *User) error {
		if sources == nil {
			return errors.ErrNilSources
		}
		if err := validateSources(sources); err != nil {
			return err
		}
		u.Sources = &sources
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

// SetACLs sets the acls a user can have access to.
// ACLs must always be set after setting the Categories.
func SetACLs(acls []acl.ACL) Options {
	return func(u *User) error {
		if acls == nil {
			return errors.ErrNilACLs
		}
		if err := u.ValidateACLs(acls...); err != nil {
			return err
		}
		u.ACLs = acls
		return nil
	}
}

// SetEmail sets the user email.
func SetEmail(email string) Options {
	return func(u *User) error {
		u.Email = email
		return nil
	}
}

// SetIndices sets the indices or index patterns a user can have access to.
func SetIndices(indices []string) Options {
	return func(u *User) error {
		if indices == nil {
			return errors.ErrNilIndices
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

// New creates a new user by running the Options on it. It returns a default user
// in case no Options are provided.
func New(username, password string, opts ...Options) (*User, error) {
	if username == "" {
		return nil, fmt.Errorf("username cannot be an empty string")
	}

	// create a default user
	u := &User{
		Username:  username,
		Password:  password,
		IsAdmin:   &isAdminFalse, // pointer to bool
		Indices:   []string{},
		CreatedAt: time.Now().Format(time.RFC3339),
		Sources:   &defaultSources,
	}

	// run the options on it
	for _, option := range opts {
		if err := option(u); err != nil {
			return nil, err
		}
	}

	// set the acls if not set by Options explicitly
	if u.ACLs == nil {
		u.ACLs = category.ACLsFor(u.Categories...)
	}

	return u, nil
}

// NewAdmin create a new user by running the Options on it. It returns a
// user with admin defaults in case no Options are provided.
func NewAdmin(username, password string, opts ...Options) (*User, error) {
	if username == "" {
		return nil, fmt.Errorf("username cannot be an empty field")
	}
	// create an admin user
	u := &User{
		Username:       username,
		Password:       password,
		IsAdmin:        &isAdminTrue,
		Categories:     GetCategories(adminActions),
		AllowedActions: &adminActions,
		Indices:        []string{"*"},
		Sources:        &defaultSources,
		CreatedAt:      time.Now().Format(time.RFC3339),
	}

	// run the options on it
	for _, option := range opts {
		if err := option(u); err != nil {
			return nil, err
		}
	}

	// set the acls if not set by Options explicitly
	if u.ACLs == nil {
		u.ACLs = category.ACLsFor(u.Categories...)
	}

	return u, nil
}

// NewContext returns the context with the given User.
func NewContext(ctx context.Context, u *User) context.Context {
	return context.WithValue(ctx, ctxKey, u)
}

// FromContext retrieves the *user.User stored against user.CtxKey from the context.
func FromContext(ctx context.Context) (*User, error) {
	ctxUser := ctx.Value(ctxKey)
	if ctxUser == nil {
		return nil, errors.NewNotFoundInContextError("*user.User")
	}
	reqUser, ok := ctxUser.(*User)
	if !ok {
		return nil, errors.NewInvalidCastError("ctxUser", "*user.User")
	}
	return reqUser, nil
}

// HasCategory checks whether the user has access to the given category.
func (u *User) HasCategory(category category.Category) bool {
	for _, c := range u.Categories {
		if c == category {
			return true
		}
	}
	return false
}

func (u *User) HasAction(action UserAction) bool {
	for _, c := range *u.AllowedActions {
		if c == action {
			return true
		}
	}
	return false
}

func (u *User) hasCategoryForACL(acl acl.ACL) bool {
	for _, c := range u.Categories {
		if c.HasACL(acl) {
			return true
		}
	}
	return false
}

// ValidateACLs checks if the user can possess the given set of acls.
func (u *User) ValidateACLs(acls ...acl.ACL) error {
	for _, a := range acls {
		if !u.hasCategoryForACL(a) {
			return fmt.Errorf(`user doesn't have category to access "%s" acl`, a)
		}
	}
	return nil
}

// HasACL checks whether the user has access to the given acl.
func (u *User) HasACL(acl acl.ACL) bool {
	for _, a := range u.ACLs {
		if a == acl {
			return true
		}
	}
	return false
}

// CanAccessCluster checks whether the user can access cluster level routes.
func (u *User) CanAccessCluster() (bool, error) {
	for _, pattern := range u.Indices {
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

// CanAccessIndex checks whether the user has access to the given index or index pattern.
func (u *User) CanAccessIndex(name string) (bool, error) {
	for _, pattern := range u.Indices {
		matched, err := util.ValidateIndex(pattern, name)
		if err != nil {
			return false, err
		}
		if matched {
			return true, nil
		}
	}
	return false, nil
}

// CanAccessIndices checks whether the user has access to the given indices.
func (u *User) CanAccessIndices(indices ...string) (bool, error) {
	for _, index := range indices {
		if ok, err := u.CanAccessIndex(index); !ok || err != nil {
			return ok, err
		}
	}
	return true, nil
}

// GetPatch generates a patch doc from the non-zero fields set in the user.
func (u *User) GetPatch() (map[string]interface{}, error) {
	patch := make(map[string]interface{})

	if u.Username != "" {
		patch["username"] = u.Username
	}
	if u.Password != "" {
		patch["password"] = u.Password
	}
	if u.IsAdmin != nil {
		patch["is_admin"] = u.IsAdmin
		if *u.IsAdmin {
			categories := GetCategories(adminActions)
			// assign the admin actions
			patch["allowed_actions"] = adminActions
			patch["categories"] = categories
			patch["acls"] = category.ACLsFor(categories...)
		}
	}
	if u.Sources != nil {
		if err := validateSources(*u.Sources); err != nil {
			return nil, err
		}
		patch["sources"] = *u.Sources
	}
	if u.AllowedActions != nil {
		categories := GetCategories(*u.AllowedActions)
		patch["allowed_actions"] = *u.AllowedActions
		patch["categories"] = categories
		patch["acls"] = category.ACLsFor(categories...)
	}
	if u.Email != "" {
		patch["email"] = u.Email
	}
	if u.Indices != nil {
		patch["indices"] = u.Indices
	}
	if u.CreatedAt != "" {
		return nil, errors.NewUnsupportedPatchError("user", "created_at")
	}

	return patch, nil
}

func (u *User) Id() string {
	return u.Username
}

// GetCategories extracts the categories from the actions
func GetCategories(actions []UserAction) []category.Category {
	// Use map to avoid duplicates
	var categoriesMap = make(map[category.Category]bool)
	for _, v := range actions {
		for _, category := range ActionToCategories[v] {
			categoriesMap[category] = true
		}
	}
	var categories = make([]category.Category, 0)
	for category := range categoriesMap {
		categories = append(categories, category)
	}
	return categories
}
