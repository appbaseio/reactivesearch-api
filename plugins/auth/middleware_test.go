package auth

import (
	"time"
	"context"
	"net/http"
	"testing"
	"net/http/httptest"
	"github.com/appbaseio/arc/model/credential"
	"github.com/appbaseio/arc/model/user"
	"github.com/appbaseio/arc/model/permission"
	"github.com/appbaseio/arc/model/category"
	"github.com/appbaseio/arc/model/op"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/assert"
	"crypto/rsa"
	"crypto/rand"
	"github.com/dgrijalva/jwt-go"
	"golang.org/x/crypto/bcrypt"
)

type mockAuthService struct {
	mock.Mock
}

func (m *mockAuthService) getCredential(ctx context.Context, username string) (credential.AuthCredential, error) {
	args := m.Called(ctx, username)
	v := args.Get(0)
	if v == nil {
		return nil, args.Error(1)
	} else {
		return v.(credential.AuthCredential), args.Error(1)
	}
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
	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte("bar"), bcrypt.DefaultCost)
	u, _ := user.New("foo", string(hashedPassword))
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
	mas.On("getCredential", ctx, "foo").Return(u, nil)

	Instance().es = mas

	BasicAuth()(ehf)(recorder, request)
	assert.Equal(t, http.StatusOK, recorder.Result().StatusCode)
	
	mas.AssertExpectations(t)
}

func TestBasicAuthWithUserPasswordWithoutUser(t *testing.T) {
	ehf := func (_ http.ResponseWriter, req *http.Request) {
		assert.Fail(t, "Should not be run")
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
	mas.On("getCredential", ctx, "user2").Return(nil, nil)

	Instance().es = mas

	BasicAuth()(ehf)(recorder, request)

	mas.AssertExpectations(t)
	assert.Equal(t, http.StatusUnauthorized, recorder.Result().StatusCode)
	aU, _ := user.FromContext(request.Context())
	assert.Nil(t, aU)
}

func TestBasicAuthWithUserWrongPassword(t *testing.T) {
	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte("bar"), bcrypt.DefaultCost)
	u, _ := user.New("user3", string(hashedPassword))
	ehf := func (_ http.ResponseWriter, request *http.Request) {
		assert.Fail(t, "Should not be run")
	}

	c := new(category.Category)
	*c = category.User
	ctx := category.NewContext(context.Background(), c)

	oper := new(op.Operation)
	*oper = op.Read
	ctx = op.NewContext(ctx, oper)

	request := httptest.NewRequest("GET", "/", nil)
	request = request.WithContext(ctx)
	request.SetBasicAuth("user3", "bar2")
	recorder := httptest.NewRecorder()

	mas := new(mockAuthService)
	mas.On("getCredential", ctx, "user3").Return(u, nil)

	Instance().es = mas

	BasicAuth()(ehf)(recorder, request)
	mas.AssertExpectations(t)
	assert.Equal(t, http.StatusUnauthorized, recorder.Result().StatusCode)
	aU, _ := user.FromContext(request.Context())
	assert.Nil(t, aU)
	assert.NotEqual(t, u, aU)
}

func TestBasicAuthTwoRequests(t *testing.T) {
	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte("bar"), bcrypt.DefaultCost)
	u, _ := user.New("user4", string(hashedPassword))
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

	mas := new(mockAuthService)
	mas.On("getCredential", ctx, "user4").Return(u, nil)
	Instance().es = mas

	request := httptest.NewRequest("GET", "/", nil)
	request = request.WithContext(ctx)
	request.SetBasicAuth("user4", "bar")
	recorder := httptest.NewRecorder()


	BasicAuth()(ehf)(recorder, request)
	assert.Equal(t, http.StatusOK, recorder.Result().StatusCode)

	ehf = func (_ http.ResponseWriter, request *http.Request) {
		assert.Fail(t, "Should not be run")
	}
	request = httptest.NewRequest("GET", "/", nil)
	request = request.WithContext(ctx)
	request.SetBasicAuth("user4", "bar2")
	recorder = httptest.NewRecorder()
	BasicAuth()(ehf)(recorder, request)
	assert.Equal(t, http.StatusUnauthorized, recorder.Result().StatusCode)
	aU, _ := user.FromContext(request.Context())
	assert.Nil(t, aU)

	mas.AssertExpectations(t)
}


func TestBasicAuthWithJWToken(t *testing.T) {
	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte("bar"), bcrypt.DefaultCost)
	u, _ := user.New("jwtUser", string(hashedPassword))
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
	mas.On("getCredential", ctx, "jwtUser").Return(u, nil)

	Instance().es = mas
	Instance().jwtRsaPublicKey = &pvt.PublicKey

	BasicAuth()(ehf)(recorder, request)
	mas.AssertExpectations(t)
	assert.Equal(t, http.StatusOK, recorder.Result().StatusCode)
}

func TestBasicAuthWithJWTokenWithoutUser(t *testing.T) {
	ehf := func (_ http.ResponseWriter, req *http.Request) {
		assert.Fail(t, "Should not be run")
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
	mas.On("getCredential", ctx, "jwtUser2").Return(nil, nil)

	Instance().es = mas
	Instance().jwtRsaPublicKey = &pvt.PublicKey

	BasicAuth()(ehf)(recorder, request)
	mas.AssertExpectations(t)
	assert.Equal(t, http.StatusUnauthorized, recorder.Result().StatusCode)
	aU, _ := user.FromContext(request.Context())
	assert.Nil(t, aU)
}
