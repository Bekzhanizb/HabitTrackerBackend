package handlers

import (
	"net/http"
	"time"

	"github.com/Bekzhanizb/HabitTrackerBackend/db"
	"github.com/Bekzhanizb/HabitTrackerBackend/models"
	"github.com/gin-gonic/gin"
)

func CreateHabit(c *gin.Context) {
	var habit models.Habit

	if err := c.ShouldBindJSON(&habit); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Некорректные данные"})
		return
	}

	if habit.Title == "" || habit.Frequency == "" || habit.UserID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Необходимо заполнить обязательные поля"})
		return
	}

	if err := db.DB.Create(&habit).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка при создании привычки"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Привычка успешно создана", "habit": habit})
}

func GetHabits(c *gin.Context) {
	userID := c.Query("user_id")

	var habits []models.Habit
	if err := db.DB.Preload("Logs").Where("user_id = ?", userID).Find(&habits).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка при получении привычек"})
		return
	}

	c.JSON(http.StatusOK, habits)
}

func LogHabit(c *gin.Context) {
	var log models.HabitLog

	if err := c.ShouldBindJSON(&log); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Некорректные данные"})
		return
	}

	log.Date = time.Now()

	if err := db.DB.Create(&log).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка при сохранении лога"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Привычка отмечена как выполненная", "log": log})
}

func UpdateHabit(c *gin.Context) {
	// получаем id из пути
	id := c.Param("id")

	// загружаем существующую привычку
	var habit models.Habit
	if err := db.DB.First(&habit, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Habit not found"})
		return
	}

	var input struct {
		Title       *string `json:"title"`
		Description *string `json:"description"`
		Frequency   *string `json:"frequency"`
		IsActive    *bool   `json:"is_active"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input"})
		return
	}

	// обновляем поля, которые пришли
	if input.Title != nil {
		habit.Title = *input.Title
	}
	if input.Description != nil {
		habit.Description = *input.Description
	}
	if input.Frequency != nil {
		habit.Frequency = *input.Frequency
	}
	if input.IsActive != nil {
		habit.IsActive = *input.IsActive
	}

	if err := db.DB.Save(&habit).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update habit"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Habit updated", "habit": habit})
}

func DeleteHabit(c *gin.Context) {
	id := c.Param("id")

	var habit models.Habit
	if err := db.DB.First(&habit, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Habit not found"})
		return
	}

	if err := db.DB.Delete(&habit).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete habit"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Habit deleted"})
}
