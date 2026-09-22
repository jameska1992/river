package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

// runScopeGuard injects the given claims (as Auth would) then applies
// RequireScope, returning the resulting status.
func runScopeGuard(claims *Claims, required ...string) int {
	r := gin.New()
	r.GET("/x", func(c *gin.Context) {
		c.Set(claimsKey, claims)
		c.Next()
	}, RequireScope(required...), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/x", nil))
	return w.Code
}

func TestRequireScope_AdminAlwaysAllowed(t *testing.T) {
	assert.Equal(t, http.StatusOK, runScopeGuard(&Claims{Role: "admin"}, "settings:tmdb"))
}

func TestRequireScope_LegacyServiceHasFullAccess(t *testing.T) {
	// A password/JWT service principal carries no AuthMethod=key and no scopes;
	// it must retain full service access so a rollout can be staged.
	assert.Equal(t, http.StatusOK, runScopeGuard(&Claims{Role: "service"}, "settings:tmdb"))
}

func TestRequireScope_KeyServiceNeedsScope(t *testing.T) {
	withScope := &Claims{Role: "service", AuthMethod: AuthMethodKey, Scopes: []string{"media:write", "settings:tmdb"}}
	assert.Equal(t, http.StatusOK, runScopeGuard(withScope, "settings:tmdb"))

	withoutScope := &Claims{Role: "service", AuthMethod: AuthMethodKey, Scopes: []string{"media:write"}}
	assert.Equal(t, http.StatusForbidden, runScopeGuard(withoutScope, "settings:tmdb"))
}

func TestRequireScope_KeyServiceAnyOfRequired(t *testing.T) {
	c := &Claims{Role: "service", AuthMethod: AuthMethodKey, Scopes: []string{"jobs:write"}}
	assert.Equal(t, http.StatusOK, runScopeGuard(c, "logs:write", "jobs:write"))
}

func TestRequireScope_UserDenied(t *testing.T) {
	assert.Equal(t, http.StatusForbidden, runScopeGuard(&Claims{Role: "user"}, "media:write"))
}

// fakeKeyAuth implements ServiceKeyAuthenticator for the Auth middleware test.
type fakeKeyAuth struct {
	name   string
	scopes []string
	ok     bool
	sawKey string
}

func (f *fakeKeyAuth) AuthenticateServiceKey(raw string) (string, []string, bool) {
	f.sawKey = raw
	return f.name, f.scopes, f.ok
}

func runAuth(secret string, keyAuth ServiceKeyAuthenticator, bearer string) (int, *Claims) {
	r := gin.New()
	var captured *Claims
	r.GET("/x", Auth(secret, keyAuth), func(c *gin.Context) {
		captured = GetClaims(c)
		c.Status(http.StatusOK)
	})
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w.Code, captured
}

func TestAuth_ValidServiceKey(t *testing.T) {
	ka := &fakeKeyAuth{name: "river-meta-movie", scopes: []string{"settings:tmdb"}, ok: true}
	code, claims := runAuth("secret", ka, "rvk_abc123")
	assert.Equal(t, http.StatusOK, code)
	assert.Equal(t, "rvk_abc123", ka.sawKey)
	if assert.NotNil(t, claims) {
		assert.Equal(t, "service", claims.Role)
		assert.Equal(t, AuthMethodKey, claims.AuthMethod)
		assert.Equal(t, "river-meta-movie", claims.Username)
		assert.Equal(t, []string{"settings:tmdb"}, claims.Scopes)
	}
}

func TestAuth_InvalidServiceKeyRejected(t *testing.T) {
	ka := &fakeKeyAuth{ok: false}
	code, _ := runAuth("secret", ka, "rvk_bogus")
	assert.Equal(t, http.StatusUnauthorized, code, "a key-shaped token that fails must 401, not fall through to JWT")
}

func TestAuth_NonKeyTokenParsedAsJWT(t *testing.T) {
	// A non-rvk_ token is treated as a JWT; a garbage one fails validation.
	ka := &fakeKeyAuth{ok: true}
	code, _ := runAuth("secret", ka, "not-a-key")
	assert.Equal(t, http.StatusUnauthorized, code)
	assert.Empty(t, ka.sawKey, "non-key token must not hit the key authenticator")
}
