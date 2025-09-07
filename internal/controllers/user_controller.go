package controllers

import (
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/pedroShimpa/go-api/internal/services"
)

var jwtKey = []byte(os.Getenv("JWT_SECRET"))

type UserController struct {
	AuthService *services.AuthService
}

func (uc *UserController) Register(c *gin.Context) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "input inválido"})
		return
	}

	if err := uc.AuthService.Register(req.Username, req.Password); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "não foi possível registrar"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "usuário criado"})
}

func (uc *UserController) Login(c *gin.Context) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "input inválido"})
		return
	}

	if !uc.AuthService.ValidateUser(req.Username, req.Password) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "usuário ou senha inválidos"})
		return
	}

	expirationTime := time.Now().Add(1 * time.Hour)
	claims := jwt.MapClaims{
		"username": req.Username,
		"exp":      expirationTime.Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(jwtKey)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro ao gerar token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"token": tokenString})
}

func (uc *UserController) Perfil(c *gin.Context) {
	username, _ := c.Get("username")
	c.JSON(http.StatusOK, gin.H{
		"message":  "Bem-vindo ao perfil!",
		"username": username,
	})
}
