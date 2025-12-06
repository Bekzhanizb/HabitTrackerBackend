package middleware

import (
	"strconv"
	"time"

	"github.com/Bekzhanizb/HabitTrackerBackend/utils"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func RequestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.FullPath()
		if path == "" {
			path = c.Request.URL.Path
		}

		c.Next()

		status := c.Writer.Status()
		duration := time.Since(start).Seconds()

		utils.ReqCount.WithLabelValues(
			c.Request.Method,
			path,
			strconv.Itoa(status),
		).Inc()

		utils.ReqDuration.WithLabelValues(
			c.Request.Method,
			path,
		).Observe(duration)

		utils.Logger.Info("http_request",
			zap.String("method", c.Request.Method),
			zap.String("path", path),
			zap.Int("status", status),
			zap.Float64("duration", duration),
			zap.String("client_ip", c.ClientIP()),
		)
	}
}
