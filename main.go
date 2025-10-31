package main

import (
	"net/http"
	"time"

	"github.com/Bekzhanizb/HabitTrackerBackend/db"
	"github.com/Bekzhanizb/HabitTrackerBackend/handlers"
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

	r.GET("/", func(c *gin.Context) {
		c.String(http.StatusOK, "Backend is running 🚀")
	})

	r.POST("/login", routes.Login)
	r.POST("/register", handlers.RegisterHandler)
	r.POST("/update-profile", routes.AuthMiddleware(), routes.UpdateProfile)

	auth := r.Group("/auth")
	auth.Use(routes.AuthMiddleware())
	auth.GET("/profile", routes.Profile)
	auth.PUT("/profile", routes.UpdateProfile)

	r.POST("/habit", handlers.CreateHabit)
	r.GET("/habits", handlers.GetHabits)
	r.POST("/habit/log", handlers.LogHabit)
	r.PUT("/habit/:id", handlers.UpdateHabit)
	r.DELETE("/habit/:id", handlers.DeleteHabit)

	routes.RegisterCityRoutes(r)

	r.Run(":8080")
}
