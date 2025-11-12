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

	habitLog := models.HabitLog{
		HabitID: habit.ID,
		Date:    time.Now(),
	}
	if err := db.DB.Create(&habitLog).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка при сохранении лога"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Привычка успешно создана", "habit": habit})
}

func GetHabits(c *gin.Context) {
	userInterface, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	currentUser := userInterface.(models.User)

	var habits []models.Habit
	query := db.DB.Preload("Logs")

	if currentUser.Role != models.RoleAdmin {
		query = query.Where("user_id = ?", currentUser.ID)
	} else {
		userID := c.Query("user_id")
		if userID != "" {
			query = query.Where("user_id = ?", userID)
		}
	}

	if err := query.Find(&habits).Error; err != nil {
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

	habitLog := models.HabitLog{
		HabitID: log.HabitID,
		Date:    time.Now(),
	}
	if err := db.DB.Create(&habitLog).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка при сохранении лога выполнения"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Привычка отмечена как выполненная", "log": log})
}

func GetHabitLogs(c *gin.Context) {
	userInterface, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	currentUser := userInterface.(models.User)

	var logs []models.HabitLog
	query := db.DB.Preload("Habit")

	if currentUser.Role != models.RoleAdmin {
		query = query.Joins("JOIN habits ON habits.id = habit_logs.habit_id").
			Where("habits.user_id = ?", currentUser.ID)
	} else {
		userID := c.Query("user_id")
		if userID != "" {
			query = query.Joins("JOIN habits ON habits.id = habit_logs.habit_id").
				Where("habits.user_id = ?", userID)
		}
	}

	if err := query.Find(&logs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка при получении логов"})
		return
	}

	c.JSON(http.StatusOK, logs)
}

func UpdateHabit(c *gin.Context) {
	id := c.Param("id")

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

	habitLog := models.HabitLog{
		HabitID: habit.ID,
		Date:    time.Now(),
	}
	if err := db.DB.Create(&habitLog).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка при сохранении лога"})
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

	habitLog := models.HabitLog{
		HabitID: habit.ID,
		Date:    time.Now(),
	}
	if err := db.DB.Create(&habitLog).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка при сохранении лога удаления"})
		return
	}

	if err := db.DB.Delete(&habit).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete habit"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Habit deleted"})
}
