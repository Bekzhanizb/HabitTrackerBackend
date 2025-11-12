package handlers

import (
	"net/http"

	"github.com/Bekzhanizb/HabitTrackerBackend/db"
	"github.com/Bekzhanizb/HabitTrackerBackend/models"
	"github.com/gin-gonic/gin"
)

// CreateDiary создает новую запись в дневнике
func CreateDiary(c *gin.Context) {
	var diary models.Diary
	if err := c.ShouldBindJSON(&diary); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Некорректные данные"})
		return
	}

	if diary.Title == "" || diary.Content == "" || diary.UserID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Необходимо заполнить обязательные поля"})
		return
	}

	if err := db.DB.Create(&diary).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка при создании записи"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Запись успешно создана", "diary": diary})
}

// GetDiary получает все записи пользователя
func GetDiary(c *gin.Context) {
	userInterface, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	currentUser := userInterface.(models.User)

	var diaries []models.Diary
	query := db.DB

	// Обычный пользователь видит только свои записи
	if currentUser.Role != models.RoleAdmin {
		query = query.Where("user_id = ?", currentUser.ID)
	} else {
		// admin может фильтровать по user_id через query
		userID := c.Query("user_id")
		if userID != "" {
			query = query.Where("user_id = ?", userID)
		}
	}

	if err := query.Find(&diaries).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка при получении записей"})
		return
	}

	c.JSON(http.StatusOK, diaries)
}

// UpdateDiary обновляет запись в дневнике
func UpdateDiary(c *gin.Context) {
	id := c.Param("id")
	var diary models.Diary

	if err := db.DB.First(&diary, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Запись не найдена"})
		return
	}

	var input struct {
		Title   *string `json:"title"`
		Content *string `json:"content"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Некорректные данные"})
		return
	}

	if input.Title != nil {
		diary.Title = *input.Title
	}
	if input.Content != nil {
		diary.Content = *input.Content
	}

	if err := db.DB.Save(&diary).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка при обновлении записи"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Запись обновлена", "diary": diary})
}

// DeleteDiary удаляет запись из дневника
func DeleteDiary(c *gin.Context) {
	id := c.Param("id")

	var diary models.Diary
	if err := db.DB.First(&diary, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Запись не найдена"})
		return
	}

	if err := db.DB.Delete(&diary).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка при удалении записи"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Запись удалена"})
}
