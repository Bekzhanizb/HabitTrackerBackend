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
	// Подключение к БД и миграция
	db.Connect()
	db.DB.AutoMigrate(&models.City{}, &models.User{}, &models.Habit{}, &models.HabitLog{}, &models.Achievement{})

	r := gin.Default()

	// CORS
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"http://localhost:3000"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	// Статика
	r.Static("/uploads", "./uploads")

	// Тестовый маршрут
	r.GET("/", func(c *gin.Context) {
		c.String(http.StatusOK, "Backend is running 🚀")
	})

	// Аутентификация
	r.POST("/login", routes.Login)
	r.POST("/register", handlers.RegisterHandler)
	r.POST("/update-profile", routes.AuthMiddleware(), routes.UpdateProfile)

	auth := r.Group("/auth")
	auth.Use(routes.AuthMiddleware())
	{
		auth.GET("/profile", routes.Profile)
		auth.PUT("/profile", routes.UpdateProfile)
	}

	// Маршруты для привычек
	habitRoutes := r.Group("/habits")
	habitRoutes.Use(routes.AuthMiddleware())
	{
		habitRoutes.POST("/", handlers.CreateHabit)
		habitRoutes.GET("/", handlers.GetHabits)
		habitRoutes.PUT("/:id", handlers.UpdateHabit)
		habitRoutes.DELETE("/:id", handlers.DeleteHabit)
		habitRoutes.POST("/log", handlers.LogHabit)
		habitRoutes.GET("/logs", routes.RoleMiddleware(models.RoleAdmin), handlers.GetHabitLogs) // доступно только админам
	}

	// Маршруты для городов
	routes.RegisterCityRoutes(r)

	r.Run(":8080")
}
