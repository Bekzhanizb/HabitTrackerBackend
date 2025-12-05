package handlers

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/Bekzhanizb/HabitTrackerBackend/db"
	"github.com/Bekzhanizb/HabitTrackerBackend/middleware"
	"github.com/Bekzhanizb/HabitTrackerBackend/models"
	"github.com/Bekzhanizb/HabitTrackerBackend/utils"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// CreateHabitRequest с валидацией
type CreateHabitRequest struct {
	Title       string `json:"title" binding:"required,min=1,max=100"`
	Description string `json:"description" binding:"max=500"`
	Frequency   string `json:"frequency" binding:"required,oneof=daily weekly monthly"`
	UserID      uint   `json:"user_id" binding:"required,min=1"`
}

func CreateHabit(c *gin.Context) {
	var req CreateHabitRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Logger.Warn("invalid_create_habit_request",
			zap.Error(err),
			zap.String("client_ip", c.ClientIP()),
		)
		utils.ErrorCount.WithLabelValues("CreateHabit", "validation").Inc()
		c.JSON(http.StatusBadRequest, gin.H{"error": "Некорректные данные", "details": err.Error()})
		return
	}

	if err := middleware.ValidateStruct(req); err != nil {
		utils.Logger.Warn("validation_failed", zap.Error(err))
		utils.ErrorCount.WithLabelValues("CreateHabit", "validation").Inc()
		c.JSON(http.StatusBadRequest, gin.H{"error": "Ошибка валидации", "details": err.Error()})
		return
	}

	// 🔥 FIX: Безопасное получение пользователя
	userInterface, exists := c.Get("user")
	if !exists {
		utils.Logger.Error("user_not_found_in_context")
		utils.ErrorCount.WithLabelValues("CreateHabit", "auth").Inc()
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	currentUser, ok := userInterface.(models.User)
	if !ok {
		utils.Logger.Error("invalid_user_type_in_context",
			zap.String("type", fmt.Sprintf("%T", userInterface)))
		utils.ErrorCount.WithLabelValues("CreateHabit", "auth").Inc()
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid user context"})
		return
	}

	if currentUser.Role != models.RoleAdmin && req.UserID != currentUser.ID {
		utils.Logger.Warn("unauthorized_habit_creation",
			zap.Uint("current_user_id", currentUser.ID),
			zap.Uint("requested_user_id", req.UserID),
		)
		utils.ErrorCount.WithLabelValues("CreateHabit", "forbidden").Inc()
		c.JSON(http.StatusForbidden, gin.H{"error": "Нет доступа"})
		return
	}

	habit := models.Habit{
		UserID:      req.UserID,
		Title:       req.Title,
		Description: req.Description,
		Frequency:   req.Frequency,
		IsActive:    true,
	}

	if err := db.DB.Create(&habit).Error; err != nil {
		utils.Logger.Error("db_create_habit_failed",
			zap.Error(err),
			zap.Uint("user_id", req.UserID),
		)
		utils.ErrorCount.WithLabelValues("CreateHabit", "database").Inc()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка при создании привычки"})
		return
	}

	habitLog := models.HabitLog{
		HabitID:     habit.ID,
		Date:        time.Now(),
		IsCompleted: false,
	}
	if err := db.DB.Create(&habitLog).Error; err != nil {
		utils.Logger.Error("db_create_habitlog_failed",
			zap.Error(err),
			zap.Uint("habit_id", habit.ID),
		)
		utils.ErrorCount.WithLabelValues("CreateHabit", "database").Inc()
	}

	utils.Logger.Info("habit_created",
		zap.Uint("habit_id", habit.ID),
		zap.Uint("user_id", req.UserID),
		zap.String("title", req.Title),
	)

	c.JSON(http.StatusOK, gin.H{"message": "Привычка успешно создана", "habit": habit})
}

