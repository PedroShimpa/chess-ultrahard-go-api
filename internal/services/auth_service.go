package services

import (
	"github.com/pedroShimpa/go-api/internal/models"
	"github.com/pedroShimpa/go-api/internal/repositories"
	"golang.org/x/crypto/bcrypt"
)

type AuthService struct {
	UserRepo *repositories.UserRepository
}

func (s *AuthService) Register(username, password string) error {
	hash, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	user := &models.User{Username: username, Password: string(hash)}
	return s.UserRepo.Create(user)
}

func (s *AuthService) ValidateUser(username, password string) bool {
	user, err := s.UserRepo.FindByUsername(username)
	if err != nil {
		return false
	}
	return bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)) == nil
}
