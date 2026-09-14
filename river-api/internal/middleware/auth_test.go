package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func init() { gin.SetMode(gin.TestMode) }

// runGuard runs a handler chain that injects claims for the given role (as
// the Auth middleware would) and then applies guard, returning the status.
func runGuard(role string, guard gin.HandlerFunc) int {
	r := gin.New()
	r.GET("/x", func(c *gin.Context) {
		c.Set(claimsKey, &Claims{Role: role})
		c.Next()
	}, guard, func(c *gin.Context) {
		c.Status(http.StatusOK)
	})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/x", nil))
	return w.Code
}

func TestAdminOrService(t *testing.T) {
	cases := map[string]int{
		"admin":   http.StatusOK,
		"service": http.StatusOK,
		"user":    http.StatusForbidden,
	}
	for role, want := range cases {
		assert.Equal(t, want, runGuard(role, AdminOrService()), "role=%q", role)
	}
}

func TestAdminOnly_RejectsService(t *testing.T) {
	// A service principal must NOT pass the admin-only gate — that's the
	// whole point: the destructive/admin surface stays admin-exclusive.
	assert.Equal(t, http.StatusForbidden, runGuard("service", AdminOnly()))
	assert.Equal(t, http.StatusOK, runGuard("admin", AdminOnly()))
}
