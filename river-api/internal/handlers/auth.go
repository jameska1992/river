package handlers

import (
	"net/http"

	"river-api/internal/middleware"
	"river-api/internal/services"

	"github.com/gin-gonic/gin"
)

type AuthHandler struct {
	svc *services.AuthService
}

func NewAuthHandler(svc *services.AuthService) *AuthHandler {
	return &AuthHandler{svc: svc}
}

type registerRequest struct {
	Username string `json:"username" binding:"required,min=3,max=32"`
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=8"`
}

// Register creates a new user account. The first user to register is
// promoted to admin automatically; subsequent registrations are regular
// users.
//
// @Summary      Register a new user
// @Description  Public endpoint. The first registered user becomes admin.
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        body  body      registerRequest  true  "Registration details"
// @Success      201   {object}  models.User
// @Failure      400   {object}  map[string]string
// @Failure      409   {object}  map[string]string  "username/email already in use"
// @Router       /auth/register [post]
func (h *AuthHandler) Register(c *gin.Context) {
	var req registerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	user, err := h.svc.Register(req.Username, req.Email, req.Password)
	if err != nil {
		c.JSON(serviceStatus(err), gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, user)
}

type loginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// Login exchanges username + password for an access token, refresh token,
// and stream token (the long-lived one used in <video> src URLs).
//
// @Summary      Login
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        body  body      loginRequest  true  "Credentials"
// @Success      200   {object}  map[string]interface{}  "{access_token, refresh_token, stream_token, user}"
// @Failure      400   {object}  map[string]string
// @Failure      401   {object}  map[string]string  "invalid credentials"
// @Router       /auth/login [post]
func (h *AuthHandler) Login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	result, err := h.svc.Login(req.Username, req.Password)
	if err != nil {
		c.JSON(serviceStatus(err), gin.H{"error": "invalid credentials"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"access_token":  result.AccessToken,
		"refresh_token": result.RefreshToken,
		"stream_token":  result.StreamToken,
		"user":          result.User,
	})
}

type tokenRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

// Refresh rotates a refresh token, returning a new access/refresh/stream
// token triplet. The submitted refresh token is revoked.
//
// @Summary      Refresh access token
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        body  body      tokenRequest  true  "Refresh token"
// @Success      200   {object}  map[string]string  "{access_token, refresh_token, stream_token}"
// @Failure      400   {object}  map[string]string
// @Failure      401   {object}  map[string]string  "invalid or expired refresh token"
// @Router       /auth/refresh [post]
func (h *AuthHandler) Refresh(c *gin.Context) {
	var req tokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	pair, err := h.svc.Refresh(req.RefreshToken)
	if err != nil {
		c.JSON(serviceStatus(err), gin.H{"error": "invalid or expired refresh token"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"access_token":  pair.AccessToken,
		"refresh_token": pair.RefreshToken,
		"stream_token":  pair.StreamToken,
	})
}

// Logout revokes the supplied refresh token. Idempotent — returns 200
// even if the token is already gone.
//
// @Summary      Logout
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        body  body      tokenRequest  true  "Refresh token to revoke"
// @Success      200   {object}  map[string]string
// @Failure      400   {object}  map[string]string
// @Router       /auth/logout [post]
func (h *AuthHandler) Logout(c *gin.Context) {
	var req tokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	h.svc.Logout(req.RefreshToken)
	c.JSON(http.StatusOK, gin.H{"message": "logged out"})
}

// Me returns the currently authenticated user.
//
// @Summary      Get current user
// @Tags         auth
// @Produce      json
// @Success      200  {object}  models.User
// @Failure      401  {object}  map[string]string
// @Security     BearerAuth
// @Router       /auth/me [get]
func (h *AuthHandler) Me(c *gin.Context) {
	claims := middleware.GetClaims(c)
	user, err := h.svc.GetUser(claims.UserID)
	if err != nil {
		c.JSON(serviceStatus(err), gin.H{"error": "user not found"})
		return
	}
	c.JSON(http.StatusOK, user)
}

type updateMeRequest struct {
	Email string `json:"email" binding:"required,email"`
}

// UpdateMe updates the current user's email.
//
// @Summary      Update current user
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        body  body      updateMeRequest  true  "New email"
// @Success      200   {object}  models.User
// @Failure      400   {object}  map[string]string
// @Failure      401   {object}  map[string]string
// @Failure      409   {object}  map[string]string
// @Security     BearerAuth
// @Router       /auth/me [put]
func (h *AuthHandler) UpdateMe(c *gin.Context) {
	claims := middleware.GetClaims(c)
	var req updateMeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	user, err := h.svc.UpdateMe(claims.UserID, req.Email)
	if err != nil {
		c.JSON(serviceStatus(err), gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, user)
}

type changePasswordRequest struct {
	CurrentPassword string `json:"current_password" binding:"required"`
	NewPassword     string `json:"new_password"     binding:"required,min=8"`
}

// ChangePassword updates the current user's password after verifying the
// current one.
//
// @Summary      Change password
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        body  body  changePasswordRequest  true  "Current + new password"
// @Success      204
// @Failure      400  {object}  map[string]string
// @Failure      401  {object}  map[string]string  "current password incorrect"
// @Security     BearerAuth
// @Router       /auth/me/password [post]
func (h *AuthHandler) ChangePassword(c *gin.Context) {
	claims := middleware.GetClaims(c)
	var req changePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.svc.ChangePassword(claims.UserID, req.CurrentPassword, req.NewPassword); err != nil {
		c.JSON(serviceStatus(err), gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}
