package middleware

import (
	"net/http"
	"slices"
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
	// AuthMethod records how the principal authenticated: "" / "jwt" for a
	// bearer JWT (human users and the legacy service-account password login),
	// or AuthMethodKey for a per-service API key. Scope enforcement only bites
	// key-authed principals, so a legacy password-authed service keeps full
	// service access — that's what lets a deployment upgrade one service at a
	// time without a coordinated cutover.
	AuthMethod string `json:"-"`
	// Scopes carries a key-authed service's granted scopes (empty for JWTs).
	Scopes []string `json:"-"`
	jwt.RegisteredClaims
}

// AuthMethodKey marks a principal that authenticated with a service API key.
const AuthMethodKey = "key"

// ServiceKeyAuthenticator validates a raw bearer token as a service API key.
// Implemented by services.ServiceKeyService; kept as an interface here so the
// middleware package stays free of repository/service dependencies.
type ServiceKeyAuthenticator interface {
	AuthenticateServiceKey(rawKey string) (name string, scopes []string, ok bool)
}

// TokenTypeAccess is the regular short-lived API token. Empty TokenType
// also counts as access for back-compat with tokens minted before this
// field existed.
const TokenTypeAccess = "access"

// TokenTypeStream is the longer-lived token used in <video> src URLs.
// Restricted by the auth middleware to /stream and /download paths.
const TokenTypeStream = "stream"

const claimsKey = "claims"

// serviceKeyPrefix marks a bearer token as a service API key rather than a JWT.
// Must match services.ServiceKeyPrefix (kept local to avoid a middleware→
// services import cycle).
const serviceKeyPrefix = "rvk_"

// Auth authenticates every protected request. It accepts two credential
// kinds on the same Bearer header: a per-service API key (prefix "rvk_",
// validated via keyAuth) or a JWT (everything else — human users and the
// legacy service-account password login). keyAuth may be nil, in which case
// only JWTs are accepted; this keeps the JWT path completely unchanged, so the
// API-key support is purely additive.
func Auth(secret string, keyAuth ServiceKeyAuthenticator) gin.HandlerFunc {
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

		// Service API key path. A key-shaped token is never JWT-parsed; if it
		// doesn't validate it's rejected outright (no silent fall-through).
		if keyAuth != nil && strings.HasPrefix(tokenStr, serviceKeyPrefix) {
			name, scopes, ok := keyAuth.AuthenticateServiceKey(tokenStr)
			if !ok {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid or revoked service key"})
				return
			}
			c.Set(claimsKey, &Claims{
				UserID:     "service:" + name,
				Username:   name,
				Role:       "service",
				AuthMethod: AuthMethodKey,
				Scopes:     scopes,
			})
			c.Next()
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

// AdminOrService gates media-write endpoints that both human admins and the
// internal services (scan / trans / meta, role "service") legitimately call.
// It deliberately does NOT cover the destructive/sensitive surface (user
// management, settings writes, deletes, scan control) — those stay AdminOnly,
// so a compromised service can't escalate beyond writing media records.
func AdminOrService() gin.HandlerFunc {
	return func(c *gin.Context) {
		claims, ok := c.MustGet(claimsKey).(*Claims)
		if !ok || (claims.Role != "admin" && claims.Role != "service") {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "admin or service access required"})
			return
		}
		c.Next()
	}
}

// RequireScope gates an endpoint that the internal services call. Access is
// granted to: any admin; a legacy service principal that authenticated by
// password/JWT (no per-key scopes — retains full service access for a
// non-disruptive rollout); or a key-authed service holding at least one of the
// required scopes. Everyone else is denied. This supersedes AdminOrService on
// the service-facing routes while preserving its behaviour for admins and
// password-authed services.
func RequireScope(required ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		claims, ok := c.MustGet(claimsKey).(*Claims)
		if !ok || claims == nil {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "forbidden"})
			return
		}
		if claims.Role == "admin" {
			c.Next()
			return
		}
		if claims.Role == "service" {
			// Legacy password/JWT service: full access (scopes only apply to
			// keys). A migrated, key-authed service must hold the scope.
			if claims.AuthMethod != AuthMethodKey {
				c.Next()
				return
			}
			for _, r := range required {
				if slices.Contains(claims.Scopes, r) {
					c.Next()
					return
				}
			}
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "service key missing required scope"})
			return
		}
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "admin or service access required"})
	}
}

func GetClaims(c *gin.Context) *Claims {
	v, _ := c.Get(claimsKey)
	claims, _ := v.(*Claims)
	return claims
}