func GetHabits(c *gin.Context) {
	utils.Logger.Info("GetHabits started", zap.String("path", c.Request.URL.Path))

	userInterface, exists := c.Get("user")
	if !exists {
		utils.Logger.Error("user_not_found_in_context",
			zap.String("headers", fmt.Sprintf("%v", c.Request.Header)))
		utils.ErrorCount.WithLabelValues("GetHabits", "auth").Inc()
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized - user not in context"})
		return
	}

	utils.Logger.Info("user_interface_type", zap.String("type", fmt.Sprintf("%T", userInterface)))

	currentUser, ok := userInterface.(models.User)
	if !ok {
		utils.Logger.Error("invalid_user_type_assertion",
			zap.String("expected", "models.User"),
			zap.String("actual", fmt.Sprintf("%T", userInterface)),
			zap.Any("value", userInterface),
		)
		utils.ErrorCount.WithLabelValues("GetHabits", "auth").Inc()
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Invalid user type in context",
			"debug": fmt.Sprintf("Expected models.User, got %T", userInterface),
		})
		return
	}

	utils.Logger.Info("current_user_retrieved",
		zap.Uint("id", currentUser.ID),
		zap.String("username", currentUser.Username),
		zap.String("role", currentUser.Role))

	var habits []models.Habit
	query := db.DB.Preload("Logs")

	if currentUser.Role != models.RoleAdmin {
		query = query.Where("user_id = ?", currentUser.ID)
		utils.Logger.Info("user_query", zap.Uint("user_id", currentUser.ID))
	} else {
		userID := c.Query("user_id")
		utils.Logger.Info("admin_query", zap.String("user_id_param", userID))
		if userID != "" {
			if id, err := strconv.Atoi(userID); err == nil {
				query = query.Where("user_id = ?", id)
			} else {
				utils.Logger.Warn("invalid_user_id_param", zap.String("user_id", userID))
				c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user_id"})
				return
			}
		}
	}

	utils.Logger.Info("executing_database_query")
	if err := query.Find(&habits).Error; err != nil {
		utils.Logger.Error("db_get_habits_failed", zap.Error(err))
		utils.ErrorCount.WithLabelValues("GetHabits", "database").Inc()
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Ошибка при получении привычек",
			"details": err.Error(),
		})
		return
	}

	utils.Logger.Info("habits_retrieved_successfully", zap.Int("count", len(habits)))
	c.JSON(http.StatusOK, habits)
}

type LogHabitRequest struct {
	HabitID     uint  `json:"habit_id" binding:"required,min=1"`
	IsCompleted *bool `json:"is_completed"`
}

func LogHabit(c *gin.Context) {
	var req LogHabitRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Logger.Warn("invalid_log_habit_request", zap.Error(err))
		utils.ErrorCount.WithLabelValues("LogHabit", "validation").Inc()
		c.JSON(http.StatusBadRequest, gin.H{"error": "Некорректные данные", "details": err.Error()})
		return
	}

	if err := middleware.ValidateStruct(req); err != nil {
		utils.ErrorCount.WithLabelValues("LogHabit", "validation").Inc()
		c.JSON(http.StatusBadRequest, gin.H{"error": "Ошибка валидации", "details": err.Error()})
		return
	}

	userInterface, exists := c.Get("user")
	if !exists {
		utils.ErrorCount.WithLabelValues("LogHabit", "auth").Inc()
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	currentUser, ok := userInterface.(models.User)
	if !ok {
		utils.Logger.Error("invalid_user_type", zap.String("type", fmt.Sprintf("%T", userInterface)))
		utils.ErrorCount.WithLabelValues("LogHabit", "auth").Inc()
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid user context"})
		return
	}

	var habit models.Habit
	if err := db.DB.First(&habit, req.HabitID).Error; err != nil {
		utils.Logger.Warn("habit_not_found", zap.Uint("habit_id", req.HabitID))
		utils.ErrorCount.WithLabelValues("LogHabit", "not_found").Inc()
		c.JSON(http.StatusNotFound, gin.H{"error": "Habit not found"})
		return
	}

	if habit.UserID != currentUser.ID && currentUser.Role != models.RoleAdmin {
		utils.Logger.Warn("unauthorized_habit_log",
			zap.Uint("habit_id", req.HabitID),
			zap.Uint("user_id", currentUser.ID),
		)
		utils.ErrorCount.WithLabelValues("LogHabit", "forbidden").Inc()
		c.JSON(http.StatusForbidden, gin.H{"error": "Нет доступа к этой привычке"})
		return
	}

	isCompleted := true
	if req.IsCompleted != nil {
		isCompleted = *req.IsCompleted
	}

	log := models.HabitLog{
		HabitID:     req.HabitID,
		Date:        time.Now(),
		IsCompleted: isCompleted,
	}

	if err := db.DB.Create(&log).Error; err != nil {
		utils.Logger.Error("db_create_log_failed",
			zap.Error(err),
			zap.Uint("habit_id", req.HabitID),
		)
		utils.ErrorCount.WithLabelValues("LogHabit", "database").Inc()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка при сохранении лога"})
		return
	}

	utils.Logger.Info("habit_logged",
		zap.Uint("habit_id", req.HabitID),
		zap.Bool("is_completed", isCompleted),
	)

	c.JSON(http.StatusOK, gin.H{"message": "Привычка отмечена", "log": log})
}

func GetHabitLogs(c *gin.Context) {
	userInterface, exists := c.Get("user")
	if !exists {
		utils.ErrorCount.WithLabelValues("GetHabitLogs", "auth").Inc()
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	currentUser, ok := userInterface.(models.User)
	if !ok {
		utils.Logger.Error("invalid_user_type", zap.String("type", fmt.Sprintf("%T", userInterface)))
		utils.ErrorCount.WithLabelValues("GetHabitLogs", "auth").Inc()
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid user context"})
		return
	}

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
		utils.Logger.Error("db_get_logs_failed", zap.Error(err))
		utils.ErrorCount.WithLabelValues("GetHabitLogs", "database").Inc()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка при получении логов"})
		return
	}

	c.JSON(http.StatusOK, logs)
}

