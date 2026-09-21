package services

import (
	"crypto/tls"

	"github.com/verbeux-ai/whatsmiau/env"
	"golang.org/x/net/context"

	"github.com/go-redis/redis/v8"
	"go.uber.org/zap"
)

var redisInstance *redis.Client

func Redis() *redis.Client {
	if redisInstance == nil {
		instance, err := NewRedis()
		if err != nil {
			zap.L().Fatal("failed to start redis", zap.Error(err))
		}

		redisInstance = instance
	}

	return redisInstance
}

func NewRedis() (*redis.Client, error) {
	opt := &redis.Options{
		Addr:     env.Env.RedisURL,
		Password: env.Env.RedisPassword,
		DB:       0,
	}

	if env.Env.RedisTLS {
		opt.TLSConfig = &tls.Config{}
	}

	client := redis.NewClient(opt)
	if err := client.Ping(context.Background()).Err(); err != nil {
		zap.L().Panic("failed to connect to redis", zap.Error(err))
	}

	return client, nil
}
