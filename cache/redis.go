package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

var (
	Client *redis.Client
	ctx    = context.Background()
)

func InitRedis(logger *zap.Logger) error {
	redisHost := os.Getenv("REDIS_HOST")
	if redisHost == "" {
		redisHost = "localhost"
	}

	redisPort := os.Getenv("REDIS_PORT")
	if redisPort == "" {
		redisPort = "6379"
	}

	Client = redis.NewClient(&redis.Options{
		Addr:         fmt.Sprintf("%s:%s", redisHost, redisPort),
		Password:     "",
		DB:           0,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
		PoolSize:     10,
	})

	if err := Client.Ping(ctx).Err(); err != nil {
		logger.Error("redis_connection_failed", zap.Error(err))
		return err
	}

	logger.Info("redis_connected", zap.String("addr", fmt.Sprintf("%s:%s", redisHost, redisPort)))
	return nil
}

func Set(key string, value interface{}, expiration time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return Client.Set(ctx, key, data, expiration).Err()
}

func Get(key string, dest interface{}) error {
	val, err := Client.Get(ctx, key).Result()
	if err != nil {
		return err
	}
	return json.Unmarshal([]byte(val), dest)
}

func Delete(key string) error {
	return Client.Del(ctx, key).Err()
}

func DeletePattern(pattern string) error {
	iter := Client.Scan(ctx, 0, pattern, 0).Iterator()
	for iter.Next(ctx) {
		if err := Client.Del(ctx, iter.Val()).Err(); err != nil {
			return err
		}
	}
	return iter.Err()
}

func IncrementCounter(key string, expiration time.Duration) (int64, error) {
	val, err := Client.Incr(ctx, key).Result()
	if err != nil {
		return 0, err
	}
	if val == 1 {
		Client.Expire(ctx, key, expiration)
	}
	return val, nil
}

func Close() error {
	if Client != nil {
		return Client.Close()
	}
	return nil
}
