package routes

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/Bekzhanizb/HabitTrackerBackend/db"
	"github.com/Bekzhanizb/HabitTrackerBackend/models"
	"github.com/Bekzhanizb/HabitTrackerBackend/utils"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
)

var jwtKey = []byte("supersecretkey")

type Claims struct {
	UserID   uint   `json:"user_id"`
	Username string `json:"username"`
	Role     string `json:"role"`
	jwt.RegisteredClaims
}

func Register(c *gin.Context) {
	var input struct {
		Username string `json:"username" binding:"required,min=3,max=50"`
		Password string `json:"password" binding:"required,min=6"`
		CityID   uint   `json:"city_id" binding:"required"`
	}

	if err := c.BindJSON(&input); err != nil {
		utils.Logger.Warn("invalid_register_request", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid data", "details": err.Error()})
		return
	}

	var existing models.User
	if err := db.DB.Where("username = ?", input.Username).First(&existing).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "Username already exists"})
		return
	}

	var city models.City
	if err := db.DB.First(&city, input.CityID).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid city ID"})
		return
	}

	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte(input.Password), 14)

	user := models.User{
		Username:     input.Username,
		PasswordHash: string(hashedPassword),
		CityID:       &input.CityID,
		Role:         models.RoleUser,
		Picture:      "/uploads/default.png",
	}

	if err := db.DB.Create(&user).Error; err != nil {
		utils.Logger.Error("db_create_user_failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user"})
		return
	}

	expirationTime := time.Now().Add(24 * time.Hour)
	claims := &Claims{
		UserID:   user.ID,
		Username: user.Username,
		Role:     user.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, _ := token.SignedString(jwtKey)

	utils.Logger.Info("user_registered", zap.Uint("user_id", user.ID), zap.String("username", user.Username))

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
		Username string `json:"username" binding:"required"`
		Password string `json:"password" binding:"required"`
	}

	if err := c.BindJSON(&input); err != nil {
		utils.Logger.Warn("invalid_login_request", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid data"})
		return
	}

	var user models.User
	result := db.DB.Where("username = ?", input.Username).First(&user)
	if result.Error != nil {
		utils.Logger.Warn("login_user_not_found", zap.String("username", input.Username))
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found"})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(input.Password)); err != nil {
		utils.Logger.Warn("login_incorrect_password", zap.String("username", input.Username))
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Incorrect password"})
		return
	}

	expirationTime := time.Now().Add(24 * time.Hour)
	claims := &Claims{
		UserID:   user.ID,
		Username: user.Username,
		Role:     user.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, _ := token.SignedString(jwtKey)

	utils.Logger.Info("user_logged_in", zap.Uint("user_id", user.ID))

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
func UpdateProfile(c *gin.Context) {
	user, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	currentUser := user.(models.User)
	username := c.PostForm("username")
	cityID := c.PostForm("city_id")

	file, err := c.FormFile("picture")
	if err == nil {
		path := fmt.Sprintf("./uploads/%d_%s", currentUser.ID, file.Filename)
		if err := c.SaveUploadedFile(file, path); err != nil {
			utils.Logger.Error("file_upload_failed", zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to upload file"})
			return
		}
		currentUser.Picture = strings.TrimPrefix(path, ".")
	}

	if username != "" && username != currentUser.Username {
		currentUser.Username = username
	}
	if cityID != "" {
		var city models.City
		if err := db.DB.First(&city, cityID).Error; err == nil {
			currentUser.CityID = &city.ID
		}
	}

	db.DB.Save(&currentUser)
	utils.Logger.Info("profile_updated", zap.Uint("user_id", currentUser.ID))
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
