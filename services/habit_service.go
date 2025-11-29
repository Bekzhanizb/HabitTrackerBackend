package services

import (
	"fmt"
	"sync"
	"time"

	"github.com/Bekzhanizb/HabitTrackerBackend/cache"
	"github.com/Bekzhanizb/HabitTrackerBackend/db"
	"github.com/Bekzhanizb/HabitTrackerBackend/models"
	"go.uber.org/zap"
)

type HabitStats struct {
	HabitID        uint    `json:"habit_id"`
	TotalLogs      int     `json:"total_logs"`
	CompletedLogs  int     `json:"completed_logs"`
	CompletionRate float64 `json:"completion_rate"`
	CurrentStreak  int     `json:"current_streak"`
	LongestStreak  int     `json:"longest_streak"`
	Error          error   `json:"-"`
}

type UserHabitStats struct {
	UserID         uint          `json:"user_id"`
	TotalHabits    int           `json:"total_habits"`
	ActiveHabits   int           `json:"active_habits"`
	OverallRate    float64       `json:"overall_completion_rate"`
	HabitStats     []HabitStats  `json:"habit_stats"`
	ProcessingTime time.Duration `json:"processing_time_ms"`
}

/*
╔═══════════════════════════════════════════════════════════════════╗
║  ОБОСНОВАНИЕ ИСПОЛЬЗОВАНИЯ CONCURRENCY (GOROUTINES + CHANNELS)    ║
╚═══════════════════════════════════════════════════════════════════╝

1. НЕЗАВИСИМЫЕ ВЫЧИСЛЕНИЯ:
   - Статистика каждой привычки вычисляется НЕЗАВИСИМО
   - Нет shared state между вычислениями
   - Идеальный кандидат для параллелизма

2. I/O ОПЕРАЦИИ:
   - Каждая горутина делает отдельный DB query
   - Database queries могут выполняться параллельно
   - Пока одна горутина ждёт DB, другие работают

3. ПРОИЗВОДИТЕЛЬНОСТЬ:
   Последовательно: 10 привычек × 50ms = 500ms
   Параллельно: max(50ms) + overhead ≈ 60ms
   УСКОРЕНИЕ: ~8x быстрее!

4. МАСШТАБИРУЕМОСТЬ:
   - При росте числа пользователей критично важно
   - У пользователя может быть 20-30 привычек
   - Без concurrency: 30 × 50ms = 1.5 секунды (плохо!)
   - С concurrency: ~70ms (отлично!)

5. ИСПОЛЬЗОВАНИЕ CHANNELS:
   - statsChan - для сбора результатов от горутин
   - errChan - для обработки ошибок
   - WaitGroup - для синхронизации завершения
*/

// CalculateUserHabitStatsConcurrently - MAIN CONCURRENT FUNCTION
func CalculateUserHabitStatsConcurrently(userID uint, logger *zap.Logger) (*UserHabitStats, error) {
	startTime := time.Now()

	// Check cache first
	cacheKey := fmt.Sprintf("user_stats:%d", userID)
	var cachedStats UserHabitStats
	if err := cache.Get(cacheKey, &cachedStats); err == nil {
		logger.Info("cache_hit", zap.String("key", cacheKey))
		return &cachedStats, nil
	}

	// Get all user habits
	var habits []models.Habit
	if err := db.DB.Where("user_id = ?", userID).Find(&habits).Error; err != nil {
		return nil, err
	}

	if len(habits) == 0 {
		return &UserHabitStats{UserID: userID}, nil
	}

	// 🔥 СОЗДАЁМ CHANNEL ДЛЯ РЕЗУЛЬТАТОВ
	statsChan := make(chan HabitStats, len(habits))
	var wg sync.WaitGroup

	// 🚀 ЗАПУСКАЕМ ГОРУТИНУ ДЛЯ КАЖДОЙ ПРИВЫЧКИ
	for _, habit := range habits {
		wg.Add(1)
		// Каждая горутина работает ПАРАЛЛЕЛЬНО!
		go func(h models.Habit) {
			defer wg.Done()
			stats := calculateSingleHabitStats(h.ID, logger)
			statsChan <- stats // Отправляем результат в channel
		}(habit)
	}

	// Закрываем channel когда все горутины завершатся
	go func() {
		wg.Wait()
		close(statsChan)
	}()

	// 📊 СОБИРАЕМ РЕЗУЛЬТАТЫ ИЗ CHANNEL
	var habitStats []HabitStats
	var totalRate float64
	activeCount := 0

	// Читаем из channel пока он не закроется
	for stat := range statsChan {
		if stat.Error != nil {
			logger.Warn("habit_stats_error",
				zap.Uint("habit_id", stat.HabitID),
				zap.Error(stat.Error),
			)
			continue
		}
		habitStats = append(habitStats, stat)
		totalRate += stat.CompletionRate
	}

	for _, h := range habits {
		if h.IsActive {
			activeCount++
		}
	}

	overallRate := 0.0
	if len(habitStats) > 0 {
		overallRate = totalRate / float64(len(habitStats))
	}

	result := &UserHabitStats{
		UserID:         userID,
		TotalHabits:    len(habits),
		ActiveHabits:   activeCount,
		OverallRate:    overallRate,
		HabitStats:     habitStats,
		ProcessingTime: time.Since(startTime),
	}

	// Cache результат
	cache.Set(cacheKey, result, 5*time.Minute)

	logger.Info("stats_calculated_concurrently",
		zap.Uint("user_id", userID),
		zap.Int("habits_count", len(habits)),
		zap.Duration("duration", result.ProcessingTime),
	)

	return result, nil
}

