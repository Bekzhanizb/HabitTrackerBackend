// cache/redis.go
package cache

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

var (
	Client *redis.Client
	ctx    = context.Background()
)

type CachedResponse struct {
	Status      int         `json:"status"`
	ContentType string      `json:"content_type"`
	Body        []byte      `json:"body"`
	Headers     http.Header `json:"headers"`
}

func InitRedis(logger *zap.Logger) error {
	redisHost := os.Getenv("REDIS_HOST")
	if redisHost == "" {
		redisHost = "localhost"
	}

	redisPort := os.Getenv("REDIS_PORT")
	if redisPort == "" {
		redisPort = "6379"
	}

	addr := fmt.Sprintf("%s:%s", redisHost, redisPort)

	Client = redis.NewClient(&redis.Options{
		Addr:         addr,
		Password:     "",
		DB:           0,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
		PoolSize:     10,
	})

	if err := Client.Ping(ctx).Err(); err != nil {
		logger.Error("redis_connection_failed",
			zap.Error(err),
			zap.String("addr", addr),
		)
		return err
	}

	logger.Info("redis_connected",
		zap.String("addr", addr),
	)

	return nil
}

func Set(key string, value interface{}, expiration time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("cache marshal failed: %w", err)
	}

	return Client.Set(ctx, key, data, expiration).Err()
}

// Get читает значение из Redis и десериализует в dest
func Get(key string, dest interface{}) error {
	val, err := Client.Get(ctx, key).Result()
	if err == redis.Nil {
		return fmt.Errorf("cache miss: %w", err)
	} else if err != nil {
		return fmt.Errorf("cache get failed: %w", err)
	}

	if err := json.Unmarshal([]byte(val), dest); err != nil {
		return fmt.Errorf("cache unmarshal failed: %w", err)
	}

	return nil
}

// Delete удаляет ключ
func Delete(key string) error {
	return Client.Del(ctx, key).Err()
}

// DeletePattern удаляет все ключи по шаблону (например, cache:1:*)
func DeletePattern(pattern string) error {
	var cursor uint64
	for {
		keys, cursor, err := Client.Scan(ctx, cursor, pattern, 100).Result()
		if err != nil {
			return fmt.Errorf("scan failed: %w", err)
		}

		if len(keys) > 0 {
			if err := Client.Del(ctx, keys...).Err(); err != nil {
				return fmt.Errorf("delete keys failed: %w", err)
			}
		}

		if cursor == 0 {
			break
		}
	}
	return nil
}

// IncrementCounter увеличивает счётчик и устанавливает TTL при первом инкременте
func IncrementCounter(key string, expiration time.Duration) (int64, error) {
	val, err := Client.Incr(ctx, key).Result()
	if err != nil {
		return 0, err
	}

	// Устанавливаем TTL только при первом увеличении (когда val становится 1)
	if val == 1 {
		if err := Client.Expire(ctx, key, expiration).Err(); err != nil {
			return val, err
		}
	}

	return val, nil
}

// Close закрывает соединение с Redis
func Close() error {
	if Client != nil {
		return Client.Close()
	}
	return nil
}

// Вспомогательная функция для логирования тела ответа (используется в middleware)
type bodyLogWriter struct {
	gin.ResponseWriter
	body *bytes.Buffer
}

func (w *bodyLogWriter) Write(b []byte) (int, error) {
	w.body.Write(b)
	return w.ResponseWriter.Write(b)
}

func (w *bodyLogWriter) WriteString(s string) (int, error) {
	w.body.WriteString(s)
	return w.ResponseWriter.WriteString(s)
}
