package redis

import (
	"context"
	"fmt"

	"event-ticketing-system/internal/config"

	"github.com/redis/go-redis/v9"
)

var Client *redis.Client

func Connect(cfg *config.Config) (*redis.Client, error) {
	Client = redis.NewClient(&redis.Options{
		Addr:     cfg.Redis.Addr(),
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})

	ctx := context.Background()
	_, err := Client.Ping(ctx).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	return Client, nil
}

func Close() error {
	if Client != nil {
		return Client.Close()
	}
	return nil
}
