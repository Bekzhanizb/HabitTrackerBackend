package handlers

import (
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"github.com/Bekzhanizb/HabitTrackerBackend/db"
	"github.com/Bekzhanizb/HabitTrackerBackend/models"
	"github.com/Bekzhanizb/HabitTrackerBackend/utils"
	"github.com/gin-gonic/gin"
)

func RegisterHandler(c *gin.Context) {
	username := c.PostForm("username")
	password := c.PostForm("password")
	cityIDStr := c.PostForm("city_id")

	if username == "" || password == "" || cityIDStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Заполните все поля"})
		return
	}

	cityID, err := strconv.Atoi(cityIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Неверный ID города"})
		return
	}

	// Проверяем, есть ли пользователь с таким именем
	var existing models.User
	if err := db.DB.Where("username = ?", username).First(&existing).Error; err == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Пользователь уже существует"})
		return
	}

	// Хэшируем пароль
	hashedPassword, err := utils.HashPassword(password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка хэширования пароля"})
		return
	}

	// Обработка аватара (если есть)
	file, err := c.FormFile("avatar")
	avatarPath := ""
	if err == nil {
		os.MkdirAll("uploads/avatars", os.ModePerm)
		avatarPath = filepath.Join("uploads/avatars", file.Filename)
		avatarPath = "/" + avatarPath
		if err := c.SaveUploadedFile(file, avatarPath); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка сохранения файла"})
			return
		}
	}

	// Создаём пользователя
	cityIDUint := uint(cityID)
	user := models.User{
		Username:     username,
		PasswordHash: hashedPassword,
		CityID:       &cityIDUint, // Теперь это указатель
		Picture:      avatarPath,
	}

	if err := db.DB.Create(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка создания пользователя"})
		return
	}

	// Генерируем JWT токен
	token, err := utils.GenerateToken(user.ID, user.Username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка генерации токена"})
		return
	}

	// Возвращаем успешный ответ
	c.JSON(http.StatusOK, gin.H{
		"message": "Регистрация успешна",
		"user": gin.H{
			"id":       user.ID,
			"username": user.Username,
			"city_id":  user.CityID,
			"avatar":   user.Picture,
		},
		"token": token,
	})
}
