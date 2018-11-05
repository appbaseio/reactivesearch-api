package user

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/appbaseio-confidential/arc/internal/errors"
	"github.com/appbaseio-confidential/arc/internal/types/acl"
	"github.com/appbaseio-confidential/arc/internal/types/category"
	"github.com/appbaseio-confidential/arc/internal/types/op"
)

type contextKey string

const (
	// Credential is a value stored against request.Crdential key in the context.
	// It basically acts as an identifier that tells whether the request uses user
	// credentials.
	Credential = contextKey("user_credential")

	// CtxKey is a key against which a *User is stored in the context.
	CtxKey = contextKey("user")

	// IndexMapping for the index that houses the user data.
	IndexMapping = `{ "settings" : { "number_of_shards" : 3, "number_of_replicas" : 2 } }`
)

// User defines a user type.
type User struct {
	Username   string              `json:"username"`
	Password   string              `json:"password"`
	IsAdmin    *bool               `json:"is_admin"`
	ACLs       []acl.ACL           `json:"acls"`
	Email      string              `json:"email"`
	Ops        []op.Operation      `json:"ops"`
	Categories []category.Category `json:"categories"`
	Indices    []string            `json:"indices"`
	CreatedAt  string              `json:"created_at"`
}

// Options is a function type used to define a user's properties.
type Options func(u *User) error

// SetIsAdmin defines whether a user is an admin or not.
func SetIsAdmin(isAdmin bool) Options {
	return func(u *User) error {
		u.IsAdmin = &isAdmin
		return nil
	}
}

// SetACLs sets the acls a user can have access to.
// ACLs must always be set before setting the Categories.
func SetACLs(acls []acl.ACL) Options {
	return func(u *User) error {
		if acls == nil {
			return errors.NilACLsError
		}
		u.ACLs = acls
		return nil
	}
}

// SetCategories sets the categories a user can have access to.
// Categories must always be set after setting the acls.
func SetCategories(categories []category.Category) Options {
	return func(u *User) error {
		if categories == nil {
			return errors.ErrNilCategories
		}

		if err := u.ValidateCategories(categories...); err != nil {
			return err
		}

		u.Categories = categories
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

// SetOps sets the operations that a user can perform.
func SetOps(ops []op.Operation) Options {
	return func(u *User) error {
		if ops == nil {
			return errors.NilOpsError
		}
		u.Ops = ops
		return nil
	}
}

// SetIndices sets the indices or index patterns a user can have access to.
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
		ACLs:      defaultACLs,
		Ops:       defaultOps,
		Indices:   []string{},
		CreatedAt: time.Now().Format(time.RFC3339),
	}

	// run the options on it
	for _, option := range opts {
		if err := option(u); err != nil {
			return nil, err
		}
	}

	// set the categories if not set by Options explicitly
	if u.Categories == nil {
		u.Categories = acl.CategoriesFor(u.ACLs...)
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
		Username:  username,
		Password:  password,
		IsAdmin:   &isAdminTrue,
		ACLs:      adminACLs,
		Ops:       adminOps,
		Indices:   []string{"*"},
		CreatedAt: time.Now().Format(time.RFC3339),
	}

	// run the options on it
	for _, option := range opts {
		if err := option(u); err != nil {
			return nil, err
		}
	}

	// set the categories if not set by Options explicitly
	if u.Categories == nil {
		u.Categories = acl.CategoriesFor(u.ACLs...)
	}

	return u, nil
}

// FromContext retrieves the *user.User stored against user.CtxKey from the context.
func FromContext(ctx context.Context) (*User, error) {
	ctxUser := ctx.Value(CtxKey)
	if ctxUser == nil {
		return nil, errors.NewNotFoundInRequestContextError("*user.User")
	}
	reqUser, ok := ctxUser.(*User)
	if !ok {
		return nil, errors.NewInvalidCastError("ctxUser", "*user.User")
	}
	return reqUser, nil
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

func (u *User) hasACLForCategory(category category.Category) bool {
	for _, acl := range u.ACLs {
		if acl.HasCategory(category) {
			return true
		}
	}
	return false
}

// ValidateCategories checks if the user can possess the given set of categories.
func (u *User) ValidateCategories(categories ...category.Category) error {
	for _, c := range categories {
		if !u.hasACLForCategory(c) {
			return fmt.Errorf(`user doesn't have acls to access "%s" category`, c)
		}
	}
	return nil
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

// CanDo checks whether the user is permitted to do the given operation.
func (u *User) CanDo(op op.Operation) bool {
	for _, o := range u.Ops {
		if o == op {
			return true
		}
	}
	return false
}

// CanAccessIndex checks whether the user has access to the given index or index pattern.
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
	}
	if u.Email != "" {
		patch["email"] = u.Email
	}
	if u.ACLs != nil {
		patch["acls"] = u.ACLs
		if u.Categories != nil {
			if err := u.ValidateCategories(u.Categories...); err != nil {
				return nil, err
			}
		} else {
			patch["categories"] = acl.CategoriesFor(u.ACLs...)
		}
	}
	if u.Categories != nil {
		patch["categories"] = u.Categories
	}
	if u.Ops != nil {
		patch["ops"] = u.Ops
	}
	if u.Indices != nil {
		patch["indices"] = u.Indices
	}

	return patch, nil
}
