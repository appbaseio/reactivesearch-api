package auth

import (
	"time"
	"context"
	"net/http"
	"testing"
	"net/http/httptest"
	"github.com/appbaseio-confidential/arc/model/user"
	"github.com/appbaseio-confidential/arc/model/permission"
	"github.com/appbaseio-confidential/arc/model/category"
	//"fmt"
	"github.com/appbaseio-confidential/arc/model/op"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/assert"
	"crypto/rsa"
	"crypto/rand"
	"github.com/dgrijalva/jwt-go"
	//"errors"
)

type mockAuthService struct {
	mock.Mock
}

func (m *mockAuthService) getCredential(ctx context.Context, username, password string, checkPassword bool) (interface{}, error) {
	args := m.Called(ctx, username, password, checkPassword)
	return args.Get(0), args.Error(1)
}

func (m *mockAuthService) putUser(ctx context.Context, u user.User) (bool, error) {
	args := m.Called(ctx, u)
	return args.Bool(0), args.Error(1)
}

func (m *mockAuthService) getUser(ctx context.Context, username string) (*user.User, error){
	args := m.Called(ctx, username)
	return args.Get(0).(*user.User), args.Error(1)
}
func (m *mockAuthService) getRawUser(ctx context.Context, username string) ([]byte, error) {
	args := m.Called(ctx, username)
	return args.Get(0).([]byte), args.Error(1)
}
func (m *mockAuthService) putPermission(ctx context.Context, p permission.Permission) (bool, error) {
	args := m.Called(ctx, p)
	return args.Bool(0), args.Error(1)
}
func (m *mockAuthService) getPermission(ctx context.Context, username string) (*permission.Permission, error) {
	args := m.Called(ctx, username)
	return args.Get(0).(*permission.Permission), args.Error(1)
}
func (m *mockAuthService) getRawPermission(ctx context.Context, username string) ([]byte, error) {
	args := m.Called(ctx, username)
	return args.Get(0).([]byte), args.Error(1)
}

func TestBasicAuthWithUserPasswordBasic(t *testing.T) {
	u, _ := user.New("foo", "bar")
	ehf := func (_ http.ResponseWriter, req *http.Request) {
		aU, _ := user.FromContext(req.Context())
		assert.Equal(t, u, aU)
	}

	c := new(category.Category)
	*c = category.User
	ctx := category.NewContext(context.Background(), c)

	oper := new(op.Operation)
	*oper = op.Read
	ctx = op.NewContext(ctx, oper)

	request := httptest.NewRequest("GET", "/", nil)
	request = request.WithContext(ctx)
	request.SetBasicAuth("foo", "bar")
	recorder := httptest.NewRecorder()

	mas := new(mockAuthService)
	mas.On("getCredential", ctx, "foo", "bar", true).Return(u, nil)

	Instance().es = mas

	BasicAuth()(ehf)(recorder, request)
	mas.AssertExpectations(t)
	assert.Equal(t, http.StatusOK, recorder.Result().StatusCode)
}

func TestBasicAuthWithUserPasswordWithoutUser(t *testing.T) {
	ehf := func (_ http.ResponseWriter, req *http.Request) {
		aU, _ := user.FromContext(req.Context())
		assert.Nil(t, aU)
	}

	c := new(category.Category)
	*c = category.User
	ctx := category.NewContext(context.Background(), c)

	oper := new(op.Operation)
	*oper = op.Read
	ctx = op.NewContext(ctx, oper)

	request := httptest.NewRequest("GET", "/", nil)
	request = request.WithContext(ctx)
	request.SetBasicAuth("user2", "bar")
	recorder := httptest.NewRecorder()

	mas := new(mockAuthService)
	mas.On("getCredential", ctx, "user2", "bar", true).Return(nil, nil)

	Instance().es = mas

	BasicAuth()(ehf)(recorder, request)

	mas.AssertExpectations(t)
	assert.Equal(t, http.StatusUnauthorized, recorder.Result().StatusCode)
}


func TestBasicAuthWithJWToken(t *testing.T) {
	u, _ := user.New("jwtUser", "bar")
	ehf := func (_ http.ResponseWriter, req *http.Request) {
		aU, _ := user.FromContext(req.Context())
		assert.Equal(t, u, aU)
	}

	c := new(category.Category)
	*c = category.User
	ctx := category.NewContext(context.Background(), c)

	oper := new(op.Operation)
	*oper = op.Read
	ctx = op.NewContext(ctx, oper)

	request := httptest.NewRequest("GET", "/", nil)
	request = request.WithContext(ctx)
	recorder := httptest.NewRecorder()

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
		    "username": "jwtUser",
		        "iat": time.Now().Unix(),
			"exp": time.Now().Unix() + 1000,
		})
	pvt, _ := rsa.GenerateKey(rand.Reader, 2048)
	tokenString, _ := token.SignedString(pvt)
	tokenString = "Bearer " + tokenString
	request.Header.Add("Authorization", tokenString)

	mas := new(mockAuthService)
	mas.On("getCredential", ctx, "jwtUser", "", false).Return(u, nil)

	Instance().es = mas
	Instance().jwtRsaPublicKey = &pvt.PublicKey

	BasicAuth()(ehf)(recorder, request)
	mas.AssertExpectations(t)
	assert.Equal(t, http.StatusOK, recorder.Result().StatusCode)
}

func TestBasicAuthWithJWTokenWithoutUser(t *testing.T) {
	ehf := func (_ http.ResponseWriter, req *http.Request) {
		aU, _ := user.FromContext(req.Context())
		assert.Nil(t, aU)
	}

	c := new(category.Category)
	*c = category.User
	ctx := category.NewContext(context.Background(), c)

	oper := new(op.Operation)
	*oper = op.Read
	ctx = op.NewContext(ctx, oper)

	request := httptest.NewRequest("GET", "/", nil)
	request = request.WithContext(ctx)
	recorder := httptest.NewRecorder()

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
		    "username": "jwtUser2",
		        "iat": time.Now().Unix(),
			"exp": time.Now().Unix() + 1000,
		})
	pvt, _ := rsa.GenerateKey(rand.Reader, 2048)
	tokenString, _ := token.SignedString(pvt)
	tokenString = "Bearer " + tokenString
	request.Header.Add("Authorization", tokenString)

	mas := new(mockAuthService)
	mas.On("getCredential", ctx, "jwtUser2", "", false).Return(nil, nil)

	Instance().es = mas
	Instance().jwtRsaPublicKey = &pvt.PublicKey

	BasicAuth()(ehf)(recorder, request)
	mas.AssertExpectations(t)
	assert.Equal(t, http.StatusUnauthorized, recorder.Result().StatusCode)
}
