package handlers

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"github.com/Bekzhanizb/HabitTrackerBackend/db"
	"github.com/Bekzhanizb/HabitTrackerBackend/models"
	"github.com/Bekzhanizb/HabitTrackerBackend/utils"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func RegisterHandler(c *gin.Context) {
	username := c.PostForm("username")
	password := c.PostForm("password")

	// 🔥 FIX: Принимаем city_id (как отправляет frontend)
	cityIDStr := c.PostForm("city_id")

	utils.Logger.Info("register_attempt",
		zap.String("username", username),
		zap.String("city_id_str", cityIDStr),
		zap.Bool("has_password", password != ""),
	)

	// Валидация входных данных
	if username == "" || password == "" || cityIDStr == "" {
		utils.Logger.Warn("register_validation_failed",
			zap.Bool("has_username", username != ""),
			zap.Bool("has_password", password != ""),
			zap.Bool("has_city_id", cityIDStr != ""),
		)
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Заполните все поля",
			"details": "username, password и city_id обязательны",
		})
		return
	}

	// Валидация username
	if len(username) < 3 || len(username) > 50 {
		utils.Logger.Warn("register_invalid_username_length", zap.Int("length", len(username)))
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Имя пользователя должно быть от 3 до 50 символов",
		})
		return
	}

	if len(password) < 4 {
		utils.Logger.Warn("register_password_too_short")
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Пароль должен быть не менее 4 символов",
		})
		return
	}

	cityID, err := strconv.Atoi(cityIDStr)
	if err != nil {
		utils.Logger.Warn("register_invalid_city_id",
			zap.String("city_id", cityIDStr),
			zap.Error(err),
		)
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Неверный ID города",
			"details": err.Error(),
		})
		return
	}

	var city models.City
	if err := db.DB.First(&city, cityID).Error; err != nil {
		utils.Logger.Warn("register_city_not_found", zap.Int("city_id", cityID))
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Город не найден",
		})
		return
	}

	var existing models.User
	if err := db.DB.Where("username = ?", username).First(&existing).Error; err == nil {
		utils.Logger.Warn("register_user_exists", zap.String("username", username))
		c.JSON(http.StatusConflict, gin.H{
			"error": "Пользователь с таким именем уже существует",
		})
		return
	}

	hashedPassword, err := utils.HashPassword(password)
	if err != nil {
		utils.Logger.Error("register_hash_failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Ошибка хэширования пароля",
		})
		return
	}

	avatarPath := "/uploads/default.png"
	file, err := c.FormFile("avatar")

	if err == nil {
		if err := os.MkdirAll("./uploads", os.ModePerm); err != nil {
			utils.Logger.Error("register_mkdir_failed", zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Ошибка создания директории",
			})
			return
		}

		ext := filepath.Ext(file.Filename)
		filename := fmt.Sprintf("%s_%d%s", username, cityID, ext)
		filePath := filepath.Join("./uploads", filename)

		utils.Logger.Info("register_saving_avatar",
			zap.String("filename", filename),
			zap.String("path", filePath),
		)

		if err := c.SaveUploadedFile(file, filePath); err != nil {
			utils.Logger.Error("register_save_file_failed", zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "Ошибка сохранения файла",
				"details": err.Error(),
			})
			return
		}

		avatarPath = "/" + filepath.ToSlash(filePath[2:]) // Убираем "./" из пути
		utils.Logger.Info("register_avatar_saved", zap.String("path", avatarPath))
	} else {
		utils.Logger.Info("register_no_avatar", zap.Error(err))
	}

	cityIDUint := uint(cityID)
	user := models.User{
		Username:     username,
		PasswordHash: hashedPassword,
		CityID:       &cityIDUint,
		Picture:      avatarPath,
		Role:         models.RoleUser,
	}

	if err := db.DB.Create(&user).Error; err != nil {
		utils.Logger.Error("register_db_create_failed",
			zap.Error(err),
			zap.String("username", username),
		)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Ошибка создания пользователя",
			"details": err.Error(),
		})
		return
	}

	token, err := utils.GenerateToken(user.ID, user.Username)
	if err != nil {
		utils.Logger.Error("register_token_generation_failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Ошибка генерации токена",
		})
		return
	}

	utils.Logger.Info("register_success",
		zap.Uint("user_id", user.ID),
		zap.String("username", user.Username),
	)

	c.JSON(http.StatusCreated, gin.H{
		"message": "Регистрация успешна",
		"user": gin.H{
			"id":       user.ID,
			"username": user.Username,
			"city_id":  user.CityID,
			"picture":  user.Picture,
			"role":     user.Role,
		},
		"token": token,
	})
}
