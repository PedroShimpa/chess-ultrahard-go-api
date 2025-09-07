package main

import (
	"log"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"

	"github.com/pedroShimpa/go-api/config"
	"github.com/pedroShimpa/go-api/internal/controllers"
	"github.com/pedroShimpa/go-api/internal/routes"
)

func main() {
	db := config.InitDB()
	controllers.InitDatabase(db)
	r := gin.Default()
	err := godotenv.Load()
	if err != nil {
		log.Println("Erro ao carregar .env, usando variáveis do sistema")
	}

	routes.RegisterRoutes(r, db)
	r.Run(":" + os.Getenv("APP_PORT"))
}
