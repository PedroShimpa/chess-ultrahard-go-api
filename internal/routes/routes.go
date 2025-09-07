package routes

import (
	"time"

	"github.com/pedroShimpa/go-api/internal/controllers"
	"github.com/pedroShimpa/go-api/internal/middleware"
	"github.com/pedroShimpa/go-api/internal/repositories"
	"github.com/pedroShimpa/go-api/internal/services"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func RegisterRoutes(r *gin.Engine, db *gorm.DB) {
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"}, // ou "*" para todos
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	userRepo := &repositories.UserRepository{DB: db}
	authService := &services.AuthService{UserRepo: userRepo}
	userController := &controllers.UserController{AuthService: authService}

	api := r.Group("/api")
	{
		api.POST("/register", userController.Register)
		api.POST("/login", userController.Login)

		protected := api.Group("/")
		protected.Use(middleware.AuthMiddleware())
		{
			protected.GET("/perfil", userController.Perfil)
			protected.POST("/game/new", controllers.StartGame)
			protected.POST("/game/move", controllers.MakeMove)
			protected.POST("/game/solo", controllers.SoloGame)
		}
	}
}
