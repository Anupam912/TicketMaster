package services

import (
	"errors"

	"event-ticketing-system/internal/config"
	"event-ticketing-system/internal/models"
	"event-ticketing-system/internal/repository"
	"event-ticketing-system/internal/utils"
	"event-ticketing-system/pkg/jwt"

	"github.com/google/uuid"
)

type AuthService struct {
	userRepo *repository.UserRepository
	config   *config.Config
}

func NewAuthService(userRepo *repository.UserRepository, cfg *config.Config) *AuthService {
	return &AuthService{
		userRepo: userRepo,
		config:   cfg,
	}
}

func (s *AuthService) Register(req *models.RegisterRequest) (*models.AuthResponse, error) {
	_, err := s.userRepo.FindByEmail(req.Email)
	if err == nil {
		return nil, errors.New("user already exists")
	}
	if err != repository.ErrUserNotFound {
		return nil, err
	}

	hashedPassword, err := utils.HashPassword(req.Password)
	if err != nil {
		return nil, err
	}

	user := &models.User{
		Email:        req.Email,
		PasswordHash: hashedPassword,
		FullName:     req.FullName,
		Role:         models.RoleUser,
	}

	if err := s.userRepo.Create(user); err != nil {
		return nil, err
	}

	token, err := jwt.GenerateToken(user.ID, user.Email, string(user.Role), s.config.JWT.Secret, s.config.JWT.ExpiryHours)
	if err != nil {
		return nil, err
	}

	return &models.AuthResponse{
		User:  user,
		Token: token,
	}, nil
}

func (s *AuthService) Login(req *models.LoginRequest) (*models.AuthResponse, error) {
	user, err := s.userRepo.FindByEmail(req.Email)
	if err != nil {
		if err == repository.ErrUserNotFound {
			return nil, errors.New("invalid credentials")
		}
		return nil, err
	}

	if !utils.CheckPasswordHash(req.Password, user.PasswordHash) {
		return nil, errors.New("invalid credentials")
	}

	token, err := jwt.GenerateToken(user.ID, user.Email, string(user.Role), s.config.JWT.Secret, s.config.JWT.ExpiryHours)
	if err != nil {
		return nil, err
	}

	return &models.AuthResponse{
		User:  user,
		Token: token,
	}, nil
}

func (s *AuthService) GetUserByID(userID uuid.UUID) (*models.User, error) {
	return s.userRepo.FindByID(userID)
}