type UpdateHabitRequest struct {
	Title       *string `json:"title" binding:"omitempty,min=1,max=100"`
	Description *string `json:"description" binding:"omitempty,max=500"`
	Frequency   *string `json:"frequency" binding:"omitempty,oneof=daily weekly monthly"`
	IsActive    *bool   `json:"is_active"`
}

func UpdateHabit(c *gin.Context) {
	id := c.Param("id")

	var habit models.Habit
	if err := db.DB.First(&habit, id).Error; err != nil {
		utils.Logger.Warn("habit_not_found_for_update", zap.String("id", id))
		utils.ErrorCount.WithLabelValues("UpdateHabit", "not_found").Inc()
		c.JSON(http.StatusNotFound, gin.H{"error": "Habit not found"})
		return
	}

	userInterface, exists := c.Get("user")
	if !exists {
		utils.ErrorCount.WithLabelValues("UpdateHabit", "auth").Inc()
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	currentUser, ok := userInterface.(models.User)
	if !ok {
		utils.Logger.Error("invalid_user_type", zap.String("type", fmt.Sprintf("%T", userInterface)))
		utils.ErrorCount.WithLabelValues("UpdateHabit", "auth").Inc()
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid user context"})
		return
	}

	if habit.UserID != currentUser.ID && currentUser.Role != models.RoleAdmin {
		utils.Logger.Warn("unauthorized_habit_update",
			zap.String("habit_id", id),
			zap.Uint("user_id", currentUser.ID),
		)
		utils.ErrorCount.WithLabelValues("UpdateHabit", "forbidden").Inc()
		c.JSON(http.StatusForbidden, gin.H{"error": "Нет доступа к этой привычке"})
		return
	}

	var req UpdateHabitRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Logger.Warn("invalid_update_request", zap.Error(err))
		utils.ErrorCount.WithLabelValues("UpdateHabit", "validation").Inc()
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input", "details": err.Error()})
		return
	}

	if req.Title != nil {
		habit.Title = *req.Title
	}
	if req.Description != nil {
		habit.Description = *req.Description
	}
	if req.Frequency != nil {
		habit.Frequency = *req.Frequency
	}
	if req.IsActive != nil {
		habit.IsActive = *req.IsActive
	}

	if err := db.DB.Save(&habit).Error; err != nil {
		utils.Logger.Error("db_update_habit_failed", zap.Error(err), zap.String("habit_id", id))
		utils.ErrorCount.WithLabelValues("UpdateHabit", "database").Inc()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update habit"})
		return
	}

	utils.Logger.Info("habit_updated", zap.String("habit_id", id))
	c.JSON(http.StatusOK, gin.H{"message": "Habit updated", "habit": habit})
}

func DeleteHabit(c *gin.Context) {
	id := c.Param("id")

	var habit models.Habit
	if err := db.DB.First(&habit, id).Error; err != nil {
		utils.Logger.Warn("habit_not_found_for_delete", zap.String("id", id))
		utils.ErrorCount.WithLabelValues("DeleteHabit", "not_found").Inc()
		c.JSON(http.StatusNotFound, gin.H{"error": "Habit not found"})
		return
	}

	userInterface, exists := c.Get("user")
	if !exists {
		utils.ErrorCount.WithLabelValues("DeleteHabit", "auth").Inc()
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	currentUser, ok := userInterface.(models.User)
	if !ok {
		utils.Logger.Error("invalid_user_type", zap.String("type", fmt.Sprintf("%T", userInterface)))
		utils.ErrorCount.WithLabelValues("DeleteHabit", "auth").Inc()
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid user context"})
		return
	}

	if habit.UserID != currentUser.ID && currentUser.Role != models.RoleAdmin {
		utils.Logger.Warn("unauthorized_habit_delete",
			zap.String("habit_id", id),
			zap.Uint("user_id", currentUser.ID),
		)
		utils.ErrorCount.WithLabelValues("DeleteHabit", "forbidden").Inc()
		c.JSON(http.StatusForbidden, gin.H{"error": "Нет доступа к этой привычке"})
		return
	}

	if err := db.DB.Where("habit_id = ?", habit.ID).Delete(&models.HabitLog{}).Error; err != nil {
		utils.Logger.Error("db_delete_logs_failed", zap.Error(err))
		utils.ErrorCount.WithLabelValues("DeleteHabit", "database").Inc()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete habit logs"})
		return
	}

	if err := db.DB.Delete(&habit).Error; err != nil {
		utils.Logger.Error("db_delete_habit_failed", zap.Error(err))
		utils.ErrorCount.WithLabelValues("DeleteHabit", "database").Inc()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete habit"})
		return
	}

	utils.Logger.Info("habit_deleted", zap.String("habit_id", id))
	c.JSON(http.StatusOK, gin.H{"message": "Habit deleted"})
}
