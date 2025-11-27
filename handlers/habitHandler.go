package handlers

import (
	"net/http"
	"strconv"
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

	// Сохраняем ровно один лог при создании
	habitLog := models.HabitLog{
		HabitID:     habit.ID,
		Date:        time.Now(),
		IsCompleted: false,
	}
	if err := db.DB.Create(&habitLog).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка при сохранении лога"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Привычка успешно создана", "habit": habit})
}

// GetHabits возвращает привычки текущего пользователя (или фильтр для админа)
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
			// безопасно конвертим, но если не число — просто вернём ошибку
			if _, err := strconv.Atoi(userID); err == nil {
				query = query.Where("user_id = ?", userID)
			} else {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user_id"})
				return
			}
		}
	}

	if err := query.Find(&habits).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка при получении привычек"})
		return
	}

	c.JSON(http.StatusOK, habits)
}

// LogHabit — помечаем выполнение привычки, создаём ровно один лог
func LogHabit(c *gin.Context) {
	var input struct {
		HabitID     uint  `json:"habit_id" binding:"required"`
		IsCompleted *bool `json:"is_completed"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Некорректные данные"})
		return
	}

	// проверяем владельца привычки
	userInterface, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	currentUser := userInterface.(models.User)

	var habit models.Habit
	if err := db.DB.First(&habit, input.HabitID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Habit not found"})
		return
	}
	if habit.UserID != currentUser.ID && currentUser.Role != models.RoleAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "Нет доступа к этой привычке"})
		return
	}

	// единый лог — помечаем дату и передачаем isCompleted (по умолчанию true)
	isCompleted := true
	if input.IsCompleted != nil {
		isCompleted = *input.IsCompleted
	}

	log := models.HabitLog{
		HabitID:     input.HabitID,
		Date:        time.Now(),
		IsCompleted: isCompleted,
	}

	if err := db.DB.Create(&log).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка при сохранении лога"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Привычка отмечена как выполненная", "log": log})
}

// GetHabitLogs — возвращает логи; админ может фильтровать по user_id
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
		// присоединяемся к habits и фильтруем по user
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

// UpdateHabit — проверяем владельца, применяем изменения
func UpdateHabit(c *gin.Context) {
	id := c.Param("id")

	var habit models.Habit
	if err := db.DB.First(&habit, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Habit not found"})
		return
	}

	// проверка владельца
	userInterface, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	currentUser := userInterface.(models.User)

	if habit.UserID != currentUser.ID && currentUser.Role != models.RoleAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "Нет доступа к этой привычке"})
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

	// при обновлении лог создавать не будем, чтобы не засорять таблицу
	c.JSON(http.StatusOK, gin.H{"message": "Habit updated", "habit": habit})
}

// DeleteHabit — проверяем владельца и удаляем; не создаём лог удаления
func DeleteHabit(c *gin.Context) {
	id := c.Param("id")

	var habit models.Habit
	if err := db.DB.First(&habit, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Habit not found"})
		return
	}

	// проверка владельца
	userInterface, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	currentUser := userInterface.(models.User)

	if habit.UserID != currentUser.ID && currentUser.Role != models.RoleAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "Нет доступа к этой привычке"})
		return
	}

	// удаляем связанные логи сначала (чтобы не осталось orphan records)
	if err := db.DB.Where("habit_id = ?", habit.ID).Delete(&models.HabitLog{}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete habit logs"})
		return
	}

	if err := db.DB.Delete(&habit).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete habit"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Habit deleted"})
}
