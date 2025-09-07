package routes

import (
	"github.com/pedroShimpa/go-api/internal/controllers"
	"github.com/pedroShimpa/go-api/internal/middleware"
	"github.com/pedroShimpa/go-api/internal/repositories"
	"github.com/pedroShimpa/go-api/internal/services"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func RegisterRoutes(r *gin.Engine, db *gorm.DB) {
	userRepo := &repositories.UserRepository{DB: db}
	authService := &services.AuthService{UserRepo: userRepo}
	userController := &controllers.UserController{AuthService: authService}

	api := r.Group("/api")
	{
		api.POST("/register", userController.Register)
		api.POST("/login", userController.Login)

		// Rotas protegidas
		protected := api.Group("/")
		protected.Use(middleware.AuthMiddleware())
		{
			protected.GET("/perfil", userController.Perfil)
			protected.POST("/game/new", controllers.StartGame)
			protected.POST("/game/move", controllers.MakeMove)
		}

	}
}