func calculateSingleHabitStats(habitID uint, logger *zap.Logger) HabitStats {
	stats := HabitStats{HabitID: habitID}

	var logs []models.HabitLog
	if err := db.DB.Where("habit_id = ?", habitID).
		Order("date DESC").
		Find(&logs).Error; err != nil {
		stats.Error = err
		return stats
	}

	stats.TotalLogs = len(logs)
	completedCount := 0

	for _, log := range logs {
		if log.IsCompleted {
			completedCount++
		}
	}
	stats.CompletedLogs = completedCount

	if stats.TotalLogs > 0 {
		stats.CompletionRate = float64(completedCount) / float64(stats.TotalLogs) * 100
	}

	// Calculate streaks
	currentStreak := 0
	longestStreak := 0
	tempStreak := 0

	for i, log := range logs {
		if log.IsCompleted {
			tempStreak++
			if i == 0 {
				currentStreak = tempStreak
			}
			if tempStreak > longestStreak {
				longestStreak = tempStreak
			}
		} else {
			if i == 0 {
				currentStreak = 0
			}
			tempStreak = 0
		}
	}

	stats.CurrentStreak = currentStreak
	stats.LongestStreak = longestStreak

	return stats
}

/*
╔═══════════════════════════════════════════════════════════════════╗
║  WORKER POOL PATTERN - ДЛЯ МАССОВЫХ ОПЕРАЦИЙ                      ║
╚═══════════════════════════════════════════════════════════════════╝

ОБОСНОВАНИЕ:
- Ограничиваем количество одновременных операций
- Предотвращаем перегрузку внешних сервисов
- Контролируемый параллелизм
*/

type NotificationJob struct {
	UserID  uint
	Message string
	Type    string
}

func ProcessNotificationsConcurrently(jobs []NotificationJob, workerCount int, logger *zap.Logger) {
	jobChan := make(chan NotificationJob, len(jobs))
	resultChan := make(chan error, len(jobs))
	var wg sync.WaitGroup

	// 🔥 ЗАПУСКАЕМ WORKER POOL
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go notificationWorker(i, jobChan, resultChan, &wg, logger)
	}

	// Отправляем задачи в channel
	for _, job := range jobs {
		jobChan <- job
	}
	close(jobChan)

	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// Собираем результаты
	successCount := 0
	errorCount := 0
	for err := range resultChan {
		if err != nil {
			errorCount++
		} else {
			successCount++
		}
	}

	logger.Info("notifications_processed",
		zap.Int("success", successCount),
		zap.Int("errors", errorCount),
		zap.Int("workers", workerCount),
	)
}

func notificationWorker(id int, jobs <-chan NotificationJob, results chan<- error, wg *sync.WaitGroup, logger *zap.Logger) {
	defer wg.Done()

	for job := range jobs {
		time.Sleep(50 * time.Millisecond) // Simulate sending

		logger.Info("notification_sent",
			zap.Int("worker_id", id),
			zap.Uint("user_id", job.UserID),
			zap.String("type", job.Type),
		)

		results <- nil
	}
}

/*
╔═══════════════════════════════════════════════════════════════════╗
║  BULK OPERATIONS WITH GOROUTINES                                  ║
╚═══════════════════════════════════════════════════════════════════╝
*/

func BulkUpdateHabitsActiveStatus(habitIDs []uint, isActive bool, logger *zap.Logger) error {
	if len(habitIDs) == 0 {
		return nil
	}

	errChan := make(chan error, len(habitIDs))
	var wg sync.WaitGroup

	// 🚀 Обновляем каждую привычку в отдельной горутине
	for _, id := range habitIDs {
		wg.Add(1)
		go func(habitID uint) {
			defer wg.Done()

			if err := db.DB.Model(&models.Habit{}).
				Where("id = ?", habitID).
				Update("is_active", isActive).Error; err != nil {
				errChan <- fmt.Errorf("failed to update habit %d: %w", habitID, err)
				return
			}

			cache.Delete(fmt.Sprintf("habit:%d", habitID))
			errChan <- nil
		}(id)
	}

	go func() {
		wg.Wait()
		close(errChan)
	}()

	for err := range errChan {
		if err != nil {
			logger.Error("bulk_update_error", zap.Error(err))
			return err
		}
	}

	logger.Info("bulk_update_completed",
		zap.Int("count", len(habitIDs)),
		zap.Bool("is_active", isActive),
	)

	return nil
}
