package db

import (
	"context"
	"focalors-go/config"
	"time"

	"github.com/redis/go-redis/v9"
)

type Redis struct {
	RedisClient *redis.Client
	RedisCtx    context.Context
	cfg         *config.RedisConfig
}

func NewRedis(ctx context.Context, cfg *config.RedisConfig) *Redis {
	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.Addr,
		Password: cfg.Password,
		DB:       cfg.DB,
	})

	if err := rdb.Ping(ctx).Err(); err != nil {
		panic("Failed to connect to Redis: " + err.Error())
	}

	return &Redis{
		RedisClient: rdb,
		RedisCtx:    ctx,
		cfg:         cfg,
	}
}

func (r *Redis) HSet(key string, values ...any) error {
	return r.RedisClient.HSet(r.RedisCtx, key, values...).Err()
}

func (r *Redis) Del(key string) error {
	return r.RedisClient.Del(r.RedisCtx, key).Err()
}

func (r *Redis) Keys(pattern string) ([]string, error) {
	return r.RedisClient.Keys(r.RedisCtx, pattern).Result()
}

func (r *Redis) HGetAll(key string) (map[string]string, error) {
	result, err := r.RedisClient.HGetAll(r.RedisCtx, key).Result()
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (r *Redis) Get(key string) (string, error) {
	cmd := r.RedisClient.Get(r.RedisCtx, key)
	if err := cmd.Err(); err != nil {
		return "", err
	}
	result, err := cmd.Result()
	if err != nil {
		return "", err
	}
	return result, nil
}

func (r *Redis) Set(key string, value any, expiration time.Duration) error {
	cmd := r.RedisClient.Set(r.RedisCtx, key, value, expiration)
	if err := cmd.Err(); err != nil {
		return err
	}
	return nil
}

func (r *Redis) Exists(key string) (int64, error) {
	cmd := r.RedisClient.Exists(r.RedisCtx, key)
	if err := cmd.Err(); err != nil {
		return 0, err
	}
	return cmd.Result()
}

func (r *Redis) Close() error {
	return r.RedisClient.Close()
}
