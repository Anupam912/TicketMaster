package handlers

import (
	"errors"
	"net/http"

	"event-ticketing-system/internal/middleware"
	"event-ticketing-system/internal/models"
	"event-ticketing-system/internal/repository"
	"event-ticketing-system/internal/services"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// AuthHandler handles authentication and user management HTTP requests.
type AuthHandler struct {
	authService *services.AuthService
}

// NewAuthHandler creates a new AuthHandler instance.
func NewAuthHandler(authService *services.AuthService) *AuthHandler {
	return &AuthHandler{
		authService: authService,
	}
}

// Register godoc
// @Summary      Register a new user
// @Description  Create a new user account with email and password
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        request body models.RegisterRequest true "Registration details"
// @Success      201 {object} models.AuthResponse
// @Failure      400 {object} map[string]string "Invalid request body"
// @Failure      409 {object} map[string]string "User already exists"
// @Failure      500 {object} map[string]string "Internal server error"
// @Router       /auth/register [post]
func (h *AuthHandler) Register(c *gin.Context) {
	var req models.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	response, err := h.authService.Register(c.Request.Context(), &req)
	if err != nil {
		if errors.Is(err, services.ErrUserExists) {
			c.JSON(http.StatusConflict, gin.H{"error": "user already exists"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "registration failed"})
		return
	}

	c.JSON(http.StatusCreated, response)
}

// Login godoc
// @Summary      Login user
// @Description  Authenticate user with email and password, returns JWT token
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        request body models.LoginRequest true "Login credentials"
// @Success      200 {object} models.AuthResponse
// @Failure      400 {object} map[string]string "Invalid request body"
// @Failure      401 {object} map[string]string "Invalid credentials"
// @Failure      500 {object} map[string]string "Internal server error"
// @Router       /auth/login [post]
func (h *AuthHandler) Login(c *gin.Context) {
	var req models.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	response, err := h.authService.Login(c.Request.Context(), &req)
	if err != nil {
		if errors.Is(err, services.ErrInvalidCreds) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "login failed"})
		return
	}

	c.JSON(http.StatusOK, response)
}

// GetMe godoc
// @Summary      Get current user profile
// @Description  Returns the authenticated user's profile information
// @Tags         auth
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Success      200 {object} map[string]models.User
// @Failure      401 {object} map[string]string "Unauthorized"
// @Failure      404 {object} map[string]string "User not found"
// @Failure      500 {object} map[string]string "Internal server error"
// @Router       /auth/me [get]
func (h *AuthHandler) GetMe(c *gin.Context) {
	userID, err := middleware.GetUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	user, err := h.authService.GetUserByID(c.Request.Context(), userID)
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get user"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"user": user})
}

// UserResponse represents a safe user response without sensitive data.
type UserResponse struct {
	ID        uuid.UUID       `json:"id" example:"550e8400-e29b-41d4-a716-446655440000"`
	Email     string          `json:"email" example:"user@example.com"`
	FullName  string          `json:"full_name" example:"John Doe"`
	Role      models.UserRole `json:"role" example:"user"`
	CreatedAt string          `json:"created_at" example:"2024-01-15T10:30:00Z"`
}

// ListUsers godoc
// @Summary      List all users (Admin only)
// @Description  Returns a list of all registered users
// @Tags         admin
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Success      200 {object} map[string][]UserResponse
// @Failure      401 {object} map[string]string "Unauthorized"
// @Failure      403 {object} map[string]string "Forbidden - Admin only"
// @Failure      500 {object} map[string]string "Internal server error"
// @Router       /admin/users [get]
func (h *AuthHandler) ListUsers(c *gin.Context) {
	users, err := h.authService.ListUsers(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list users"})
		return
	}

	safeUsers := make([]UserResponse, len(users))
	for i, user := range users {
		safeUsers[i] = UserResponse{
			ID:        user.ID,
			Email:     user.Email,
			FullName:  user.FullName,
			Role:      user.Role,
			CreatedAt: user.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		}
	}

	c.JSON(http.StatusOK, gin.H{"users": safeUsers})
}

// PromoteToAdmin godoc
// @Summary      Promote user to admin (Admin only)
// @Description  Promotes a regular user to admin role
// @Tags         admin
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path string true "User ID" format(uuid)
// @Success      200 {object} map[string]interface{} "User promoted successfully"
// @Failure      400 {object} map[string]string "Invalid user ID"
// @Failure      401 {object} map[string]string "Unauthorized"
// @Failure      404 {object} map[string]string "User not found"
// @Failure      409 {object} map[string]string "User is already an admin"
// @Failure      500 {object} map[string]string "Internal server error"
// @Router       /admin/users/{id}/promote [post]
func (h *AuthHandler) PromoteToAdmin(c *gin.Context) {
	userID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user ID"})
		return
	}

	user, err := h.authService.PromoteToAdmin(c.Request.Context(), userID)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrAlreadyAdmin):
			c.JSON(http.StatusConflict, gin.H{"error": "user is already an admin"})
		case errors.Is(err, repository.ErrUserNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to promote user"})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "user promoted to admin",
		"user": UserResponse{
			ID:       user.ID,
			Email:    user.Email,
			FullName: user.FullName,
			Role:     user.Role,
		},
	})
}

// DemoteToUser godoc
// @Summary      Demote admin to user (Admin only)
// @Description  Demotes an admin to regular user role
// @Tags         admin
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path string true "User ID" format(uuid)
// @Success      200 {object} map[string]interface{} "User demoted successfully"
// @Failure      400 {object} map[string]string "Invalid user ID or cannot demote yourself"
// @Failure      401 {object} map[string]string "Unauthorized"
// @Failure      404 {object} map[string]string "User not found"
// @Failure      409 {object} map[string]string "User is already a regular user"
// @Failure      500 {object} map[string]string "Internal server error"
// @Router       /admin/users/{id}/demote [post]
func (h *AuthHandler) DemoteToUser(c *gin.Context) {
	userID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user ID"})
		return
	}

	currentUserID, _ := middleware.GetUserID(c)
	if currentUserID == userID {
		c.JSON(http.StatusBadRequest, gin.H{"error": "cannot demote yourself"})
		return
	}

	user, err := h.authService.DemoteToUser(c.Request.Context(), userID)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrAlreadyRegular):
			c.JSON(http.StatusConflict, gin.H{"error": "user is already a regular user"})
		case errors.Is(err, repository.ErrUserNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to demote user"})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "user demoted to regular user",
		"user": UserResponse{
			ID:       user.ID,
			Email:    user.Email,
			FullName: user.FullName,
			Role:     user.Role,
		},
	})
}
