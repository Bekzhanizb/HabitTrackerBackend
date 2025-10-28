package routes

import (
	"net/http"

	"github.com/Bekzhanizb/HabitTrackerBackend/db"
	"github.com/Bekzhanizb/HabitTrackerBackend/models"
	"github.com/gin-gonic/gin"
)

func RegisterCityRoutes(router *gin.Engine) {
	router.GET("/api/cities", func(c *gin.Context) {
		var cities []models.City
		if err := db.DB.Find(&cities).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, cities)
	})
}
