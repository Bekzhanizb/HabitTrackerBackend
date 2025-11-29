package middleware

import (
	"bytes"
	"fmt"
	"net/http"
	"time"

	"github.com/Bekzhanizb/HabitTrackerBackend/cache"
	"github.com/Bekzhanizb/HabitTrackerBackend/utils"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// CacheMiddleware caches GET requests
func CacheMiddleware(duration time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Only cache GET requests
		if c.Request.Method != http.MethodGet {
			c.Next()
			return
		}

		// Generate cache key from URL and user ID
		userInterface, exists := c.Get("user")
		userID := uint(0)
		if exists {
			userID = userInterface.(gin.H)["id"].(uint)
		}

		cacheKey := fmt.Sprintf("cache:%d:%s", userID, c.Request.URL.Path)

		// Try to get from cache
		var cachedResponse CachedResponse
		if err := cache.Get(cacheKey, &cachedResponse); err == nil {
			utils.Logger.Info("cache_hit", zap.String("key", cacheKey))

			// Set cached headers
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

		// Cache miss - continue with request
		utils.Logger.Info("cache_miss", zap.String("key", cacheKey))
		c.Header("X-Cache", "MISS")

		// Capture response
		blw := &bodyLogWriter{body: bytes.NewBufferString(""), ResponseWriter: c.Writer}
		c.Writer = blw

		c.Next()

		// Cache successful responses
		if c.Writer.Status() == http.StatusOK {
			cachedResp := CachedResponse{
				Status:      c.Writer.Status(),
				ContentType: c.Writer.Header().Get("Content-Type"),
				Body:        blw.body.Bytes(),
				Headers:     c.Writer.Header(),
			}

			if err := cache.Set(cacheKey, cachedResp, duration); err != nil {
				utils.Logger.Warn("cache_set_failed", zap.Error(err))
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
			return err
		}
	}
	return nil
}

// RateLimitMiddleware implements rate limiting using Redis
func RateLimitMiddleware(maxRequests int, window time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		clientIP := c.ClientIP()
		key := fmt.Sprintf("rate_limit:%s", clientIP)

		// Increment counter
		count, err := cache.IncrementCounter(key, window)
		if err != nil {
			utils.Logger.Error("rate_limit_error", zap.Error(err))
			c.Next()
			return
		}

		// Set headers
		c.Header("X-RateLimit-Limit", fmt.Sprintf("%d", maxRequests))
		c.Header("X-RateLimit-Remaining", fmt.Sprintf("%d", maxRequests-int(count)))

		// Check if limit exceeded
		if count > int64(maxRequests) {
			utils.Logger.Warn("rate_limit_exceeded",
				zap.String("ip", clientIP),
				zap.Int64("count", count),
			)
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error": "Rate limit exceeded. Please try again later.",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}
