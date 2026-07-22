package handlers

import (
	"net/http"

	"river-api/internal/services"

	"github.com/gin-gonic/gin"
)

type AdminUsersHandler struct {
	authSvc     *services.AuthService
	progressSvc *services.ProgressService
}

func NewAdminUsersHandler(authSvc *services.AuthService, progressSvc *services.ProgressService) *AdminUsersHandler {
	return &AdminUsersHandler{authSvc: authSvc, progressSvc: progressSvc}
}

// ListUsers returns every user account.
//
// @Summary      List users
// @Tags         admin-users
// @Produce      json
// @Success      200  {array}   models.User
// @Failure      500  {object}  map[string]string
// @Security     BearerAuth
// @Router       /admin/users [get]
func (h *AdminUsersHandler) ListUsers(c *gin.Context) {
	users, err := h.authSvc.ListUsers()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, users)
}

type adminCreateUserRequest struct {
	Username string `json:"username" binding:"required,min=3,max=32"`
	Email    string `json:"email"    binding:"required,email"`
	Password string `json:"password" binding:"required,min=8"`
	Role     string `json:"role"     binding:"required,oneof=admin user"`
}

// CreateUser provisions a new account with a chosen role.
//
// @Summary      Create user
// @Tags         admin-users
// @Accept       json
// @Produce      json
// @Param        body  body      adminCreateUserRequest  true  "User details"
// @Success      201   {object}  models.User
// @Failure      400   {object}  map[string]string
// @Failure      409   {object}  map[string]string
// @Security     BearerAuth
// @Router       /admin/users [post]
func (h *AdminUsersHandler) CreateUser(c *gin.Context) {
	var req adminCreateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	user, err := h.authSvc.AdminCreateUser(req.Username, req.Email, req.Password, req.Role)
	if err != nil {
		c.JSON(serviceStatus(err), gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, user)
}

type adminUpdateUserRequest struct {
	Username string `json:"username" binding:"required,min=3,max=32"`
	Email    string `json:"email"    binding:"required,email"`
	Role     string `json:"role"     binding:"required,oneof=admin user"`
}

// UpdateUser changes another user's username/email/role.
//
// @Summary      Update user
// @Tags         admin-users
// @Accept       json
// @Produce      json
// @Param        id    path      string                  true  "User ID"
// @Param        body  body      adminUpdateUserRequest  true  "Fields to update"
// @Success      200   {object}  models.User
// @Failure      400   {object}  map[string]string
// @Failure      404   {object}  map[string]string
// @Security     BearerAuth
// @Router       /admin/users/{id} [put]
func (h *AdminUsersHandler) UpdateUser(c *gin.Context) {
	id := c.Param("id")
	var req adminUpdateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	user, err := h.authSvc.UpdateUser(id, req.Username, req.Email, req.Role)
	if err != nil {
		c.JSON(serviceStatus(err), gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, user)
}

type adminSetPasswordRequest struct {
	Password string `json:"password" binding:"required,min=8"`
}

// SetPassword resets another user's password without needing the old one.
//
// @Summary      Set user password
// @Tags         admin-users
// @Accept       json
// @Produce      json
// @Param        id    path  string                   true  "User ID"
// @Param        body  body  adminSetPasswordRequest  true  "New password"
// @Success      204
// @Failure      400  {object}  map[string]string
// @Failure      404  {object}  map[string]string
// @Security     BearerAuth
// @Router       /admin/users/{id}/set-password [post]
func (h *AdminUsersHandler) SetPassword(c *gin.Context) {
	id := c.Param("id")
	var req adminSetPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.authSvc.SetPassword(id, req.Password); err != nil {
		c.JSON(serviceStatus(err), gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusNoContent, nil)
}

// DeleteUser removes a user. Cannot be used to delete the caller.
//
// @Summary      Delete user
// @Tags         admin-users
// @Param        id  path  string  true  "User ID"
// @Success      204
// @Failure      400  {object}  map[string]string  "cannot delete your own account"
// @Failure      404  {object}  map[string]string
// @Security     BearerAuth
// @Router       /admin/users/{id} [delete]
func (h *AdminUsersHandler) DeleteUser(c *gin.Context) {
	id := c.Param("id")
	callerID := c.GetString("user_id")
	if id == callerID {
		c.JSON(http.StatusBadRequest, gin.H{"error": "cannot delete your own account"})
		return
	}
	if err := h.authSvc.DeleteUser(id); err != nil {
		c.JSON(serviceStatus(err), gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusNoContent, nil)
}

// GetActivity returns a user's recent watch activity (movies + episodes).
//
// @Summary      Get user activity
// @Tags         admin-users
// @Produce      json
// @Param        id  path  string  true  "User ID"
// @Success      200  {array}   object
// @Failure      404  {object}  map[string]string
// @Security     BearerAuth
// @Router       /admin/users/{id}/activity [get]
func (h *AdminUsersHandler) GetActivity(c *gin.Context) {
	id := c.Param("id")
	items, err := h.progressSvc.GetUserActivity(id)
	if err != nil {
		c.JSON(serviceStatus(err), gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, items)
}
