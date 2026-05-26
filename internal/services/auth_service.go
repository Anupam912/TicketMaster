package services

import (
	"context"
	"errors"
	"fmt"

	"event-ticketing-system/internal/config"
	"event-ticketing-system/internal/models"
	"event-ticketing-system/internal/repository"
	"event-ticketing-system/internal/utils"
	"event-ticketing-system/pkg/jwt"

	"github.com/google/uuid"
)

// Sentinel errors for auth service operations.
var (
	ErrUserExists       = errors.New("user already exists")
	ErrInvalidCreds     = errors.New("invalid credentials")
	ErrAlreadyAdmin     = errors.New("user is already an admin")
	ErrAlreadyRegular   = errors.New("user is already a regular user")
)

// AuthService handles authentication and user management operations.
type AuthService struct {
	userRepo *repository.UserRepository
	config   *config.Config
}

// NewAuthService creates a new AuthService instance.
func NewAuthService(userRepo *repository.UserRepository, cfg *config.Config) *AuthService {
	return &AuthService{
		userRepo: userRepo,
		config:   cfg,
	}
}

// Register creates a new user account and returns authentication response with JWT token.
func (s *AuthService) Register(ctx context.Context, req *models.RegisterRequest) (*models.AuthResponse, error) {
	_, err := s.userRepo.FindByEmail(ctx, req.Email)
	if err == nil {
		return nil, ErrUserExists
	}
	if !errors.Is(err, repository.ErrUserNotFound) {
		return nil, fmt.Errorf("check existing user: %w", err)
	}

	hashedPassword, err := utils.HashPassword(req.Password)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	user := &models.User{
		Email:        req.Email,
		PasswordHash: hashedPassword,
		FullName:     req.FullName,
		Role:         models.RoleUser,
	}

	if err := s.userRepo.Create(ctx, user); err != nil {
		return nil, fmt.Errorf("create user: %w", err)
	}

	token, err := jwt.GenerateToken(user.ID, user.Email, string(user.Role), s.config.JWT.Secret, s.config.JWT.ExpiryHours)
	if err != nil {
		return nil, fmt.Errorf("generate token: %w", err)
	}

	return &models.AuthResponse{
		User:  user,
		Token: token,
	}, nil
}

// Login authenticates a user and returns authentication response with JWT token.
func (s *AuthService) Login(ctx context.Context, req *models.LoginRequest) (*models.AuthResponse, error) {
	user, err := s.userRepo.FindByEmail(ctx, req.Email)
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			return nil, ErrInvalidCreds
		}
		return nil, fmt.Errorf("find user: %w", err)
	}

	if !utils.CheckPasswordHash(req.Password, user.PasswordHash) {
		return nil, ErrInvalidCreds
	}

	token, err := jwt.GenerateToken(user.ID, user.Email, string(user.Role), s.config.JWT.Secret, s.config.JWT.ExpiryHours)
	if err != nil {
		return nil, fmt.Errorf("generate token: %w", err)
	}

	return &models.AuthResponse{
		User:  user,
		Token: token,
	}, nil
}

// GetUserByID retrieves a user by their UUID.
func (s *AuthService) GetUserByID(ctx context.Context, userID uuid.UUID) (*models.User, error) {
	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("find user: %w", err)
	}
	return user, nil
}

// PromoteToAdmin promotes a regular user to admin role.
func (s *AuthService) PromoteToAdmin(ctx context.Context, userID uuid.UUID) (*models.User, error) {
	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("find user: %w", err)
	}

	if user.Role == models.RoleAdmin {
		return nil, ErrAlreadyAdmin
	}

	if err := s.userRepo.UpdateRole(ctx, userID, models.RoleAdmin); err != nil {
		return nil, fmt.Errorf("update role: %w", err)
	}

	user.Role = models.RoleAdmin
	return user, nil
}

// DemoteToUser demotes an admin to regular user role.
func (s *AuthService) DemoteToUser(ctx context.Context, userID uuid.UUID) (*models.User, error) {
	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("find user: %w", err)
	}

	if user.Role == models.RoleUser {
		return nil, ErrAlreadyRegular
	}

	if err := s.userRepo.UpdateRole(ctx, userID, models.RoleUser); err != nil {
		return nil, fmt.Errorf("update role: %w", err)
	}

	user.Role = models.RoleUser
	return user, nil
}

// ListUsers retrieves all users. This operation is intended for admin use.
func (s *AuthService) ListUsers(ctx context.Context) ([]*models.User, error) {
	users, err := s.userRepo.ListUsers(ctx)
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}
	return users, nil
}
