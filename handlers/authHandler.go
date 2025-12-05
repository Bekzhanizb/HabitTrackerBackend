package handlers

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/Bekzhanizb/HabitTrackerBackend/db"
	"github.com/Bekzhanizb/HabitTrackerBackend/middleware"
	"github.com/Bekzhanizb/HabitTrackerBackend/models"
	"github.com/Bekzhanizb/HabitTrackerBackend/utils"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"go.uber.org/zap"
)

func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		utils.Logger.Info("AuthMiddleware started", zap.String("path", c.Request.URL.Path))

		tokenString := c.GetHeader("Authorization")
		if tokenString == "" || !strings.HasPrefix(tokenString, "Bearer ") {
			utils.Logger.Warn("missing_or_invalid_token", zap.String("auth_header", tokenString))
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Missing or invalid token"})
			c.Abort()
			return
		}

		tokenString = strings.TrimPrefix(tokenString, "Bearer ")
		utils.Logger.Info("token_extracted", zap.String("token_length", string(rune(len(tokenString)))))

		claims := jwt.MapClaims{}
		token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
			return middleware.JwtKey, nil
		})

		if err != nil {
			utils.Logger.Warn("token_parse_error", zap.Error(err))
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired token", "details": err.Error()})
			c.Abort()
			return
		}

		if !token.Valid {
			utils.Logger.Warn("token_invalid")
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
			c.Abort()
			return
		}

		utils.Logger.Info("token_valid", zap.Any("claims", claims))

		userIDFloat, ok := claims["user_id"].(float64)
		if !ok {
			utils.Logger.Error("user_id_not_found_in_claims", zap.Any("claims", claims))
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token claims"})
			c.Abort()
			return
		}

		userID := uint(userIDFloat)
		utils.Logger.Info("user_id_extracted", zap.Uint("user_id", userID))

		var user models.User
		if err := db.DB.First(&user, userID).Error; err != nil {
			utils.Logger.Warn("user_not_found_in_db",
				zap.Uint("user_id", userID),
				zap.Error(err))
			c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found"})
			c.Abort()
			return
		}

		utils.Logger.Info("user_loaded_from_db",
			zap.Uint("user_id", user.ID),
			zap.String("username", user.Username),
			zap.String("role", user.Role))

		c.Set("user", user)
		c.Set("role", user.Role)

		utils.Logger.Info("user_set_in_context", zap.Uint("user_id", user.ID))

		c.Next()
	}
}

func RoleMiddleware(requiredRoles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		userInterface, exists := c.Get("user")
		if !exists {
			utils.Logger.Warn("role_middleware_user_not_found")
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
			c.Abort()
			return
		}

		user, ok := userInterface.(models.User)
		if !ok {
			utils.Logger.Error("role_middleware_invalid_user_type",
				zap.String("type", fmt.Sprintf("%T", userInterface)))
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid user"})
			c.Abort()
			return
		}

		roleMatched := false
		for _, role := range requiredRoles {
			if user.Role == role {
				roleMatched = true
				break
			}
		}

		if !roleMatched {
			utils.Logger.Warn("role_middleware_forbidden",
				zap.String("user_role", user.Role),
				zap.Strings("required_roles", requiredRoles))
			c.JSON(http.StatusForbidden, gin.H{"error": "Forbidden"})
			c.Abort()
			return
		}

		c.Next()
	}
}
