package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Bekzhanizb/HabitTrackerBackend/db"
	"github.com/Bekzhanizb/HabitTrackerBackend/handlers"
	"github.com/Bekzhanizb/HabitTrackerBackend/middleware"
	"github.com/Bekzhanizb/HabitTrackerBackend/models"
	"github.com/Bekzhanizb/HabitTrackerBackend/routes"
	"github.com/Bekzhanizb/HabitTrackerBackend/utils"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
)

func main() {
	// Инициализация логирования и метрик
	utils.InitLogger()
	defer utils.Logger.Sync()
	utils.InitMetrics()

	utils.Logger.Info("starting_application")

	// Подключение к БД
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

	// Создаем тестовые города, если их нет
	seedCities()

	// Настройка Gin
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()

	// Middleware в правильном порядке
	r.Use(middleware.Recovery())
	r.Use(middleware.RequestLogger())
	r.Use(middleware.SecurityHeaders())

	// CORS
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"http://localhost:3000"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	// Статика
	r.Static("/uploads", "./uploads")

	// Health check
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":    "ok",
			"timestamp": time.Now(),
			"database":  "connected",
		})
	})

	// Публичные эндпоинты
	r.POST("/api/register", routes.Register)
	r.POST("/api/login", routes.Login)
	r.GET("/api/cities", func(c *gin.Context) {
		var cities []models.City
		if err := db.DB.Find(&cities).Error; err != nil {
			utils.Logger.Error("get_cities_failed", zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, cities)
	})

	// Защищенные эндпоинты
	api := r.Group("/api")
	api.Use(handlers.AuthMiddleware())
	{
		// Профиль
		api.GET("/profile", routes.Profile)
		api.PUT("/profile", routes.UpdateProfile)

		// Привычки
		api.GET("/habits", handlers.GetHabits)
		api.POST("/habits", handlers.CreateHabit)
		api.POST("/habits/log", handlers.LogHabit)
		api.PUT("/habits/:id", handlers.UpdateHabit)
		api.DELETE("/habits/:id", handlers.DeleteHabit)

		// Логи привычек (только admin)
		api.GET("/habits/logs", handlers.RoleMiddleware(models.RoleAdmin), handlers.GetHabitLogs)

		// Дневник
		api.GET("/diary", handlers.GetDiary)
		api.POST("/diary", handlers.CreateDiary)
		api.PUT("/diary/:id", handlers.UpdateDiary)
		api.DELETE("/diary/:id", handlers.DeleteDiary)
	}

	// Метрики Prometheus
	r.GET("/metrics", gin.WrapH(promhttp.Handler()))

	// Запуск сервера
	startServer(r)
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
		}
		db.DB.Create(&cities)
		fmt.Println("✅ Seed cities created")
	}
}

func startServer(router *gin.Engine) {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	utils.Logger.Info("starting_http_server", zap.String("port", port))

	fmt.Println("\n🚀 ================================")
	fmt.Println("   Habit Tracker Backend Started")
	fmt.Println("   ================================")
	fmt.Printf("   🌐 Server:  http://localhost:%s\n", port)
	fmt.Printf("   📊 Metrics: http://localhost:%s/metrics\n", port)
	fmt.Printf("   ❤️  Health: http://localhost:%s/health\n", port)
	fmt.Println("   ================================\n")

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			utils.Logger.Fatal("http_server_failed", zap.Error(err))
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	utils.Logger.Info("shutting_down_server")
	fmt.Println("\n🛑 Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		utils.Logger.Fatal("server_forced_shutdown", zap.Error(err))
	}

	utils.Logger.Info("server_stopped")
	fmt.Println("✅ Server stopped gracefully")
}
