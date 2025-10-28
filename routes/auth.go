package routes

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/Bekzhanizb/HabitTrackerBackend/db"
	"github.com/Bekzhanizb/HabitTrackerBackend/models"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

var jwtKey = []byte("supersecretkey")

type Claims struct {
	UserID   uint
	Username string
	jwt.RegisteredClaims
}

func Register(c *gin.Context) {
	var input struct {
		Username string `json:"username"`
		Password string `json:"password"`
		CityID   uint   `json:"city_id"`
	}

	if err := c.BindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid data"})
		return
	}

	// Проверяем, что пользователь не существует
	var existing models.User
	if err := db.DB.Where("username = ?", input.Username).First(&existing).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "Username already exists"})
		return
	}

	// Проверяем, что город существует
	var city models.City
	if err := db.DB.First(&city, input.CityID).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid city ID"})
		return
	}

	// Хэшируем пароль
	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)

	user := models.User{
		Username:     input.Username,
		PasswordHash: string(hashedPassword),
		CityID:       &input.CityID,
		Role:         "user",
		Picture:      "/uploads/default.png",
	}

	if err := db.DB.Create(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user"})
		return
	}

	// Генерация JWT
	expirationTime := time.Now().Add(24 * time.Hour)
	claims := &Claims{
		UserID:   user.ID,
		Username: user.Username,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, _ := token.SignedString(jwtKey)

	// Отправляем ответ фронтенду
	c.JSON(http.StatusCreated, gin.H{
		"message": "User registered successfully",
		"token":   tokenString,
		"user": gin.H{
			"id":       user.ID,
			"username": user.Username,
			"city_id":  user.CityID,
			"role":     user.Role,
			"picture":  user.Picture,
		},
	})
}

func Login(c *gin.Context) {
	var input struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}

	if err := c.BindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid data"})
		return
	}

	var user models.User
	result := db.DB.Where("username = ?", input.Username).First(&user)
	if result.Error != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found"})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(input.Password)); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Incorrect password"})
		return
	}

	expirationTime := time.Now().Add(24 * time.Hour)
	claims := &Claims{
		UserID:   user.ID,
		Username: user.Username,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, _ := token.SignedString(jwtKey)

	c.JSON(http.StatusOK, gin.H{
		"token": tokenString,
		"user": gin.H{
			"id":       user.ID,
			"username": user.Username,
			"city_id":  user.CityID,
			"picture":  user.Picture,
			"role":     user.Role,
		},
	})
}

// UpdateProfile
func UpdateProfile(c *gin.Context) {
	user, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	currentUser := user.(models.User)

	username := c.PostForm("username")
	cityID := c.PostForm("city_id")

	// Uploads
	file, err := c.FormFile("picture")
	if err == nil {
		path := fmt.Sprintf("./uploads/%d_%s", currentUser.ID, file.Filename)
		if err := c.SaveUploadedFile(file, path); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to upload file"})
			return
		}
		currentUser.Picture = strings.TrimPrefix(path, ".")
	}

	if username != "" {
		currentUser.Username = username
	}
	if cityID != "" {
		var city models.City
		if err := db.DB.First(&city, cityID).Error; err == nil {
			currentUser.CityID = &city.ID
		}
	}

	db.DB.Save(&currentUser)
	c.JSON(http.StatusOK, gin.H{"message": "Profile updated", "user": currentUser})
}

func Profile(c *gin.Context) {
	user, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	c.JSON(http.StatusOK, user)
}
