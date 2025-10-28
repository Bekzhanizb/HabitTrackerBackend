package main

import (
	"github.com/Bekzhanizb/HabitTrackerBackend/db"
	"github.com/Bekzhanizb/HabitTrackerBackend/models"
	"github.com/Bekzhanizb/HabitTrackerBackend/routes"
	"github.com/gin-gonic/gin"
)

func main() {
	db.Connect()
	db.DB.AutoMigrate(&models.City{}, &models.User{})

	r := gin.Default()
	r.Static("/uploads", "./uploads")

	r.POST("/register", routes.Register)
	r.POST("/login", routes.Login)

	auth := r.Group("/auth")
	auth.Use(routes.AuthMiddleware())
	auth.GET("/profile", routes.Profile)

	r.Run(":8080")
}
