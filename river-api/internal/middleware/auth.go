package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

type Claims struct {
	UserID   string `json:"user_id"`
	Username string `json:"username"`
	Role     string `json:"role"`
	// TokenType differentiates short-lived API tokens (empty / "access")
	// from longer-lived "stream" tokens used for media playback URLs that
	// the browser can't refresh mid-flight (the <video> element can't send
	// custom auth headers, so the token has to live in the URL). Stream
	// tokens are gated to streaming/download endpoints below, so the
	// longer TTL doesn't widen the blast radius elsewhere.
	TokenType string `json:"token_type,omitempty"`
	jwt.RegisteredClaims
}

// TokenTypeAccess is the regular short-lived API token. Empty TokenType
// also counts as access for back-compat with tokens minted before this
// field existed.
const TokenTypeAccess = "access"

// TokenTypeStream is the longer-lived token used in <video> src URLs.
// Restricted by the auth middleware to /stream and /download paths.
const TokenTypeStream = "stream"

const claimsKey = "claims"

func Auth(secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		var tokenStr string
		if header := c.GetHeader("Authorization"); strings.HasPrefix(header, "Bearer ") {
			tokenStr = strings.TrimPrefix(header, "Bearer ")
		} else if q := c.Query("token"); q != "" {
			tokenStr = q
		} else {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing authorization"})
			return
		}
		claims := &Claims{}

		token, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (any, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, jwt.ErrSignatureInvalid
			}
			return []byte(secret), nil
		})

		if err != nil || !token.Valid {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired token"})
			return
		}

		// Stream tokens only work on stream/download endpoints. An attacker
		// who got hold of one can't use it for the wider API surface.
		if claims.TokenType == TokenTypeStream && !isStreamPath(c.Request.URL.Path) {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "stream token not valid for this endpoint"})
			return
		}

		c.Set(claimsKey, claims)
		c.Next()
	}
}

// isStreamPath returns true for routes that serve media bytes — /stream
// (range-served playback) and /download (whole-file attachment). These
// are the only endpoints that accept stream-type tokens.
func isStreamPath(p string) bool {
	return strings.HasSuffix(p, "/stream") || strings.HasSuffix(p, "/download")
}

func AdminOnly() gin.HandlerFunc {
	return func(c *gin.Context) {
		claims, ok := c.MustGet(claimsKey).(*Claims)
		if !ok || claims.Role != "admin" {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "admin access required"})
			return
		}
		c.Next()
	}
}

func GetClaims(c *gin.Context) *Claims {
	v, _ := c.Get(claimsKey)
	claims, _ := v.(*Claims)
	return claims
}
