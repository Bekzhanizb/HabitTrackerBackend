package main

import (
	"time"

	"github.com/Bekzhanizb/HabitTrackerBackend/db"
	"github.com/Bekzhanizb/HabitTrackerBackend/models"
	"github.com/Bekzhanizb/HabitTrackerBackend/routes"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func main() {
	db.Connect()
	db.DB.AutoMigrate(&models.City{}, &models.User{}, &models.Habit{}, &models.HabitLog{}, &models.Achievement{})

	r := gin.Default()

	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"http://localhost:3000"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	r.Static("/uploads", "./uploads")

	r.POST("/register", routes.Register)
	r.POST("/login", routes.Login)

	auth := r.Group("/auth")
	auth.Use(routes.AuthMiddleware())
	auth.GET("/profile", routes.Profile)
	auth.PUT("/profile", routes.UpdateProfile)

	routes.RegisterCityRoutes(r)

	r.Run(":8080")
}
