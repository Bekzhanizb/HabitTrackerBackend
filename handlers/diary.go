package handlers

import (
	"net/http"

	"github.com/Bekzhanizb/HabitTrackerBackend/db"
	"github.com/Bekzhanizb/HabitTrackerBackend/middleware"
	"github.com/Bekzhanizb/HabitTrackerBackend/models"
	"github.com/Bekzhanizb/HabitTrackerBackend/utils"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type CreateDiaryRequest struct {
	Title   string `json:"title" binding:"required,min=1,max=200"`
	Content string `json:"content" binding:"required,min=1,max=10000"`
	UserID  uint   `json:"user_id" binding:"required,min=1"`
}

func CreateDiary(c *gin.Context) {
	var req CreateDiaryRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Logger.Warn("invalid_create_diary_request", zap.Error(err))
		utils.ErrorCount.WithLabelValues("CreateDiary", "validation").Inc()
		c.JSON(http.StatusBadRequest, gin.H{"error": "Некорректные данные", "details": err.Error()})
		return
	}

	if err := middleware.ValidateStruct(req); err != nil {
		utils.ErrorCount.WithLabelValues("CreateDiary", "validation").Inc()
		c.JSON(http.StatusBadRequest, gin.H{"error": "Ошибка валидации", "details": err.Error()})
		return
	}

	userInterface, exists := c.Get("user")
	if !exists {
		utils.ErrorCount.WithLabelValues("CreateDiary", "auth").Inc()
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	currentUser := userInterface.(models.User)

	if currentUser.Role != models.RoleAdmin && req.UserID != currentUser.ID {
		utils.Logger.Warn("unauthorized_diary_creation",
			zap.Uint("current_user_id", currentUser.ID),
			zap.Uint("requested_user_id", req.UserID),
		)
		utils.ErrorCount.WithLabelValues("CreateDiary", "forbidden").Inc()
		c.JSON(http.StatusForbidden, gin.H{"error": "Нет доступа"})
		return
	}

	diary := models.Diary{
		UserID:  req.UserID,
		Title:   req.Title,
		Content: req.Content,
	}

	if err := db.DB.Create(&diary).Error; err != nil {
		utils.Logger.Error("db_create_diary_failed", zap.Error(err))
		utils.ErrorCount.WithLabelValues("CreateDiary", "database").Inc()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка при создании записи"})
		return
	}

	utils.Logger.Info("diary_created",
		zap.Uint("diary_id", diary.ID),
		zap.Uint("user_id", req.UserID),
	)

	c.JSON(http.StatusOK, gin.H{"message": "Запись успешно создана", "diary": diary})
}

func GetDiary(c *gin.Context) {
	userInterface, exists := c.Get("user")
	if !exists {
		utils.ErrorCount.WithLabelValues("GetDiary", "auth").Inc()
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	currentUser := userInterface.(models.User)

	var diaries []models.Diary
	query := db.DB

	if currentUser.Role != models.RoleAdmin {
		query = query.Where("user_id = ?", currentUser.ID)
	} else {
		userID := c.Query("user_id")
		if userID != "" {
			query = query.Where("user_id = ?", userID)
		}
	}

	if err := query.Order("created_at DESC").Find(&diaries).Error; err != nil {
		utils.Logger.Error("db_get_diary_failed", zap.Error(err))
		utils.ErrorCount.WithLabelValues("GetDiary", "database").Inc()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка при получении записей"})
		return
	}

	c.JSON(http.StatusOK, diaries)
}

type UpdateDiaryRequest struct {
	Title   *string `json:"title" binding:"omitempty,min=1,max=200"`
	Content *string `json:"content" binding:"omitempty,min=1,max=10000"`
}

func UpdateDiary(c *gin.Context) {
	id := c.Param("id")

	var diary models.Diary
	if err := db.DB.First(&diary, id).Error; err != nil {
		utils.Logger.Warn("diary_not_found", zap.String("id", id))
		utils.ErrorCount.WithLabelValues("UpdateDiary", "not_found").Inc()
		c.JSON(http.StatusNotFound, gin.H{"error": "Запись не найдена"})
		return
	}

	userInterface, exists := c.Get("user")
	if !exists {
		utils.ErrorCount.WithLabelValues("UpdateDiary", "auth").Inc()
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	currentUser := userInterface.(models.User)

	if diary.UserID != currentUser.ID && currentUser.Role != models.RoleAdmin {
		utils.Logger.Warn("unauthorized_diary_update",
			zap.String("diary_id", id),
			zap.Uint("user_id", currentUser.ID),
		)
		utils.ErrorCount.WithLabelValues("UpdateDiary", "forbidden").Inc()
		c.JSON(http.StatusForbidden, gin.H{"error": "Нет доступа"})
		return
	}

	var req UpdateDiaryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Logger.Warn("invalid_update_diary_request", zap.Error(err))
		utils.ErrorCount.WithLabelValues("UpdateDiary", "validation").Inc()
		c.JSON(http.StatusBadRequest, gin.H{"error": "Некорректные данные", "details": err.Error()})
		return
	}

	if req.Title != nil {
		diary.Title = *req.Title
	}
	if req.Content != nil {
		diary.Content = *req.Content
	}

	if err := db.DB.Save(&diary).Error; err != nil {
		utils.Logger.Error("db_update_diary_failed", zap.Error(err))
		utils.ErrorCount.WithLabelValues("UpdateDiary", "database").Inc()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка при обновлении записи"})
		return
	}

	utils.Logger.Info("diary_updated", zap.String("diary_id", id))
	c.JSON(http.StatusOK, gin.H{"message": "Запись обновлена", "diary": diary})
}

func DeleteDiary(c *gin.Context) {
	id := c.Param("id")

	var diary models.Diary
	if err := db.DB.First(&diary, id).Error; err != nil {
		utils.Logger.Warn("diary_not_found_for_delete", zap.String("id", id))
		utils.ErrorCount.WithLabelValues("DeleteDiary", "not_found").Inc()
		c.JSON(http.StatusNotFound, gin.H{"error": "Запись не найдена"})
		return
	}

	userInterface, exists := c.Get("user")
	if !exists {
		utils.ErrorCount.WithLabelValues("DeleteDiary", "auth").Inc()
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	currentUser := userInterface.(models.User)

	if diary.UserID != currentUser.ID && currentUser.Role != models.RoleAdmin {
		utils.Logger.Warn("unauthorized_diary_delete",
			zap.String("diary_id", id),
			zap.Uint("user_id", currentUser.ID),
		)
		utils.ErrorCount.WithLabelValues("DeleteDiary", "forbidden").Inc()
		c.JSON(http.StatusForbidden, gin.H{"error": "Нет доступа"})
		return
	}

	if err := db.DB.Delete(&diary).Error; err != nil {
		utils.Logger.Error("db_delete_diary_failed", zap.Error(err))
		utils.ErrorCount.WithLabelValues("DeleteDiary", "database").Inc()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка при удалении записи"})
		return
	}

	utils.Logger.Info("diary_deleted", zap.String("diary_id", id))
	c.JSON(http.StatusOK, gin.H{"message": "Запись удалена"})
}
