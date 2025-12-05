package middleware

import (
	"bytes"
	"fmt"
	"net/http"
	"time"

	"github.com/Bekzhanizb/HabitTrackerBackend/cache"
	"github.com/Bekzhanizb/HabitTrackerBackend/models"
	"github.com/Bekzhanizb/HabitTrackerBackend/utils"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// CacheMiddleware caches GET requests
func CacheMiddleware(duration time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Method != http.MethodGet {
			c.Next()
			return
		}

		userID := uint(0)
		if userInterface, exists := c.Get("user"); exists {
			if user, ok := userInterface.(models.User); ok {
				userID = user.ID
				utils.Logger.Debug("cache_user_found", zap.Uint("user_id", userID))
			} else {
				utils.Logger.Warn("cache_invalid_user_type",
					zap.String("expected", "models.User"),
					zap.String("actual", fmt.Sprintf("%T", userInterface)),
				)
			}
		} else {
			utils.Logger.Debug("cache_no_user_in_context")
		}

		cacheKey := fmt.Sprintf("cache:%d:%s?%s", userID, c.Request.URL.Path, c.Request.URL.RawQuery)

		utils.Logger.Debug("cache_check",
			zap.String("key", cacheKey),
			zap.Uint("user_id", userID))

		var cachedResponse CachedResponse
		if err := cache.Get(cacheKey, &cachedResponse); err == nil {
			utils.Logger.Info("cache_hit", zap.String("key", cacheKey))

			for key, values := range cachedResponse.Headers {
				for _, value := range values {
					c.Header(key, value)
				}
			}
			c.Header("X-Cache", "HIT")

			c.Data(cachedResponse.Status, cachedResponse.ContentType, cachedResponse.Body)
			c.Abort()
			return
		}

		utils.Logger.Info("cache_miss", zap.String("key", cacheKey))
		c.Header("X-Cache", "MISS")

		blw := &bodyLogWriter{body: bytes.NewBufferString(""), ResponseWriter: c.Writer}
		c.Writer = blw

		c.Next()

		if c.Writer.Status() == http.StatusOK {
			cachedResp := CachedResponse{
				Status:      c.Writer.Status(),
				ContentType: c.Writer.Header().Get("Content-Type"),
				Body:        blw.body.Bytes(),
				Headers:     c.Writer.Header(),
			}

			if err := cache.Set(cacheKey, cachedResp, duration); err != nil {
				utils.Logger.Warn("cache_set_failed",
					zap.Error(err),
					zap.String("key", cacheKey),
				)
			} else {
				utils.Logger.Info("cache_set_success",
					zap.String("key", cacheKey),
					zap.Duration("ttl", duration),
				)
			}
		}
	}
}

type CachedResponse struct {
	Status      int         `json:"status"`
	ContentType string      `json:"content_type"`
	Body        []byte      `json:"body"`
	Headers     http.Header `json:"headers"`
}

type bodyLogWriter struct {
	gin.ResponseWriter
	body *bytes.Buffer
}

func (w bodyLogWriter) Write(b []byte) (int, error) {
	w.body.Write(b)
	return w.ResponseWriter.Write(b)
}

func (w bodyLogWriter) WriteString(s string) (int, error) {
	w.body.WriteString(s)
	return w.ResponseWriter.WriteString(s)
}

// InvalidateUserCache invalidates all cache entries for a specific user
func InvalidateUserCache(userID uint) error {
	pattern := fmt.Sprintf("cache:%d:*", userID)
	utils.Logger.Info("invalidating_user_cache", zap.Uint("user_id", userID))
	return cache.DeletePattern(pattern)
}

// InvalidateHabitCache invalidates cache for habit-related endpoints
func InvalidateHabitCache(userID uint) error {
	patterns := []string{
		fmt.Sprintf("cache:%d:/api/habits", userID),
		fmt.Sprintf("cache:%d:/api/habits/logs", userID),
		fmt.Sprintf("user_stats:%d", userID),
	}

	for _, pattern := range patterns {
		if err := cache.Delete(pattern); err != nil {
			utils.Logger.Warn("cache_delete_failed",
				zap.String("pattern", pattern),
				zap.Error(err),
			)
			return err
		}
	}

	utils.Logger.Info("habit_cache_invalidated", zap.Uint("user_id", userID))
	return nil
}

// RateLimitMiddleware implements rate limiting using Redis
func RateLimitMiddleware(maxRequests int, window time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		clientIP := c.ClientIP()
		key := fmt.Sprintf("rate_limit:%s", clientIP)

		count, err := cache.IncrementCounter(key, window)
		if err != nil {
			utils.Logger.Error("rate_limit_error", zap.Error(err))
			c.Next()
			return
		}

		c.Header("X-RateLimit-Limit", fmt.Sprintf("%d", maxRequests))
		c.Header("X-RateLimit-Remaining", fmt.Sprintf("%d", max(0, maxRequests-int(count))))

		if count > int64(maxRequests) {
			utils.Logger.Warn("rate_limit_exceeded",
				zap.String("ip", clientIP),
				zap.Int64("count", count),
			)
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error": "Слишком много запросов. Попробуйте позже.",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
