package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Bekzhanizb/HabitTrackerBackend/cache"
	"github.com/Bekzhanizb/HabitTrackerBackend/db"
	"github.com/Bekzhanizb/HabitTrackerBackend/handlers"
	"github.com/Bekzhanizb/HabitTrackerBackend/middleware"
	"github.com/Bekzhanizb/HabitTrackerBackend/models"
	"github.com/Bekzhanizb/HabitTrackerBackend/routes"
	"github.com/Bekzhanizb/HabitTrackerBackend/services"
	"github.com/Bekzhanizb/HabitTrackerBackend/utils"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
)

func main() {
	utils.InitLogger()
	defer utils.Logger.Sync()
	utils.InitMetrics()

	utils.Logger.Info("starting_application", zap.String("version", "2.0.0"))

	db.Connect()

	if err := db.DB.AutoMigrate(
		&models.City{},
		&models.User{},
		&models.Habit{},
		&models.HabitLog{},
		&models.Achievement{},
		&models.Diary{},
	); err != nil {
		utils.Logger.Fatal("migration_failed", zap.Error(err))
	}

	if err := cache.InitRedis(utils.Logger); err != nil {
		utils.Logger.Fatal("redis_initialization_failed", zap.Error(err))
	}
	defer cache.Close()

	seedCities()

	gin.SetMode(gin.ReleaseMode)
	r := gin.New()

	r.Use(cors.New(cors.Config{
		AllowOrigins: []string{
			"http://localhost:3000",
		},
		AllowMethods: []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders: []string{
			"Origin",
			"Content-Type",
			"Accept",
			"Authorization",
			"X-CSRF-Token",
			"X-Requested-With",
		},
		ExposeHeaders: []string{
			"Content-Length",
			"X-CSRF-Token",
		},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	r.Use(middleware.Recovery())
	r.Use(middleware.RequestLogger())
	r.Use(middleware.SecurityHeaders())

	r.Use(middleware.RateLimitMiddleware(100, time.Minute))

	csrfMiddleware := middleware.CSRFMiddleware(
		[]byte("32-byte-long-supersecret-key-1234567890"),
		"/api/csrf",
		"/health",
		"/metrics",
		"/api/login",
		"/api/register",
		"/api/cities",
	)

	r.Use(csrfMiddleware)

	r.Static("/uploads", "./uploads")

	r.GET("/health", healthCheckHandler)

	r.GET("/api/csrf", middleware.GetCSRFToken())

	public := r.Group("/api")
	{
		public.POST("/register", handlers.RegisterHandler)
		public.POST("/login", routes.Login)
		public.GET("/cities", getCitiesHandler)
	}

	api := r.Group("/api")
	api.Use(handlers.AuthMiddleware())
	{
		api.GET("/profile", routes.Profile)
		api.PUT("/profile", routes.UpdateProfile)

		habits := api.Group("/habits")
		{
			habits.GET("", middleware.CacheMiddleware(2*time.Minute), handlers.GetHabits)
			habits.POST("", handlers.CreateHabit)
			habits.POST("/log", handlers.LogHabit)
			habits.PUT("/:id", handlers.UpdateHabit)
			habits.DELETE("/:id", handlers.DeleteHabit)
			habits.GET("/stats", getHabitStatsHandler)
			habits.GET("/logs",
				handlers.RoleMiddleware(models.RoleAdmin),
				middleware.CacheMiddleware(5*time.Minute),
				handlers.GetHabitLogs,
			)
			habits.POST("/bulk/activate",
				handlers.RoleMiddleware(models.RoleAdmin),
				bulkActivateHabitsHandler,
			)
		}

		diary := api.Group("/diary")
		{
			diary.GET("", middleware.CacheMiddleware(2*time.Minute), handlers.GetDiary)
			diary.POST("", handlers.CreateDiary)
			diary.PUT("/:id", handlers.UpdateDiary)
			diary.DELETE("/:id", handlers.DeleteDiary)
		}

		cacheAPI := api.Group("/cache")
		cacheAPI.Use(handlers.RoleMiddleware(models.RoleAdmin))
		{
			cacheAPI.DELETE("/clear", clearCacheHandler)
			cacheAPI.DELETE("/user/:id", clearUserCacheHandler)
		}
	}

	r.GET("/api/users", handlers.GetUsersHandler)

	r.Use(middleware.RequestLogger())
	r.GET("/debug/context", handlers.AuthMiddleware(), func(c *gin.Context) {
		userInterface, exists := c.Get("user")
		c.JSON(200, gin.H{
			"user_exists": exists,
			"user_type":   fmt.Sprintf("%T", userInterface),
			"user_value":  userInterface,
		})
	})

	r.GET("/metrics", gin.WrapH(promhttp.Handler()))

	startServerWithGracefulShutdown(r)
}

func seedCities() {
	var count int64
	db.DB.Model(&models.City{}).Count(&count)
	if count == 0 {
		cities := []models.City{
			{Name: "Almaty"},
			{Name: "Astana"},
			{Name: "Shymkent"},
			{Name: "Karaganda"},
			{Name: "Aktobe"},
			{Name: "Taraz"},
			{Name: "Pavlodar"},
		}
		if err := db.DB.Create(&cities).Error; err != nil {
			utils.Logger.Error("seed_cities_failed", zap.Error(err))
		} else {
			utils.Logger.Info("seed_cities_created", zap.Int("count", len(cities)))
		}
	}
}

func healthCheckHandler(c *gin.Context) {
	sqlDB, err := db.DB.DB()
	dbStatus := "connected"
	if err != nil || sqlDB.Ping() != nil {
		dbStatus = "disconnected"
	}

	redisStatus := "connected"
	if err := cache.Client.Ping(context.Background()).Err(); err != nil {
		redisStatus = "disconnected"
	}

	c.JSON(http.StatusOK, gin.H{
		"status":    "ok",
		"timestamp": time.Now(),
		"database":  dbStatus,
		"redis":     redisStatus,
		"version":   "2.0.0",
	})
}

func getCitiesHandler(c *gin.Context) {
	var cities []models.City
	cacheKey := "cities:all"

	if err := cache.Get(cacheKey, &cities); err == nil {
		c.Header("X-Cache", "HIT")
		c.JSON(http.StatusOK, cities)
		return
	}

	if err := db.DB.Find(&cities).Error; err != nil {
		utils.Logger.Error("get_cities_failed", zap.Error(err))
		utils.ErrorCount.WithLabelValues("GetCities", "database").Inc()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch cities"})
		return
	}

	cache.Set(cacheKey, cities, time.Hour)
	c.Header("X-Cache", "MISS")
	c.JSON(http.StatusOK, cities)
}

func getHabitStatsHandler(c *gin.Context) {
	userInterface, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	currentUser := userInterface.(models.User)

	stats, err := services.CalculateUserHabitStatsConcurrently(currentUser.ID, utils.Logger)
	if err != nil {
		utils.Logger.Error("calculate_stats_failed", zap.Error(err))
		utils.ErrorCount.WithLabelValues("GetHabitStats", "calculation").Inc()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to calculate statistics"})
		return
	}

	c.JSON(http.StatusOK, stats)
}

func bulkActivateHabitsHandler(c *gin.Context) {
	var req struct {
		HabitIDs []uint `json:"habit_ids" binding:"required"`
		IsActive bool   `json:"is_active"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request", "details": err.Error()})
		return
	}

	if err := services.BulkUpdateHabitsActiveStatus(req.HabitIDs, req.IsActive, utils.Logger); err != nil {
		utils.Logger.Error("bulk_update_failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update habits"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Habits updated successfully",
		"count":   len(req.HabitIDs),
	})
}

func clearCacheHandler(c *gin.Context) {
	if err := cache.Client.FlushDB(context.Background()).Err(); err != nil {
		utils.Logger.Error("cache_clear_failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to clear cache"})
		return
	}

	utils.Logger.Info("cache_cleared_by_admin")
	c.JSON(http.StatusOK, gin.H{"message": "Cache cleared successfully"})
}

func clearUserCacheHandler(c *gin.Context) {
	userID := c.Param("id")

	var id uint
	if _, err := fmt.Sscanf(userID, "%d", &id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	if err := middleware.InvalidateUserCache(id); err != nil {
		utils.Logger.Error("user_cache_clear_failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to clear user cache"})
		return
	}

	utils.Logger.Info("user_cache_cleared", zap.Uint("user_id", id))
	c.JSON(http.StatusOK, gin.H{"message": "User cache cleared"})
}

func startServerWithGracefulShutdown(router *gin.Engine) {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// TLS конфигурация для HTTPS
	tlsConfig := &tls.Config{
		MinVersion: tls.VersionTLS12,
		CipherSuites: []uint16{
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
		},
	}

	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
		TLSConfig:    tlsConfig,
	}

	utils.Logger.Info("starting_http_server",
		zap.String("port", port),
		zap.String("environment", gin.Mode()),
	)

	fmt.Println("\n🚀 ================================")
	fmt.Println("   Habit Tracker Backend v2.0")
	fmt.Println("   ================================")
	fmt.Printf("   🌐 Server:  http://localhost:%s\n", port)
	fmt.Printf("   📊 Metrics: http://localhost:%s/metrics\n", port)
	fmt.Printf("   ❤️  Health: http://localhost:%s/health\n", port)
	fmt.Printf("   🔒 Redis:   Connected\n")
	fmt.Printf("   💾 DB:      Connected\n")
	fmt.Println("   ================================\n")

	go func() {
		if gin.Mode() == gin.ReleaseMode && fileExists("./certs/server.crt") {
			utils.Logger.Info("starting_https_server")
			if err := srv.ListenAndServeTLS("./certs/server.crt", "./certs/server.key"); err != nil && err != http.ErrServerClosed {
				utils.Logger.Fatal("https_server_failed", zap.Error(err))
			}
		} else {
			if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				utils.Logger.Fatal("http_server_failed", zap.Error(err))
			}
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	utils.Logger.Info("shutting_down_server")
	fmt.Println("\n🛑 Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		utils.Logger.Fatal("server_forced_shutdown", zap.Error(err))
	}

	utils.Logger.Info("server_stopped")
	fmt.Println("✅ Server stopped gracefully")
}

func fileExists(filename string) bool {
	_, err := os.Stat(filename)
	return err == nil
}
