package config

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisClient struct {
	Client *redis.Client
}

func NewRedisClient(cfg *Config) (*RedisClient, error) {
	opts, err := redis.ParseURL(cfg.RedisURL)
	if err != nil {
		// Fallback to default connection
		opts = &redis.Options{
			Addr:     "localhost:6379",
			Password: "",
			DB:       0,
		}
	}

	client := redis.NewClient(opts)
	
	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	log.Println("Redis connection established successfully")
	return &RedisClient{Client: client}, nil
}

func (r *RedisClient) Close() error {
	return r.Client.Close()
}