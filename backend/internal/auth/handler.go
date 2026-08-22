package auth

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler { return &Handler{service: service} }

type loginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type passwordRequest struct {
	CurrentPassword string `json:"currentPassword" binding:"required"`
	NewPassword     string `json:"newPassword" binding:"required"`
}

type createUserRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
	Role     Role   `json:"role"`
}

type resetPasswordRequest struct {
	NewPassword string `json:"newPassword" binding:"required"`
}

func (h *Handler) RegisterPublicRoutes(router gin.IRouter) {
	router.POST("/auth/login", h.login)
}

func (h *Handler) RegisterProtectedRoutes(router gin.IRouter) {
	router.GET("/auth/me", h.me)
	router.POST("/auth/logout", h.logout)
	router.POST("/auth/password", h.changePassword)
}

func (h *Handler) RegisterAdminRoutes(router gin.IRouter) {
	router.GET("/users", h.listUsers)
	router.POST("/users", h.createUser)
	router.POST("/users/:id/password", h.resetPassword)
	router.POST("/users/:id/disable", h.setDisabled(true))
	router.POST("/users/:id/enable", h.setDisabled(false))
}

func (h *Handler) login(c *gin.Context) {
	var request loginRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		writeAuthError(c, http.StatusBadRequest, "username and password are required")
		return
	}
	user, token, err := h.service.Login(c.Request.Context(), request.Username, request.Password, loginAttemptKey(c, request.Username))
	if err != nil {
		if isExpectedAuthError(err) {
			writeAuthError(c, http.StatusUnauthorized, "invalid username or password")
			return
		}
		writeAuthError(c, http.StatusServiceUnavailable, "authentication service unavailable")
		return
	}
	h.service.SetSessionCookie(c, token)
	c.JSON(http.StatusOK, gin.H{"user": user})
}

func (h *Handler) me(c *gin.Context) {
	user, _ := CurrentUser(c)
	c.JSON(http.StatusOK, gin.H{"user": user})
}

func (h *Handler) logout(c *gin.Context) {
	if cookie, err := c.Request.Cookie(h.service.CookieName()); err == nil {
		if err := h.service.Logout(c.Request.Context(), cookie.Value); err != nil {
			writeAuthError(c, http.StatusServiceUnavailable, "failed to log out")
			return
		}
	}
	h.service.ClearSessionCookie(c)
	c.Status(http.StatusNoContent)
}

func (h *Handler) changePassword(c *gin.Context) {
	user, _ := CurrentUser(c)
	var request passwordRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		writeAuthError(c, http.StatusBadRequest, "current and new passwords are required")
		return
	}
	if err := h.service.ChangePassword(c.Request.Context(), user.ID, request.CurrentPassword, request.NewPassword); err != nil {
		switch {
		case errors.Is(err, ErrInvalidCredentials):
			writeAuthError(c, http.StatusBadRequest, "current password is incorrect")
		case errors.Is(err, ErrWeakPassword):
			writeAuthError(c, http.StatusBadRequest, "new password must be 12 to 128 characters")
		default:
			writeAuthError(c, http.StatusServiceUnavailable, "failed to change password")
		}
		return
	}
	h.service.ClearSessionCookie(c)
	c.Status(http.StatusNoContent)
}

func (h *Handler) listUsers(c *gin.Context) {
	users, err := h.service.ListUsers(c.Request.Context())
	if err != nil {
		writeAuthError(c, http.StatusServiceUnavailable, "failed to list users")
		return
	}
	c.JSON(http.StatusOK, gin.H{"users": users})
}

func (h *Handler) createUser(c *gin.Context) {
	var request createUserRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		writeAuthError(c, http.StatusBadRequest, "username and password are required")
		return
	}
	if request.Role == "" {
		request.Role = RoleUser
	}
	user, err := h.service.CreateUser(c.Request.Context(), request.Username, request.Password, request.Role)
	if err != nil {
		switch {
		case errors.Is(err, ErrUsernameTaken):
			writeAuthError(c, http.StatusConflict, "username already exists")
		case errors.Is(err, ErrWeakPassword):
			writeAuthError(c, http.StatusBadRequest, "password must be 12 to 128 characters")
		default:
			writeAuthError(c, http.StatusBadRequest, "invalid user data")
		}
		return
	}
	c.JSON(http.StatusCreated, gin.H{"user": user})
}

func (h *Handler) resetPassword(c *gin.Context) {
	userID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || userID < 1 {
		writeAuthError(c, http.StatusBadRequest, "id must be a positive integer")
		return
	}
	var request resetPasswordRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		writeAuthError(c, http.StatusBadRequest, "new password is required")
		return
	}
	if err := h.service.ResetPassword(c.Request.Context(), userID, request.NewPassword); err != nil {
		if errors.Is(err, ErrWeakPassword) {
			writeAuthError(c, http.StatusBadRequest, "password must be 12 to 128 characters")
			return
		}
		if errors.Is(err, ErrUserNotFound) {
			writeAuthError(c, http.StatusNotFound, "user not found")
			return
		}
		writeAuthError(c, http.StatusServiceUnavailable, "failed to reset password")
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) setDisabled(disabled bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil || userID < 1 {
			writeAuthError(c, http.StatusBadRequest, "id must be a positive integer")
			return
		}
		current, _ := CurrentUser(c)
		if current.ID == userID {
			writeAuthError(c, http.StatusBadRequest, "administrator cannot disable itself")
			return
		}
		if err := h.service.SetDisabled(c.Request.Context(), userID, disabled); err != nil {
			if errors.Is(err, ErrUserNotFound) {
				writeAuthError(c, http.StatusNotFound, "user not found")
				return
			}
			writeAuthError(c, http.StatusServiceUnavailable, "failed to update user")
			return
		}
		c.Status(http.StatusNoContent)
	}
}

func writeAuthError(c *gin.Context, status int, message string) {
	c.JSON(status, gin.H{"error": message})
}
